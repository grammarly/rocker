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
	"rocker/imagename"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/fsouza/go-dockerclient"

	log "github.com/Sirupsen/logrus"
)

type Client interface {
	InspectImage(name string) (*docker.Image, error)
	PullImage(name string) error
	CreateContainer(state State) (id string, err error)
	RunContainer(containerID string, attach bool) error
}

type DockerClient struct {
	client *docker.Client
	auth   docker.AuthConfiguration
}

func NewDockerClient(dockerClient *docker.Client, auth docker.AuthConfiguration) *DockerClient {
	return &DockerClient{
		client: dockerClient,
		auth:   auth,
	}
}

func (c *DockerClient) InspectImage(name string) (*docker.Image, error) {
	img, err := c.client.InspectImage(name)
	// We simply return nil in case image not found
	if err == docker.ErrNoSuchImage {
		return nil, nil
	}
	return img, err
}

func (c *DockerClient) PullImage(name string) error {

	var (
		image                  = imagename.NewFromString(name)
		pipeReader, pipeWriter = io.Pipe()
		def                    = log.StandardLogger()
		fdOut, isTerminalOut   = term.GetFdInfo(def.Out)
		out                    = def.Out
		errch                  = make(chan error)
	)

	if !isTerminalOut {
		out = def.Writer()
	}

	pullOpts := docker.PullImageOptions{
		Repository:    image.NameWithRegistry(),
		Registry:      image.Registry,
		Tag:           image.GetTag(),
		OutputStream:  pipeWriter,
		RawJSONStream: true,
	}

	go func() {
		err := c.client.PullImage(pullOpts, c.auth)

		if err := pipeWriter.Close(); err != nil {
			log.Errorf("pipeWriter.Close() err: %s", err)
		}

		errch <- err
	}()

	if err := jsonmessage.DisplayJSONMessagesStream(pipeReader, out, fdOut, isTerminalOut); err != nil {
		return fmt.Errorf("Failed to process json stream for image: %s, error: %s", image, err)
	}

	if err := <-errch; err != nil {
		return fmt.Errorf("Failed to pull image: %s, error: %s", image, err)
	}

	return nil
}

func (c *DockerClient) CreateContainer(state State) (string, error) {
	// volumesFrom := builder.getMountContainerIds()
	// binds := builder.getBinds()

	state.container.Image = state.imageID

	// TODO: assign human readable name?

	opts := docker.CreateContainerOptions{
		Config:     &state.container,
		HostConfig: &docker.HostConfig{
		// Binds:       binds,
		// VolumesFrom: volumesFrom,
		},
	}

	container, err := c.client.CreateContainer(opts)
	if err != nil {
		return "", err
	}

	log.Infof("  ---> Created container %.12s (image id = %.12s)", container.ID, state.imageID)

	return container.ID, nil
}

func (c *DockerClient) RunContainer(containerID string, attach bool) error {
	return fmt.Errorf("RunContainer not implemented yet")
}
