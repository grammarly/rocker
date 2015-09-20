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

	"github.com/fsouza/go-dockerclient"
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
	case "run":
		return &CommandRun{cfg}, nil
	case "env":
		return &CommandEnv{cfg}, nil
	case "tag":
		return &CommandTag{cfg}, nil
	case "copy":
		return &CommandCopy{cfg}, nil
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

func (c *CommandFrom) Execute(b *Build) (state State, err error) {
	// TODO: for "scratch" image we may use /images/create

	if len(c.cfg.args) != 1 {
		return state, fmt.Errorf("FROM requires one argument")
	}

	var (
		img  *docker.Image
		name = c.cfg.args[0]
	)

	// If Pull is true, then img will remain nil and it will be pulled below
	if !b.cfg.Pull {
		if img, err = b.client.InspectImage(name); err != nil {
			return state, err
		}
	}

	if img == nil {
		if err = b.client.PullImage(name); err != nil {
			return state, err
		}
		if img, err = b.client.InspectImage(name); err != nil {
			return state, err
		}
		if img == nil {
			return state, fmt.Errorf("FROM: Failed to inspect image after pull: %s", name)
		}
	}

	state = b.state
	state.imageID = img.ID
	state.container = *img.Config

	return state, nil
}

// CommandReset cleans the builder state before the next FROM
type CommandReset struct{}

func (c *CommandReset) String() string {
	return "Cleaning up state before the next FROM"
}

func (c *CommandReset) Execute(b *Build) (State, error) {
	state := b.state
	state.imageID = ""
	return state, nil
}

// CommandCommit commits collected changes
type CommandCommit struct{}

func (c *CommandCommit) String() string {
	return "Committing changes"
}

func (c *CommandCommit) Execute(b *Build) (State, error) {
	return b.state, nil
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

	if s.imageID == "" {
		return s, fmt.Errorf("Please provide a source image with `FROM` prior to run")
	}

	cmd := handleJSONArgs(c.cfg.args, c.cfg.attrs)

	if !c.cfg.attrs["json"] {
		cmd = append([]string{"/bin/sh", "-c"}, cmd...)
	}

	// TODO: test with ENTRYPOINT

	// We run this command in the container using CMD
	origCmd := s.container.Cmd
	s.container.Cmd = cmd

	// Restore command after commit
	s.postCommit = func(s State) (State, error) {
		s.container.Cmd = origCmd
		return s, nil
	}

	if s.containerID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	if err = b.client.RunContainer(s.containerID, false); err != nil {
		return s, err
	}

	return s, nil
}

// CommandEnv implements ENV
type CommandEnv struct {
	cfg ConfigCommand
}

func (c *CommandEnv) String() string {
	return c.cfg.original
}

func (c *CommandEnv) Execute(b *Build) (State, error) {
	return b.state, nil
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
	return b.state, nil
}
