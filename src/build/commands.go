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

package build

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/grammarly/rocker/src/imagename"
	"github.com/grammarly/rocker/src/shellparser"
	"github.com/grammarly/rocker/src/util"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/nat"
	"github.com/docker/docker/pkg/units"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/fsouza/go-dockerclient"
	"github.com/kr/pretty"
)

// ConfigCommand configuration parameters for any command
type ConfigCommand struct {
	name      string
	args      []string
	attrs     map[string]bool
	flags     map[string]string
	original  string
	isOnbuild bool
}

// Command interface describes and command that is executed by build
type Command interface {
	// Execute does the command execution and returns modified state.
	// Note that here we use State not by reference because we want
	// it to be immutable. In future, it may encoded/decoded from json
	// and passed to the external command implementations.
	Execute(b *Build) (State, error)

	// Returns true if the command should be executed
	ShouldRun(b *Build) (bool, error)

	// String returns the human readable string representation of the command
	String() string
}

// EnvReplacableCommand interface describes the command that can replace ENV
// variables into arguments of itself
type EnvReplacableCommand interface {
	ReplaceEnv(env []string) error
}

// NewCommand make a new command according to the configuration given
func NewCommand(cfg ConfigCommand) (cmd Command) {
	// TODO: use reflection?
	switch cfg.name {
	case "from":
		cmd = &CommandFrom{CommandBase{cfg}}
	case "maintainer":
		cmd = &CommandMaintainer{CommandBase{cfg}}
	case "run":
		cmd = &CommandRun{CommandBase{cfg}}
	case "attach":
		cmd = &CommandAttach{CommandBase{cfg}}
	case "env":
		cmd = &CommandEnv{CommandBase{cfg}}
	case "label":
		cmd = &CommandLabel{CommandBase{cfg}}
	case "workdir":
		cmd = &CommandWorkdir{CommandBase{cfg}}
	case "tag":
		cmd = &CommandTag{CommandBase{cfg}}
	case "push":
		cmd = &CommandPush{CommandBase{cfg}}
	case "copy":
		cmd = &CommandCopy{CommandBase{cfg}}
	case "add":
		cmd = &CommandAdd{CommandBase{cfg}}
	case "cmd":
		cmd = &CommandCmd{CommandBase{cfg}}
	case "entrypoint":
		cmd = &CommandEntrypoint{CommandBase{cfg}}
	case "expose":
		cmd = &CommandExpose{CommandBase{cfg}}
	case "volume":
		cmd = &CommandVolume{CommandBase{cfg}}
	case "user":
		cmd = &CommandUser{CommandBase{cfg}}
	case "onbuild":
		cmd = &CommandOnbuild{CommandBase{cfg}}
	case "mount":
		cmd = &CommandMount{CommandBase{cfg}}
	case "export":
		cmd = &CommandExport{CommandBase{cfg}}
	case "import":
		cmd = &CommandImport{CommandBase{cfg}}
	case "arg":
		cmd = &CommandArg{CommandBase{cfg}}
	default:
		panic(fmt.Sprintf("Unknown command: %s", cfg.name))
	}

	if cfg.isOnbuild {
		cmd = &CommandOnbuildWrap{cmd}
	}

	return
}

// CommandBase implements base command that includes ConfigCommand
// and always run without a precondition
type CommandBase struct {
	cfg ConfigCommand
}

// String returns the human readable string representation of the command
func (c *CommandBase) String() string {
	return c.cfg.original
}

// ShouldRun returns true if the command should be executed
func (c *CommandBase) ShouldRun(b *Build) (bool, error) {
	return true, nil
}

// CommandFrom implements FROM
type CommandFrom struct {
	CommandBase
}

// Execute runs the command
func (c *CommandFrom) Execute(b *Build) (s State, err error) {
	// TODO: for "scratch" image we may use /images/create

	if len(c.cfg.args) != 1 {
		return s, fmt.Errorf("FROM requires one argument")
	}

	var (
		img  *docker.Image
		name = c.cfg.args[0]
	)

	if name == "scratch" {
		s.NoBaseImage = true
		s.Size = 0
		s.ParentSize = 0
		b.ProducedSize = 0
		b.VirtualSize = 0
		return s, nil
	}

	if img, err = b.lookupImage(name); err != nil {
		return s, fmt.Errorf("FROM error: %s", err)
	}

	if img == nil {
		return s, fmt.Errorf("FROM: image %s not found", name)
	}

	// We want to say the size of the FROM image. Better to do it
	// from the client, but don't know how to do it better,
	// without duplicating InspectImage calls and making unnecessary functions

	s = b.state
	s.ImageID = img.ID
	s.Config = docker.Config{}

	s.Size = img.VirtualSize

	// As we don't know size of parent image for that of FROM command,
	// initialize ParentSize so that Produced (e.g. added) Size would
	// be zero for FROM image
	s.ParentSize = img.VirtualSize

	// From now and thereon, ProducedSize is maintained to be Size - ParentSize
	b.ProducedSize = s.Size - s.ParentSize
	b.VirtualSize = s.Size

	if img.Config != nil {
		s.Config = *img.Config
	}

	fields := log.Fields{}
	if b.cfg.LogJSON {
		fields["size"] = s.Size
		fields["delta"] = s.Size - s.ParentSize
	} else {
		fields["size"] = units.HumanSize(float64(img.VirtualSize))
	}

	log.WithFields(fields).Infof("| Image %.12s", img.ID)

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
	CommandBase
}

// Execute runs the command
func (c *CommandMaintainer) Execute(b *Build) (State, error) {
	if len(c.cfg.args) != 1 {
		return b.state, fmt.Errorf("MAINTAINER requires exactly one argument")
	}

	// Don't see any sense of doing a commit here, as Docker does

	return b.state, nil
}

// CommandCleanup cleans the builder state before the next FROM
type CommandCleanup struct {
	final  bool
	tagged bool
}

// String returns the human readable string representation of the command
func (c *CommandCleanup) String() string {
	return "Cleaning up"
}

// ShouldRun returns true if the command should be executed
func (c *CommandCleanup) ShouldRun(b *Build) (bool, error) {
	return true, nil
}

// Execute runs the command
func (c *CommandCleanup) Execute(b *Build) (State, error) {
	s := b.state

	if b.cfg.NoGarbage && !c.tagged && s.ImageID != "" && s.ProducedImage {
		if err := b.client.RemoveImage(s.ImageID); err != nil {
			return s, err
		}
	}

	// Cleanup state
	dirtyState := s
	s = NewState(b)

	// Keep some stuff between froms
	s.ExportsID = dirtyState.ExportsID

	// For final cleanup we want to keep imageID
	if c.final {
		s.ImageID = dirtyState.ImageID
	} else {
		log.Infof("====================================")
	}

	return s, nil
}

// CommandCommit commits collected changes
type CommandCommit struct{}

// String returns the human readable string representation of the command
func (c *CommandCommit) String() string {
	return "Commit changes"
}

// ShouldRun returns true if the command should be executed
func (c *CommandCommit) ShouldRun(b *Build) (bool, error) {
	return b.state.GetCommits() != "", nil
}

// Execute runs the command
func (c *CommandCommit) Execute(b *Build) (s State, err error) {
	s = b.state

	commits := s.GetCommits()
	if commits == "" {
		return s, nil
	}

	if s.ImageID == "" && !s.NoBaseImage {
		return s, fmt.Errorf("Please provide a source image with `from` prior to commit")
	}

	// TODO: ?
	// if len(commits) == 0 && s.NoCache.ContainerID == "" { log.Infof("| Skip")

	// TODO: verify that we need to check cache in commit only for
	//       a non-container actions

	if s.NoCache.ContainerID == "" {

		// Check cache
		var hit bool
		s, hit, err = b.probeCache(s)
		if err != nil {
			return s, err
		}
		if hit {
			return s, nil
		}

		origCmd := s.Config.Cmd
		s.Config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + commits}

		if s.NoCache.ContainerID, err = b.client.CreateContainer(s); err != nil {
			return s, err
		}

		s.Config.Cmd = origCmd
	}

	defer func(id string) {
		s.CleanCommits()
		if err := b.client.RemoveContainer(id); err != nil {
			log.Errorf("Failed to remove temporary container %.12s, error: %s", id, err)
		}
	}(s.NoCache.ContainerID)

	var img *docker.Image
	if img, err = b.client.CommitContainer(&s); err != nil {
		return s, err
	}

	s.NoCache.ContainerID = ""
	s.ParentID = s.ImageID
	s.ImageID = img.ID
	s.ProducedImage = true

	if b.cache != nil {
		if err := b.cache.Put(s); err != nil {
			return s, err
		}
	}

	// Store some stuff to the build
	b.ProducedSize += s.Size - s.ParentSize
	b.VirtualSize = s.Size

	return s, nil
}

// CommandRun implements RUN
type CommandRun struct {
	CommandBase
}

// Execute runs the command
func (c *CommandRun) Execute(b *Build) (s State, err error) {
	s = b.state

	if s.ImageID == "" && !s.NoBaseImage {
		return s, fmt.Errorf("Please provide a source image with `FROM` prior to run")
	}

	cmd := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	if !c.cfg.attrs["json"] {
		cmd = append([]string{"/bin/sh", "-c"}, cmd...)
	}

	buildEnv := []string{}
	configEnv := runconfigopts.ConvertKVStringsToMap(s.Config.Env)
	for key, val := range s.NoCache.BuildArgs {
		if !b.allowedBuildArgs[key] {
			// skip build-args that are not in allowed list, meaning they have
			// not been defined by an "ARG" Dockerfile command yet.
			// This is an error condition but only if there is no "ARG" in the entire
			// Dockerfile, so we'll generate any necessary errors after we parsed
			// the entire file (see 'leftoverArgs' processing in evaluator.go )
			continue
		}
		if _, ok := configEnv[key]; !ok {
			buildEnv = append(buildEnv, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// derive the command to use for probeCache() and to commit in this container.
	// Note that we only do this if there are any build-time env vars.  Also, we
	// use the special argument "|#" at the start of the args array. This will
	// avoid conflicts with any RUN command since commands can not
	// start with | (vertical bar). The "#" (number of build envs) is there to
	// help ensure proper cache matches. We don't want a RUN command
	// that starts with "foo=abc" to be considered part of a build-time env var.
	saveCmd := cmd
	if len(buildEnv) > 0 {
		sort.Strings(buildEnv)
		tmpEnv := append([]string{fmt.Sprintf("|%d", len(buildEnv))}, buildEnv...)
		saveCmd = append(tmpEnv, saveCmd...)
	}

	s.Commit("RUN %q", saveCmd)

	// Check cache
	s, hit, err := b.probeCache(s)
	if err != nil {
		return s, err
	}
	if hit {
		return s, nil
	}

	// We run this command in the container using CMD
	origCmd := s.Config.Cmd
	origEntrypoint := s.Config.Entrypoint
	origEnv := s.Config.Env
	s.Config.Cmd = cmd
	s.Config.Entrypoint = []string{}
	s.Config.Env = append(s.Config.Env, buildEnv...)

	if s.NoCache.ContainerID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	if err = b.client.RunContainer(s.NoCache.ContainerID, false); err != nil {
		b.client.RemoveContainer(s.NoCache.ContainerID)
		return s, err
	}

	// Restore command after commit
	s.Config.Cmd = origCmd
	s.Config.Entrypoint = origEntrypoint
	s.Config.Env = origEnv

	return s, nil
}

// CommandAttach implements ATTACH
type CommandAttach struct {
	CommandBase
}

// Execute runs the command
func (c *CommandAttach) Execute(b *Build) (s State, err error) {
	s = b.state

	// simply ignore this command if we don't wanna attach
	// TODO: skip via ShouldRun() ?
	if !b.cfg.Attach {
		log.Infof("Skip ATTACH; use --attach option to get inside")
		// s.SkipCommit()
		return s, nil
	}

	if s.ImageID == "" && !s.NoBaseImage {
		return s, fmt.Errorf("Please provide a source image with `FROM` prior to ATTACH")
	}

	cmd := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	} else if !c.cfg.attrs["json"] {
		cmd = append([]string{"/bin/sh", "-c"}, cmd...)
	}

	// TODO: do s.commit unique

	// We run this command in the container using CMD

	// Backup the config so we can restore it later
	origState := s
	defer func() {
		s = origState
	}()

	s.Config.Cmd = cmd
	s.Config.Entrypoint = []string{}
	s.Config.Tty = true
	s.Config.OpenStdin = true
	s.Config.StdinOnce = true
	s.Config.AttachStdin = true
	s.Config.AttachStderr = true
	s.Config.AttachStdout = true

	if s.NoCache.ContainerID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	if err = b.client.RunContainer(s.NoCache.ContainerID, true); err != nil {
		b.client.RemoveContainer(s.NoCache.ContainerID)
		return s, err
	}

	return s, nil
}

// CommandEnv implements ENV
type CommandEnv struct {
	CommandBase
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandEnv) ReplaceEnv(env []string) error {
	return replaceEnv(c.cfg.args, env)
}

// Execute runs the command
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
	CommandBase
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandLabel) ReplaceEnv(env []string) error {
	return replaceEnv(c.cfg.args, env)
}

// Execute runs the command
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
	CommandBase
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandWorkdir) ReplaceEnv(env []string) error {
	return replaceEnv(c.cfg.args, env)
}

// Execute runs the command
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
	CommandBase
}

// Execute runs the command
func (c *CommandCmd) Execute(b *Build) (s State, err error) {
	s = b.state

	cmd := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	if !c.cfg.attrs["json"] {
		cmd = append([]string{"/bin/sh", "-c"}, cmd...)
	}

	s.Config.Cmd = cmd

	s.Commit(fmt.Sprintf("CMD %q", cmd))

	if len(c.cfg.args) != 0 {
		s.NoCache.CmdSet = true
	}

	return s, nil
}

// CommandEntrypoint implements ENTRYPOINT
type CommandEntrypoint struct {
	CommandBase
}

// Execute runs the command
func (c *CommandEntrypoint) Execute(b *Build) (s State, err error) {
	s = b.state

	parsed := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	switch {
	case c.cfg.attrs["json"]:
		// ENTRYPOINT ["echo", "hi"]
		s.Config.Entrypoint = parsed
	case len(parsed) == 0:
		// ENTRYPOINT []
		s.Config.Entrypoint = []string{}
	default:
		// ENTRYPOINT echo hi
		s.Config.Entrypoint = []string{"/bin/sh", "-c", parsed[0]}
	}

	s.Commit(fmt.Sprintf("ENTRYPOINT %q", s.Config.Entrypoint))

	// TODO: test this
	// when setting the entrypoint if a CMD was not explicitly set then
	// set the command to nil
	if !s.NoCache.CmdSet {
		s.Config.Cmd = nil
	}

	return s, nil
}

// CommandExpose implements EXPOSE
type CommandExpose struct {
	CommandBase
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandExpose) ReplaceEnv(env []string) error {
	return replaceEnv(c.cfg.args, env)
}

// Execute runs the command
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
	CommandBase
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandVolume) ReplaceEnv(env []string) error {
	return replaceEnv(c.cfg.args, env)
}

// Execute runs the command
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
	CommandBase
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandUser) ReplaceEnv(env []string) error {
	return replaceEnv(c.cfg.args, env)
}

// Execute runs the command
func (c *CommandUser) Execute(b *Build) (s State, err error) {

	s = b.state

	if len(c.cfg.args) != 1 {
		return s, fmt.Errorf("USER requires exactly one argument")
	}

	s.Config.User = c.cfg.args[0]

	s.Commit(fmt.Sprintf("USER %v", c.cfg.args))

	return s, nil
}

// CommandOnbuild implements ONBUILD
type CommandOnbuild struct {
	CommandBase
}

// Execute runs the command
func (c *CommandOnbuild) Execute(b *Build) (s State, err error) {

	s = b.state

	if len(c.cfg.args) == 0 {
		return s, fmt.Errorf("ONBUILD requires at least one argument")
	}

	command := strings.ToUpper(strings.TrimSpace(c.cfg.args[0]))
	switch command {
	case "ONBUILD":
		return s, fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
	case "MAINTAINER", "FROM":
		return s, fmt.Errorf("%s isn't allowed as an ONBUILD trigger", command)
	}

	orig := regexp.MustCompile(`(?i)^\s*ONBUILD\s*`).ReplaceAllString(c.cfg.original, "")

	s.Config.OnBuild = append(s.Config.OnBuild, orig)
	s.Commit(fmt.Sprintf("ONBUILD %s", orig))

	return s, nil
}

// CommandTag implements TAG
type CommandTag struct {
	CommandBase
}

// Execute runs the command
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
	CommandBase
}

// Execute runs the command
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

	image := imagename.NewFromString(c.cfg.args[0])
	artifact := imagename.Artifact{
		Name:      image,
		Pushed:    b.cfg.Push,
		Tag:       image.GetTag(),
		ImageID:   b.state.ImageID,
		BuildTime: time.Now(),
	}

	// push image and add some lines to artifacts
	if b.cfg.Push {
		digest, err := b.client.PushImage(image.String())
		if err != nil {
			return b.state, err
		}
		artifact.SetDigest(digest)
	} else {
		log.Infof("| Don't push. Pass --push flag to actually push to the registry")
	}

	// Publish artifact files
	if b.cfg.ArtifactsPath != "" {
		if err := os.MkdirAll(b.cfg.ArtifactsPath, 0755); err != nil {
			return b.state, fmt.Errorf("Failed to create directory %s for the artifacts, error: %s", b.cfg.ArtifactsPath, err)
		}

		filePath := filepath.Join(b.cfg.ArtifactsPath, artifact.GetFileName())

		artifacts := imagename.Artifacts{
			RockerArtifacts: []imagename.Artifact{artifact},
		}
		content, err := yaml.Marshal(artifacts)
		if err != nil {
			return b.state, err
		}

		if err := ioutil.WriteFile(filePath, content, 0644); err != nil {
			return b.state, fmt.Errorf("Failed to write artifact file %s, error: %s", filePath, err)
		}

		log.Infof("| Saved artifact file %s", filePath)
		log.Debugf("Artifact properties: %# v", pretty.Formatter(artifact))
	}

	return b.state, nil
}

// CommandCopy implements COPY
type CommandCopy struct {
	CommandBase
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandCopy) ReplaceEnv(env []string) error {
	return replaceEnv(c.cfg.args, env)
}

// Execute runs the command
func (c *CommandCopy) Execute(b *Build) (State, error) {
	if len(c.cfg.args) < 2 {
		return b.state, fmt.Errorf("COPY requires at least two arguments")
	}
	return copyFiles(b, c.cfg.args, "COPY")
}

// CommandAdd implements ADD
// For now it is an alias of COPY, but later will add urls and archives to it
type CommandAdd struct {
	CommandBase
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandAdd) ReplaceEnv(env []string) error {
	return replaceEnv(c.cfg.args, env)
}

// Execute runs the command
func (c *CommandAdd) Execute(b *Build) (State, error) {
	if len(c.cfg.args) < 2 {
		return b.state, fmt.Errorf("ADD requires at least two arguments")
	}
	return addFiles(b, c.cfg.args)
}

// CommandMount implements MOUNT
type CommandMount struct {
	CommandBase
}

// Execute runs the command
func (c *CommandMount) Execute(b *Build) (s State, err error) {

	s = b.state

	if len(c.cfg.args) == 0 {
		return b.state, fmt.Errorf("MOUNT requires at least one argument")
	}

	commitIds := []string{}

	for _, arg := range c.cfg.args {

		switch strings.Contains(arg, ":") {
		// MOUNT src:dest
		case true:
			var (
				pair = strings.SplitN(arg, ":", 2)
				src  = pair[0]
				dest = pair[1]
				err  error
			)

			// Process relative paths in volumes
			if strings.HasPrefix(src, "~") {
				src = strings.Replace(src, "~", os.Getenv("HOME"), 1)
			}
			if !path.IsAbs(src) {
				src = path.Join(b.cfg.ContextDir, src)
			}

			if src, err = b.client.ResolveHostPath(src); err != nil {
				return s, err
			}

			if s.NoCache.HostConfig.Binds == nil {
				s.NoCache.HostConfig.Binds = []string{}
			}

			s.NoCache.HostConfig.Binds = append(s.NoCache.HostConfig.Binds, src+":"+dest)
			commitIds = append(commitIds, arg)

		// MOUNT dir
		case false:
			if !path.IsAbs(arg) {
				return s, fmt.Errorf("Invalid volume destination path: '%s', mount path must be absolute", arg)
			}
			c, err := b.getVolumeContainer(arg)
			if err != nil {
				return s, err
			}

			if s.NoCache.HostConfig.Binds == nil {
				s.NoCache.HostConfig.Binds = []string{}
			}

			s.NoCache.HostConfig.Binds = append(s.NoCache.HostConfig.Binds,
				mountsToBinds(c.Mounts, "")...)

			commitIds = append(commitIds, strings.TrimLeft(c.Name, "/")+":"+arg)
		}
	}

	s.Commit(fmt.Sprintf("MOUNT %q", commitIds))

	return s, nil
}

// CommandExport implements EXPORT
type CommandExport struct {
	CommandBase
}

// Execute runs the command
func (c *CommandExport) Execute(b *Build) (s State, err error) {

	s = b.state
	args := c.cfg.args

	if len(args) == 0 {
		return s, fmt.Errorf("EXPORT requires at least one argument")
	}

	// If only one argument was given to EXPORT, use basename of a file
	// EXPORT /my/dir/file.tar --> /EXPORT_VOLUME/file.tar
	if len(args) < 2 {
		args = []string{args[0], "/"}
	}

	src := args[0 : len(args)-1]
	dest := args[len(args)-1] // last one is always the dest

	// EXPORT /my/dir my_dir --> /EXPORT_VOLUME/my_dir
	// EXPORT /my/dir /my_dir --> /EXPORT_VOLUME/my_dir
	// EXPORT /my/dir stuff/ --> /EXPORT_VOLUME/stuff/my_dir
	// EXPORT /my/dir /stuff/ --> /EXPORT_VOLUME/stuff/my_dir
	// EXPORT /my/dir/* / --> /EXPORT_VOLUME/stuff/my_dir

	s.Commit("EXPORT %q to %s, prev_export_container_salt: %s", src, dest, b.prevExportContainerID)

	// build the command
	cmdDestPath, err := util.ResolvePath(ExportsPath, dest)
	if err != nil {
		return s, fmt.Errorf("Invalid EXPORT destination: %s", dest)
	}

	s, hit, err := b.probeCacheAndPreserveCommits(s)
	if err != nil {
		return s, err
	}
	if hit {
		b.prevExportContainerID = s.ExportsID
		b.currentExportContainerName = exportsContainerName(s.ParentID, s.GetCommits())
		log.Infof("| Export container: %s", b.currentExportContainerName)
		log.Debugf("===EXPORT CONTAINER NAME: %s ('%s', '%s')", b.currentExportContainerName, s.ParentID, s.GetCommits())
		s.CleanCommits()
		return s, nil
	}

	prevExportContainerName := b.currentExportContainerName
	b.currentExportContainerName = exportsContainerName(s.ImageID, s.GetCommits())

	exportsContainer, err := b.getExportsContainerAndSync(b.currentExportContainerName, prevExportContainerName)
	if err != nil {
		return s, err
	}

	// Remember original stuff so we can restore it when we finished
	var exportsID string
	origState := s

	defer func() {
		s = origState
		s.ExportsID = exportsID
		b.prevExportContainerID = exportsID
	}()

	// Append exports container as a volume
	s.NoCache.HostConfig.Binds = append(s.NoCache.HostConfig.Binds,
		mountsToBinds(exportsContainer.Mounts, "")...)
	cmd := []string{"/opt/rsync/bin/rsync", "-a", "--delete-during"}

	if b.cfg.Verbose {
		cmd = append(cmd, "--verbose")
	}

	cmd = append(cmd, src...)
	cmd = append(cmd, cmdDestPath)

	s.Config.Cmd = cmd
	s.Config.Entrypoint = []string{}

	if exportsID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}
	defer b.client.RemoveContainer(exportsID)

	log.Infof("| Running in %.12s: %s", exportsID, strings.Join(cmd, " "))

	if err = b.client.RunContainer(exportsID, false); err != nil {
		return s, err
	}

	return s, nil
}

// CommandImport implements IMPORT
type CommandImport struct {
	CommandBase
}

// Execute runs the command
func (c *CommandImport) Execute(b *Build) (s State, err error) {
	s = b.state
	args := c.cfg.args

	if len(args) == 0 {
		return s, fmt.Errorf("IMPORT requires at least one argument")
	}
	if b.prevExportContainerID == "" {
		return s, fmt.Errorf("You have to EXPORT something first in order to IMPORT")
	}

	// TODO: EXPORT and IMPORT cache is not invalidated properly in between
	// 			 different tracks of the same build. The EXPORT may be cached
	// 			 because it was built earlier with the same prerequisites, but the actual
	// 			 data in the exports container may be from the latest EXPORT of different
	// 			 build. So we need to prefix ~/.rocker_exports dir with some id somehow.
	if b.currentExportContainerName == "" {
		return s, fmt.Errorf("You have to EXPORT something first to do IMPORT")
	}

	exportsContainer, err := b.getExportsContainer(b.currentExportContainerName)
	if err != nil {
		return s, err
	}

	log.Infof("| Import from %s (%.12s)", b.currentExportContainerName, exportsContainer.ID)

	// If only one argument was given to IMPORT, use the same path for destination
	// IMPORT /my/dir/file.tar --> ADD ./EXPORT_VOLUME/my/dir/file.tar /my/dir/file.tar
	if len(args) < 2 {
		args = []string{args[0], "/"}
	}
	dest := args[len(args)-1] // last one is always the dest
	src := []string{}

	for _, arg := range args[0 : len(args)-1] {
		argResolved, err := util.ResolvePath(ExportsPath, arg)
		if err != nil {
			return s, fmt.Errorf("Invalid IMPORT source: %s", arg)
		}
		src = append(src, argResolved)
	}

	s.Commit("IMPORT %q : %q %s", b.prevExportContainerID, src, dest)

	// Check cache
	s, hit, err := b.probeCache(s)
	if err != nil {
		return s, err
	}
	if hit {
		return s, nil
	}

	// Remember original stuff so we can restore it when we finished
	origState := s

	var importID string

	defer func() {
		s = origState
		s.NoCache.ContainerID = importID
	}()

	cmd := []string{"/opt/rsync/bin/rsync", "-a"}

	if b.cfg.Verbose {
		cmd = append(cmd, "--verbose")
	}

	cmd = append(cmd, src...)
	cmd = append(cmd, dest)

	s.Config.Cmd = cmd
	s.Config.Entrypoint = []string{}

	// Append exports container as a volume
	s.NoCache.HostConfig.Binds = append(s.NoCache.HostConfig.Binds,
		mountsToBinds(exportsContainer.Mounts, "")...)

	if importID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	log.Infof("| Running in %.12s: %s", importID, strings.Join(cmd, " "))

	if err = b.client.RunContainer(importID, false); err != nil {
		return s, err
	}

	// TODO: if b.exportsCacheBusted and IMPORT cache was invalidated,
	// 			 CommitCommand then caches it anyway.

	return s, nil
}

// CommandArg implements ARG
type CommandArg struct {
	CommandBase
}

// Execute runs the command
func (c *CommandArg) Execute(b *Build) (s State, err error) {
	s = b.state
	args := c.cfg.args

	if len(args) != 1 {
		return s, fmt.Errorf("ARG requires exactly one argument definition")
	}

	var (
		name       string
		value      string
		hasDefault bool

		arg = args[0]
	)

	// Borrowed from Docker source:
	// 'arg' can just be a name or name-value pair. Note that this is different
	// from 'env' that handles the split of name and value at the parser level.
	// The reason for doing it differently for 'arg' is that we support just
	// defining an arg and not assign it a value (while 'env' always expects a
	// name-value pair). If possible, it will be good to harmonize the two.
	if strings.Contains(arg, "=") {
		parts := strings.SplitN(arg, "=", 2)
		name = parts[0]
		value = parts[1]
		hasDefault = true
	} else {
		name = arg
		hasDefault = false
	}
	// add the arg to allowed list of build-time args from this step on.
	b.allowedBuildArgs[name] = true

	// If there is a default value associated with this arg then add it to the
	// b.buildArgs if one is not already passed to the builder. The args passed
	// to builder override the default value of 'arg'.
	if _, ok := s.NoCache.BuildArgs[name]; !ok && hasDefault {
		s.NoCache.BuildArgs[name] = value
	}

	s.Commit("ARG %s", arg)

	return s, nil
}

// CommandOnbuildWrap wraps ONBUILD command
type CommandOnbuildWrap struct {
	cmd Command
}

// String returns the human readable string representation of the command
func (c *CommandOnbuildWrap) String() string {
	return "ONBUILD " + c.cmd.String()
}

// ShouldRun returns true if the command should be executed
func (c *CommandOnbuildWrap) ShouldRun(b *Build) (bool, error) {
	return true, nil
}

// Execute runs the command
func (c *CommandOnbuildWrap) Execute(b *Build) (State, error) {
	return c.cmd.Execute(b)
}

// ReplaceEnv implements EnvReplacableCommand interface
func (c *CommandOnbuildWrap) ReplaceEnv(env []string) error {
	if command, ok := c.cmd.(EnvReplacableCommand); ok {
		return command.ReplaceEnv(env)
	}
	return nil
}

////////// Private stuff //////////

func replaceEnv(args []string, env []string) (err error) {

	defaultEnv := []string{"PATH=" + DefaultPathEnv}

	env1 := replaceOrAppendEnvValues(defaultEnv, env)

	for i, v := range args {
		if args[i], err = shellparser.ProcessWord(v, env1); err != nil {
			return err
		}
	}
	return nil
}
