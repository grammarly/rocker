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
	"os"
	"rocker/template"

	"github.com/fsouza/go-dockerclient"
)

type BuildConfig struct {
	Rockerfile string
	OutStream  io.Writer
	InStream   io.ReadCloser
	Auth       *docker.AuthConfiguration
	Vars       template.Vars
	ContextDir string
}

type Build struct {
	cfg               *BuildConfig
	client            *Client
	rockerfileContent string
}

func New(client *Client, cfg *BuildConfig) (*Build, error) {
	b := &Build{
		cfg:    cfg,
		client: &client,
	}

	fd, err := os.Open(b.cfg.Rockerfile)
	if err != nil {
		return fmt.Errorf("Failed to open file %s, error: %s", b.cfg.Rockerfile, err)
	}
	defer fd.Close()

	data, err := template.ProcessConfigTemplate(b.cfg.Rockerfile, fd, b.cfg.Vars, map[string]interface{}{})
	if err != nil {
		return err
	}
	b.rockerfileContent = data.String()

	// TODO: print

	return b, nil
}
