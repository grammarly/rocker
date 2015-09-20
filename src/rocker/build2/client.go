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
)

type Client interface {
	InspectImage(name string) (*docker.Image, error)
	PullImage(name string) error
}

type DockerClientConfig struct {
	Client    *docker.Client
	OutStream io.Writer
	InStream  io.ReadCloser
	Auth      *docker.AuthConfiguration
}

type DockerClient struct {
	client *docker.Client
	cfg    DockerClientConfig
}

func NewDockerClient(dockerClient *docker.Client, cfg DockerClientConfig) *DockerClient {
	return &DockerClient{
		client: dockerClient,
		cfg:    cfg,
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
		fdOut, isTerminalOut   = term.GetFdInfo(c.cfg.OutStream)
		image                  = imagename.NewFromString(name)
		pipeReader, pipeWriter = io.Pipe()
		errch                  = make(chan error)
	)

	pullOpts := docker.PullImageOptions{
		Repository:    image.NameWithRegistry(),
		Registry:      image.Registry,
		Tag:           image.GetTag(),
		OutputStream:  pipeWriter,
		RawJSONStream: true,
	}

	go func() {
		err := c.client.PullImage(pullOpts, *c.cfg.Auth)

		if err := pipeWriter.Close(); err != nil {
			// TODO: logrus error
			fmt.Printf("pipeWriter.Close() err: %s\n", err)
		}

		errch <- err
	}()

	if err := jsonmessage.DisplayJSONMessagesStream(pipeReader, c.cfg.OutStream, fdOut, isTerminalOut); err != nil {
		return fmt.Errorf("Failed to process json stream for image: %s, error: %s", image, err)
	}

	if err := <-errch; err != nil {
		return fmt.Errorf("Failed to pull image: %s, error: %s", image, err)
	}

	return nil
}
