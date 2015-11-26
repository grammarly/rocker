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

package dockerclient

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

func TestNewDockerClient(t *testing.T) {
	cli, err := New()
	if err != nil {
		t.Fatal(err)
	}

	info, err := cli.Info()
	if err != nil {
		t.Fatal(err)
	}

	assert.IsType(t, &docker.Env{}, info)
}

func TestEntrypointOverride(t *testing.T) {
	t.Skip()

	cli, err := New()
	if err != nil {
		t.Fatal(err)
	}

	container, err := cli.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image:        "test-entrypoint-override",
			Entrypoint:   []string{},
			Cmd:          []string{"/bin/ls"},
			AttachStdout: true,
			AttachStderr: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cli.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, Force: true}); err != nil {
			t.Fatal(err)
		}
	}()

	success := make(chan struct{})
	var buf bytes.Buffer

	attachOpts := docker.AttachToContainerOptions{
		Container:    container.ID,
		OutputStream: &buf,
		ErrorStream:  &buf,
		Stream:       true,
		Stdout:       true,
		Stderr:       true,
		Success:      success,
	}
	go func() {
		if err := cli.AttachToContainer(attachOpts); err != nil {
			t.Fatal(fmt.Errorf("Attach container error: %s", err))
		}
	}()

	success <- <-success

	if err := cli.StartContainer(container.ID, &docker.HostConfig{}); err != nil {
		t.Fatal(fmt.Errorf("Failed to start container, error: %s", err))
	}

	statusCode, err := cli.WaitContainer(container.ID)
	if err != nil {
		t.Fatal(fmt.Errorf("Wait container error: %s", err))
	}

	println(buf.String())

	if statusCode != 0 {
		t.Fatal(fmt.Errorf("Failed to run container, exit with code %d", statusCode))
	}
}

func TestNewVolumesBug(t *testing.T) {
	cli, err := New()
	if err != nil {
		t.Fatal(err)
	}

	c1, out, err := runContainer(t, cli, &docker.Config{
		Image: "alpine:3.2",
		Cmd:   []string{"touch", "/data/file"},
		Volumes: map[string]struct{}{
			"/data": struct{}{},
		},
		AttachStdout: true,
		AttachStderr: true,
	}, &docker.HostConfig{})
	defer func() {
		if err := cli.RemoveContainer(docker.RemoveContainerOptions{ID: c1.ID, Force: true, RemoveVolumes: true}); err != nil {
			t.Fatal(err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("C1: %s   out: %s    err: %s\n", c1.ID, out, err)

	c2, out2, err := runContainer(t, cli, &docker.Config{
		Image:        "alpine:3.2",
		Cmd:          []string{"ls", "/data/file"},
		AttachStdout: true,
		AttachStderr: true,
	}, &docker.HostConfig{
		Binds: []string{c1.Mounts[0].Source + ":/data:ro"},
		// VolumesFrom: []string{c1},
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("C2: %s   out: %s    err: %s\n", c2.ID, out2, err)

	if err := cli.RemoveContainer(docker.RemoveContainerOptions{ID: c2.ID, Force: true, RemoveVolumes: true}); err != nil {
		t.Fatal(err)
	}

	c3, out3, err := runContainer(t, cli, &docker.Config{
		Image:        "alpine:3.2",
		Cmd:          []string{"ls", "/data/file"},
		AttachStdout: true,
		AttachStderr: true,
	}, &docker.HostConfig{
		Binds: []string{c1.Mounts[0].Source + ":/data:ro"},
		// VolumesFrom: []string{c1},
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("C3: %s   out: %s    err: %s\n", c3.ID, out3, err)

	if err := cli.RemoveContainer(docker.RemoveContainerOptions{ID: c3.ID, Force: true, RemoveVolumes: true}); err != nil {
		t.Fatal(err)
	}
}

func runContainer(t *testing.T, cli *docker.Client, cfg *docker.Config, hostCfg *docker.HostConfig) (*docker.Container, string, error) {
	container, err := cli.CreateContainer(docker.CreateContainerOptions{
		Config:     cfg,
		HostConfig: hostCfg,
	})
	if err != nil {
		return nil, "", err
	}

	var (
		buf     bytes.Buffer
		success = make(chan struct{})

		attachOpts = docker.AttachToContainerOptions{
			Container:    container.ID,
			OutputStream: &buf,
			ErrorStream:  &buf,
			Stream:       true,
			Stdout:       true,
			Stderr:       true,
			Success:      success,
		}
	)

	go func() {
		if err := cli.AttachToContainer(attachOpts); err != nil {
			t.Errorf("Attach container error: %s", err)
		}
	}()

	success <- <-success

	if err := cli.StartContainer(container.ID, &docker.HostConfig{}); err != nil {
		return container, "", err
	}

	statusCode, err := cli.WaitContainer(container.ID)
	if err != nil {
		return container, "", err
	}

	if statusCode != 0 {
		err = fmt.Errorf("Failed to run container, exit with code %d", statusCode)
	}

	c2, err := cli.InspectContainer(container.ID)
	if err != nil {
		return container, "", err
	}

	// pretty.Println(c2)

	return c2, buf.String(), err
}
