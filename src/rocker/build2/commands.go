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

type CommandFrom struct {
	cfg ConfigCommand
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

type CommandReset struct{}

func (c *CommandReset) Execute(b *Build) (State, error) {
	return b.state, nil
}

type CommandCommit struct{}

func (c *CommandCommit) Execute(b *Build) (State, error) {
	return b.state, nil
}

type CommandRun struct {
	cfg ConfigCommand
}

func (c *CommandRun) Execute(b *Build) (State, error) {
	return b.state, nil
}

type CommandEnv struct {
	cfg ConfigCommand
}

func (c *CommandEnv) Execute(b *Build) (State, error) {
	return b.state, nil
}

type CommandTag struct {
	cfg ConfigCommand
}

func (c *CommandTag) Execute(b *Build) (State, error) {
	return b.state, nil
}

type CommandCopy struct {
	cfg ConfigCommand
}

func (c *CommandCopy) Execute(b *Build) (State, error) {
	return b.state, nil
}
