// +build integration

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
	"bytes"
	"github.com/grammarly/rocker/src/dockerclient"
	"github.com/grammarly/rocker/src/template"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/kr/pretty"
)

func TestInteg_Build_Mount(t *testing.T) {
	b, _, c := runBuildInteg(t, `
FROM alpine:3.2
MOUNT /data
RUN touch /data/file
RUN ls /data/file
`, Config{})

	defer func() {
		if err := c.client.RemoveImageExtended(b.state.ImageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()
}

func TestInteg_Build_Export(t *testing.T) {
	b, _, c := runBuildInteg(t, `
FROM alpine:3.2
RUN touch file
EXPORT file
EXPORT file
IMPORT file /etc/
IMPORT file /etc/
RUN ls /etc/file
`, Config{})

	defer func() {
		if err := c.client.RemoveImageExtended(b.state.ImageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	pretty.Println(b.state)
}

// internal helpers

func runBuildInteg(t *testing.T, rockerfileContent string, cfg Config) (*Build, string, *DockerClient) {
	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)

	r, err := NewRockerfile(fn.Name(), strings.NewReader(rockerfileContent), template.Vars{}, template.Funs{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.NoCache = true

	dockerCli, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	logger := logrus.New()
	logger.Out = io.MultiWriter(&buf, os.Stdout)

	c := NewDockerClient(dockerCli, docker.AuthConfiguration{}, logger)
	b := New(c, r, nil, cfg)

	defer func() {
		dockerCli.RemoveContainer(docker.RemoveContainerOptions{
			ID:            b.exportsContainerName(),
			Force:         true,
			RemoveVolumes: true,
		})
	}()

	p, err := NewPlan(r.Commands(), true)
	if err != nil {
		t.Fatal(err)
	}

	if err := b.Run(p); err != nil {
		t.Fatal(err)
	}

	return b, buf.String(), c
}
