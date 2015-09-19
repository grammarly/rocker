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
	"io"
	"rocker/template"

	"github.com/fsouza/go-dockerclient"
)

var (
	NoBaseImageSpecifier = "scratch"
)

type BuildConfig struct {
	OutStream  io.Writer
	InStream   io.ReadCloser
	Auth       *docker.AuthConfiguration
	Vars       template.Vars
	ContextDir string
	Pull       bool
}

type Build struct {
	rockerfile *Rockerfile
	cfg        BuildConfig
	container  *docker.Config
	client     Client

	imageID string
}

func New(client Client, rockerfile *Rockerfile, cfg BuildConfig) (b *Build, err error) {
	b = &Build{
		rockerfile: rockerfile,
		cfg:        cfg,
		client:     client,
	}

	return b, nil
}

func (b *Build) Run(plan Plan) error {
	for k, c := range plan {
		fmt.Printf("Step %d: %q\n", k, c)
		if err := c.Execute(b); err != nil {
			return err
		}
	}
	return nil
}
