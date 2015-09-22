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
	"io"

	"github.com/fsouza/go-dockerclient"
	"github.com/kr/pretty"

	log "github.com/Sirupsen/logrus"
)

var (
	NoBaseImageSpecifier = "scratch"
)

type Config struct {
	OutStream  io.Writer
	InStream   io.ReadCloser
	ContextDir string
	Pull       bool
	NoGarbage  bool
}

type State struct {
	config      docker.Config
	imageID     string
	containerID string
	commitMsg   []string
	postCommit  func(s State) (s1 State, err error)
}

type Build struct {
	rockerfile *Rockerfile
	cfg        Config
	client     Client
	state      State
}

func New(client Client, rockerfile *Rockerfile, cfg Config) *Build {
	return &Build{
		rockerfile: rockerfile,
		cfg:        cfg,
		client:     client,
		state:      State{},
	}
}

func (b *Build) Run(plan Plan) (err error) {

	for k, c := range plan {

		log.Debugf("Step %d: %# v", k+1, pretty.Formatter(c))
		log.Infof("Step %d: %s", k+1, c)

		if b.state, err = c.Execute(b); err != nil {
			return err
		}

		log.Debugf("State after step %d: %# v", k+1, pretty.Formatter(b.state))
	}

	if b.cfg.NoGarbage && b.state.imageID != "" {
		if err := b.client.RemoveImage(b.state.imageID); err != nil {
			return err
		}
	}

	return nil
}

func (b *Build) GetState() State {
	return b.state
}

func (b *Build) GetImageID() string {
	return b.state.imageID
}
