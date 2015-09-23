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
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/units"
	"github.com/fsouza/go-dockerclient"
)

const (
	COMMIT_SKIP = "COMMIT_SKIP"
)

type ConfigCommand struct {
	name     string
	args     []string
	attrs    map[string]bool
	flags    map[string]string
	original string
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

func NewCommand(cfg ConfigCommand) (Command, error) {
	// TODO: use reflection
	switch cfg.name {
	case "from":
		return &CommandFrom{cfg}, nil
	case "maintainer":
		return &CommandMaintainer{cfg}, nil
	case "run":
		return &CommandRun{cfg}, nil
	case "env":
		return &CommandEnv{cfg}, nil
	case "label":
		return &CommandLabel{cfg}, nil
	case "workdir":
		return &CommandWorkdir{cfg}, nil
	case "tag":
		return &CommandTag{cfg}, nil
	case "copy":
		return &CommandCopy{cfg}, nil
	case "add":
		return &CommandAdd{cfg}, nil
	case "cmd":
		return &CommandCmd{cfg}, nil
	}
	return nil, fmt.Errorf("Unknown command: %s", cfg.name)
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

	return s, nil
}

// CommandMaintainer implements CMD
type CommandMaintainer struct {
	cfg ConfigCommand
}

func (c *CommandMaintainer) String() string {
	return c.cfg.original
}

func (c *CommandMaintainer) Execute(b *Build) (s State, err error) {
	s = b.state
	if len(c.cfg.args) != 1 {
		return s, fmt.Errorf("MAINTAINER requires exactly one argument")
	}

	// Don't see any sense of doing a commit here, as Docker does
	s.SkipCommit()

	return s, nil
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

	if len(commits) == 0 {
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

	// TODO: unsetting CMD?
	// if len(args) != 0 {
	// 	b.cmdSet = true
	// }

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

// CommandCopy implements ADD
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
