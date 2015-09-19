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
	Execute(b *Build) error
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

func (c *CommandFrom) Execute(b *Build) (err error) {
	// TODO: for "scratch" image we may use /images/create

	if len(c.cfg.args) != 1 {
		return fmt.Errorf("FROM requires one argument")
	}

	var (
		img  *docker.Image
		name = c.cfg.args[0]
	)

	if img, err = b.client.InspectImage(name); err != nil {
		return err
	}

	if img == nil {
		if err = b.client.PullImage(name); err != nil {
			return err
		}
		if img, err = b.client.InspectImage(name); err != nil {
			return err
		}
		if img == nil {
			return fmt.Errorf("FROM: Failed to inspect image after pull: %s", name)
		}
	}

	b.imageID = img.ID

	return nil
}

type CommandReset struct{}

func (c *CommandReset) Execute(b *Build) error {
	return nil
}

type CommandCommit struct{}

func (c *CommandCommit) Execute(b *Build) error {
	return nil
}

type CommandRun struct {
	cfg ConfigCommand
}

func (c *CommandRun) Execute(b *Build) error {
	return nil
}

type CommandEnv struct {
	cfg ConfigCommand
}

func (c *CommandEnv) Execute(b *Build) error {
	return nil
}

type CommandTag struct {
	cfg ConfigCommand
}

func (c *CommandTag) Execute(b *Build) error {
	return nil
}

type CommandCopy struct {
	cfg ConfigCommand
}

func (c *CommandCopy) Execute(b *Build) error {
	return nil
}
