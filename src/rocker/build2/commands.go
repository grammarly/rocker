/*-
 * Copyright 2015 Grammarly, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package build2

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/units"
	"github.com/fsouza/go-dockerclient"
)

const (
	COMMIT_SKIP = "COMMIT_SKIP"
)

type ConfigCommand struct {
	name      string
	args      []string
	attrs     map[string]bool
	flags     map[string]string
	original  string
	isOnbuild bool
}

type Command interface {
	// Execute does the command execution and returns modified state.
	// Note that here we use State not by reference because we want
	// it to be immutable. In future, it may encoded/decoded from json
	// and passed to the external command implementations.
	Execute(b *Build) (State, error)

	// String returns the human readable string representation of the command
	String() string
}

func NewCommand(cfg ConfigCommand) (cmd Command, err error) {
	// TODO: use reflection?
	switch cfg.name {
	case "from":
		cmd = &CommandFrom{cfg}
	case "maintainer":
		cmd = &CommandMaintainer{cfg}
	case "run":
		cmd = &CommandRun{cfg}
	case "attach":
		cmd = &CommandAttach{cfg}
	case "env":
		cmd = &CommandEnv{cfg}
	case "label":
		cmd = &CommandLabel{cfg}
	case "workdir":
		cmd = &CommandWorkdir{cfg}
	case "tag":
		cmd = &CommandTag{cfg}
	case "push":
		cmd = &CommandPush{cfg}
	case "copy":
		cmd = &CommandCopy{cfg}
	case "add":
		cmd = &CommandAdd{cfg}
	case "cmd":
		cmd = &CommandCmd{cfg}
	case "entrypoint":
		cmd = &CommandEntrypoint{cfg}
	case "expose":
		cmd = &CommandExpose{cfg}
	case "volume":
		cmd = &CommandVolume{cfg}
	case "user":
		cmd = &CommandUser{cfg}
	default:
		return nil, fmt.Errorf("Unknown command: %s", cfg.name)
	}

	if cfg.isOnbuild {
		cmd = &CommandOnbuildWrap{cmd}
	}

	return cmd, nil
}

// CommandFrom implements FROM
type CommandFrom struct {
	cfg ConfigCommand
}

func (c *CommandFrom) String() string {
	return c.cfg.original
}

func (c *CommandFrom) Execute(b *Build) (s State, err error) {
	// TODO: for "scratch" image we may use /images/create

	if len(c.cfg.args) != 1 {
		return s, fmt.Errorf("FROM requires one argument")
	}

	var (
		img  *docker.Image
		name = c.cfg.args[0]
	)

	// If Pull is true, then img will remain nil and it will be pulled below
	if !b.cfg.Pull {
		if img, err = b.client.InspectImage(name); err != nil {
			return s, err
		}
	}

	if img == nil {
		if err = b.client.PullImage(name); err != nil {
			return s, err
		}
		if img, err = b.client.InspectImage(name); err != nil {
			return s, err
		}
		if img == nil {
			return s, fmt.Errorf("FROM: Failed to inspect image after pull: %s", name)
		}
	}

	// We want to say the size of the FROM image. Better to do it
	// from the client, but don't know how to do it better,
	// without duplicating InspectImage calls and making unnecessary functions

	log.WithFields(log.Fields{
		"size": units.HumanSize(float64(img.VirtualSize)),
	}).Infof("| Image %.12s", img.ID)

	s = b.state
	s.ImageID = img.ID
	s.Config = *img.Config

	// If we don't have OnBuild triggers, then we are done
	if len(s.Config.OnBuild) == 0 {
		return s, nil
	}

	log.Infof("| Found %d ONBUILD triggers", len(s.Config.OnBuild))

	// Remove them from the config, since the config will be committed.
	s.InjectCommands = s.Config.OnBuild
	s.Config.OnBuild = []string{}

	return s, nil
}

// CommandMaintainer implements CMD
type CommandMaintainer struct {
	cfg ConfigCommand
}

func (c *CommandMaintainer) String() string {
	return c.cfg.original
}

func (c *CommandMaintainer) Execute(b *Build) (State, error) {
	if len(c.cfg.args) != 1 {
		return b.state, fmt.Errorf("MAINTAINER requires exactly one argument")
	}

	// Don't see any sense of doing a commit here, as Docker does

	return b.state, nil
}

// CommandReset cleans the builder state before the next FROM
type CommandCleanup struct {
	final  bool
	tagged bool
}

func (c *CommandCleanup) String() string {
	return "Cleaning up"
}

func (c *CommandCleanup) Execute(b *Build) (State, error) {
	s := b.state

	if b.cfg.NoGarbage && !c.tagged && s.ImageID != "" && s.ProducedImage {
		if err := b.client.RemoveImage(s.ImageID); err != nil {
			return s, err
		}
	}

	// For final cleanup we want to keep imageID
	if !c.final {
		s.ImageID = ""
	}

	return s, nil
}

// CommandCommit commits collected changes
type CommandCommit struct{}

func (c *CommandCommit) String() string {
	return "Commit changes"
}

func (c *CommandCommit) Execute(b *Build) (s State, err error) {
	s = b.state

	// Collect commits that are not skipped
	commits := []string{}
	for _, msg := range s.CommitMsg {
		if msg != COMMIT_SKIP {
			commits = append(commits, msg)
		}
	}

	// Reset collected commit messages after the commit
	s.CommitMsg = []string{}

	if len(commits) == 0 && s.ContainerID == "" {
		log.Infof("| Skip")
		return s, nil
	}

	message := strings.Join(commits, "; ")

	if s.ContainerID == "" {
		origCmd := s.Config.Cmd
		s.Config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + message}

		if s.ContainerID, err = b.client.CreateContainer(s); err != nil {
			return s, err
		}

		s.Config.Cmd = origCmd
	}

	if s.ImageID, err = b.client.CommitContainer(s, message); err != nil {
		return s, err
	}

	s.ProducedImage = true

	if err = b.client.RemoveContainer(s.ContainerID); err != nil {
		return s, err
	}

	s.ContainerID = ""

	return s, nil
}

// CommandRun implements RUN
type CommandRun struct {
	cfg ConfigCommand
}

func (c *CommandRun) String() string {
	return c.cfg.original
}

func (c *CommandRun) Execute(b *Build) (s State, err error) {
	s = b.state

	if s.ImageID == "" {
		return s, fmt.Errorf("Please provide a source image with `FROM` prior to run")
	}

	cmd := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	if !c.cfg.attrs["json"] {
		cmd = append([]string{"/bin/sh", "-c"}, cmd...)
	}

	// TODO: test with ENTRYPOINT

	// We run this command in the container using CMD
	origCmd := s.Config.Cmd
	s.Config.Cmd = cmd

	if s.ContainerID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	if err = b.client.RunContainer(s.ContainerID, false); err != nil {
		return s, err
	}

	// Restore command after commit
	s.Config.Cmd = origCmd

	return s, nil
}

// CommandAttach implements ATTACH
type CommandAttach struct {
	cfg ConfigCommand
}

func (c *CommandAttach) String() string {
	return c.cfg.original
}

func (c *CommandAttach) Execute(b *Build) (s State, err error) {
	s = b.state

	// simply ignore this command if we don't wanna attach
	if !b.cfg.Attach {
		log.Infof("Skip ATTACH; use --attach option to get inside")
		s.SkipCommit()
		return s, nil
	}

	if s.ImageID == "" {
		return s, fmt.Errorf("Please provide a source image with `FROM` prior to ATTACH")
	}

	cmd := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	} else if !c.cfg.attrs["json"] {
		cmd = append([]string{"/bin/sh", "-c"}, cmd...)
	}

	// TODO: test with ENTRYPOINT

	// We run this command in the container using CMD

	// Backup the config so we can restore it later
	origConfig := s.Config

	s.Config.Cmd = cmd
	s.Config.Entrypoint = []string{}
	s.Config.Tty = true
	s.Config.OpenStdin = true
	s.Config.StdinOnce = true
	s.Config.AttachStdin = true
	s.Config.AttachStderr = true
	s.Config.AttachStdout = true

	if s.ContainerID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	if err = b.client.RunContainer(s.ContainerID, true); err != nil {
		return s, err
	}

	// Restore the config
	s.Config = origConfig

	return s, nil
}

// CommandEnv implements ENV
type CommandEnv struct {
	cfg ConfigCommand
}

func (c *CommandEnv) String() string {
	return c.cfg.original
}

func (c *CommandEnv) Execute(b *Build) (s State, err error) {

	s = b.state
	args := c.cfg.args

	if len(args) == 0 {
		return s, fmt.Errorf("ENV requires at least one argument")
	}

	if len(args)%2 != 0 {
		// should never get here, but just in case
		return s, fmt.Errorf("Bad input to ENV, too many args")
	}

	commitStr := "ENV"

	for j := 0; j < len(args); j += 2 {
		// name  ==> args[j]
		// value ==> args[j+1]
		newVar := strings.Join(args[j:j+2], "=")
		commitStr += " " + newVar

		gotOne := false
		for i, envVar := range s.Config.Env {
			envParts := strings.SplitN(envVar, "=", 2)
			if envParts[0] == args[j] {
				s.Config.Env[i] = newVar
				gotOne = true
				break
			}
		}
		if !gotOne {
			s.Config.Env = append(s.Config.Env, newVar)
		}
	}

	s.Commit(commitStr)

	return s, nil
}

// CommandLabel implements LABEL
type CommandLabel struct {
	cfg ConfigCommand
}

func (c *CommandLabel) String() string {
	return c.cfg.original
}

func (c *CommandLabel) Execute(b *Build) (s State, err error) {

	s = b.state
	args := c.cfg.args

	if len(args) == 0 {
		return s, fmt.Errorf("LABEL requires at least one argument")
	}

	if len(args)%2 != 0 {
		// should never get here, but just in case
		return s, fmt.Errorf("Bad input to LABEL, too many args")
	}

	commitStr := "LABEL"

	if s.Config.Labels == nil {
		s.Config.Labels = map[string]string{}
	}

	for j := 0; j < len(args); j++ {
		// name  ==> args[j]
		// value ==> args[j+1]
		newVar := args[j] + "=" + args[j+1] + ""
		commitStr += " " + newVar

		s.Config.Labels[args[j]] = args[j+1]
		j++
	}

	s.Commit(commitStr)

	return s, nil
}

// CommandWorkdir implements WORKDIR
type CommandWorkdir struct {
	cfg ConfigCommand
}

func (c *CommandWorkdir) String() string {
	return c.cfg.original
}

func (c *CommandWorkdir) Execute(b *Build) (s State, err error) {

	s = b.state

	if len(c.cfg.args) != 1 {
		return s, fmt.Errorf("WORKDIR requires exactly one argument")
	}

	workdir := c.cfg.args[0]

	if !filepath.IsAbs(workdir) {
		current := s.Config.WorkingDir
		workdir = filepath.Join("/", current, workdir)
	}

	s.Config.WorkingDir = workdir

	s.Commit(fmt.Sprintf("WORKDIR %v", workdir))

	return s, nil
}

// CommandCmd implements CMD
type CommandCmd struct {
	cfg ConfigCommand
}

func (c *CommandCmd) String() string {
	return c.cfg.original
}

func (c *CommandCmd) Execute(b *Build) (s State, err error) {
	s = b.state

	cmd := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	if !c.cfg.attrs["json"] {
		cmd = append([]string{"/bin/sh", "-c"}, cmd...)
	}

	s.Config.Cmd = cmd

	s.Commit(fmt.Sprintf("CMD %q", cmd))

	if len(c.cfg.args) != 0 {
		s.CmdSet = true
	}

	return s, nil
}

// CommandEntrypoint implements ENTRYPOINT
type CommandEntrypoint struct {
	cfg ConfigCommand
}

func (c *CommandEntrypoint) String() string {
	return c.cfg.original
}

func (c *CommandEntrypoint) Execute(b *Build) (s State, err error) {
	s = b.state

	parsed := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	switch {
	case c.cfg.attrs["json"]:
		// ENTRYPOINT ["echo", "hi"]
		s.Config.Entrypoint = parsed
	case len(parsed) == 0:
		// ENTRYPOINT []
		s.Config.Entrypoint = nil
	default:
		// ENTRYPOINT echo hi
		s.Config.Entrypoint = []string{"/bin/sh", "-c", parsed[0]}
	}

	s.Commit(fmt.Sprintf("ENTRYPOINT %q", s.Config.Entrypoint))

	// TODO: test this
	// when setting the entrypoint if a CMD was not explicitly set then
	// set the command to nil
	if !s.CmdSet {
		s.Config.Cmd = nil
	}

	return s, nil
}

// CommandExpose implements EXPOSE
type CommandExpose struct {
	cfg ConfigCommand
}

func (c *CommandExpose) String() string {
	return c.cfg.original
}

func (c *CommandExpose) Execute(b *Build) (s State, err error) {

	s = b.state

	if len(c.cfg.args) == 0 {
		return s, fmt.Errorf("EXPOSE requires at least one argument")
	}

	if s.Config.ExposedPorts == nil {
		s.Config.ExposedPorts = map[docker.Port]struct{}{}
	}

	ports, _, err := nat.ParsePortSpecs(c.cfg.args)
	if err != nil {
		return s, err
	}

	// instead of using ports directly, we build a list of ports and sort it so
	// the order is consistent. This prevents cache burst where map ordering
	// changes between builds
	portList := make([]string, len(ports))
	var i int
	for port := range ports {
		dockerPort := docker.Port(port)
		if _, exists := s.Config.ExposedPorts[dockerPort]; !exists {
			s.Config.ExposedPorts[dockerPort] = struct{}{}
		}
		portList[i] = string(port)
		i++
	}
	sort.Strings(portList)

	message := fmt.Sprintf("EXPOSE %s", strings.Join(portList, " "))
	s.Commit(message)

	return s, nil
}

// CommandVolume implements VOLUME
type CommandVolume struct {
	cfg ConfigCommand
}

func (c *CommandVolume) String() string {
	return c.cfg.original
}

func (c *CommandVolume) Execute(b *Build) (s State, err error) {

	s = b.state

	if len(c.cfg.args) == 0 {
		return s, fmt.Errorf("VOLUME requires at least one argument")
	}

	if s.Config.Volumes == nil {
		s.Config.Volumes = map[string]struct{}{}
	}
	for _, v := range c.cfg.args {
		v = strings.TrimSpace(v)
		if v == "" {
			return s, fmt.Errorf("Volume specified can not be an empty string")
		}
		s.Config.Volumes[v] = struct{}{}
	}

	s.Commit(fmt.Sprintf("VOLUME %v", c.cfg.args))

	return s, nil
}

// CommandUser implements USER
type CommandUser struct {
	cfg ConfigCommand
}

func (c *CommandUser) String() string {
	return c.cfg.original
}

func (c *CommandUser) Execute(b *Build) (s State, err error) {

	s = b.state

	if len(c.cfg.args) != 1 {
		return s, fmt.Errorf("USER requires exactly one argument")
	}

	s.Config.User = c.cfg.args[0]

	s.Commit(fmt.Sprintf("USER %v", c.cfg.args))

	return s, nil
}

// CommandTag implements TAG
type CommandTag struct {
	cfg ConfigCommand
}

func (c *CommandTag) String() string {
	return c.cfg.original
}

func (c *CommandTag) Execute(b *Build) (State, error) {
	if len(c.cfg.args) != 1 {
		return b.state, fmt.Errorf("TAG requires exactly one argument")
	}

	if b.state.ImageID == "" {
		return b.state, fmt.Errorf("Cannot TAG on empty image")
	}

	if err := b.client.TagImage(b.state.ImageID, c.cfg.args[0]); err != nil {
		return b.state, err
	}

	return b.state, nil
}

// CommandPush implements PUSH
type CommandPush struct {
	cfg ConfigCommand
}

func (c *CommandPush) String() string {
	return c.cfg.original
}

func (c *CommandPush) Execute(b *Build) (State, error) {
	if len(c.cfg.args) != 1 {
		return b.state, fmt.Errorf("PUSH requires exactly one argument")
	}

	if b.state.ImageID == "" {
		return b.state, fmt.Errorf("Cannot PUSH empty image")
	}

	if err := b.client.TagImage(b.state.ImageID, c.cfg.args[0]); err != nil {
		return b.state, err
	}

	if err := b.client.PushImage(c.cfg.args[0]); err != nil {
		return b.state, err
	}

	return b.state, nil
}

// CommandCopy implements COPY
type CommandCopy struct {
	cfg ConfigCommand
}

func (c *CommandCopy) String() string {
	return c.cfg.original
}

func (c *CommandCopy) Execute(b *Build) (State, error) {
	if len(c.cfg.args) < 2 {
		return b.state, fmt.Errorf("COPY requires at least two arguments")
	}
	return copyFiles(b, c.cfg.args, "COPY")
}

// CommandAdd implements ADD
// For now it is an alias of COPY, but later will add urls and archives to it
type CommandAdd struct {
	cfg ConfigCommand
}

func (c *CommandAdd) String() string {
	return c.cfg.original
}

func (c *CommandAdd) Execute(b *Build) (State, error) {
	if len(c.cfg.args) < 2 {
		return b.state, fmt.Errorf("ADD requires at least two arguments")
	}
	return copyFiles(b, c.cfg.args, "ADD")
}

// CommandOnbuildWrap wraps ONBUILD command
type CommandOnbuildWrap struct {
	cmd Command
}

func (c *CommandOnbuildWrap) String() string {
	return "ONBUILD " + c.cmd.String()
}

func (c *CommandOnbuildWrap) Execute(b *Build) (State, error) {
	return c.cmd.Execute(b)
}
