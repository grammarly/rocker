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

type Client interface {
	InspectImage(name string) (*docker.Image, error)
	PullImage(name string) error
}

type DockerClient struct {
	client *docker.Client
}

func NewDockerClient(dockerClient *docker.Client) *DockerClient {
	return &DockerClient{
		client: dockerClient,
	}
}

func (c *DockerClient) InspectImage(name string) (*docker.Image, error) {
	return c.client.InspectImage(name)
}

func (c *DockerClient) PullImage(name string) error {
	return fmt.Errorf("PullImage not implemented yet")
}
