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
	"os/signal"

	"rocker/dockerclient"
	"rocker/imagename"

	"github.com/docker/docker/pkg/units"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/fsouza/go-dockerclient"
	"github.com/kr/pretty"

	log "github.com/Sirupsen/logrus"
)

type Client interface {
	InspectImage(name string) (*docker.Image, error)
	PullImage(name string) error
	RemoveImage(imageID string) error
	TagImage(imageID, imageName string) error
	PushImage(imageName string) error
	EnsureImage(imageName string) error
	CreateContainer(state State) (id string, err error)
	RunContainer(containerID string, attach bool) error
	CommitContainer(state State, message string) (imageID string, err error)
	RemoveContainer(containerID string) error
	UploadToContainer(containerID string, stream io.Reader, path string) error
	EnsureContainer(containerName string, config *docker.Config, purpose string) (containerID string, err error)
	ResolveHostPath(path string) (resultPath string, err error)
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

	opts := docker.PullImageOptions{
		Repository:    image.NameWithRegistry(),
		Registry:      image.Registry,
		Tag:           image.GetTag(),
		OutputStream:  pipeWriter,
		RawJSONStream: true,
	}

	log.Infof("| Pull image %s", image)
	log.Debugf("Pull image %s with options: %# v", image, opts)

	go func() {
		errch <- jsonmessage.DisplayJSONMessagesStream(pipeReader, out, fdOut, isTerminalOut)
	}()

	if err := c.client.PullImage(opts, c.auth); err != nil {
		return err
	}

	return <-errch
}

func (c *DockerClient) RemoveImage(imageID string) error {
	log.Infof("| Remove image %.12s", imageID)

	opts := docker.RemoveImageOptions{
		Force:   true,
		NoPrune: false,
	}
	return c.client.RemoveImageExtended(imageID, opts)
}

func (c *DockerClient) CreateContainer(s State) (string, error) {

	s.Config.Image = s.ImageID

	// TODO: assign human readable name?

	opts := docker.CreateContainerOptions{
		Config:     &s.Config,
		HostConfig: &s.HostConfig,
	}

	log.Debugf("Create container: %# v", pretty.Formatter(opts))

	container, err := c.client.CreateContainer(opts)
	if err != nil {
		return "", err
	}

	log.Infof("| Created container %.12s (image %.12s)", container.ID, s.ImageID)

	return container.ID, nil
}

func (c *DockerClient) RunContainer(containerID string, attachStdin bool) error {

	var (
		success  = make(chan struct{})
		finished = make(chan struct{}, 1)
		sigch    = make(chan os.Signal, 1)
		errch    = make(chan error)

		// Wrap output streams with logger
		def       = log.StandardLogger()
		outLogger = &log.Logger{
			Out:       def.Out,
			Formatter: NewContainerFormatter(containerID, log.InfoLevel),
			Level:     def.Level,
		}
		errLogger = &log.Logger{
			Out:       def.Out,
			Formatter: NewContainerFormatter(containerID, log.ErrorLevel),
			Level:     def.Level,
		}

		in                 = os.Stdin
		fdIn, isTerminalIn = term.GetFdInfo(in)
	)

	attachOpts := docker.AttachToContainerOptions{
		Container:    containerID,
		OutputStream: outLogger.Writer(),
		ErrorStream:  errLogger.Writer(),
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		Success:      success,
	}

	// Used by ATTACH
	if attachStdin {
		log.Infof("| Attach stdin to the container %.12s", containerID)

		if !isTerminalIn {
			return fmt.Errorf("Cannot attach to a container on non tty input")
		}

		attachOpts.InputStream = readerVoidCloser{in}
		attachOpts.OutputStream = os.Stdout
		attachOpts.ErrorStream = os.Stderr
		attachOpts.Stdin = true
		attachOpts.RawTerminal = true
	}

	// We want do debug the final attach options before setting raw term
	log.Debugf("Attach to container with options: %# v", attachOpts)

	if attachStdin {
		oldState, err := term.SetRawTerminal(fdIn)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(fdIn, oldState)
	}

	go func() {
		if err := c.client.AttachToContainer(attachOpts); err != nil {
			select {
			case <-finished:
				// Ignore any attach errors when we have finished already.
				// It may happen if we attach stdin, then container exit, but then there is other input from stdin continues.
				// This is the case when multiple ATTACH command are used in a single Rockerfile.
				// The problem though is that we cannot close stdin, to have it available for the subsequent ATTACH;
				// therefore, hijack goroutine from the previous ATTACH will hang until the input received and then
				// it will fire an error.
				// It's ok for `rocker` since it is not a daemon, but rather a one-off command.
				//
				// Also, there is still a problem that `rocker` loses second character from the Stdin in a second ATTACH.
				// But let's consider it a corner case.
			default:
				// Print the error. We cannot return it because the main routine is handing on WaitContaienr
				log.Errorf("Got error while attaching to container %.12s: %s", containerID, err)
			}
		}
	}()

	success <- <-success

	// TODO: support options for container resources constraints like `docker build` has

	if err := c.client.StartContainer(containerID, &docker.HostConfig{}); err != nil {
		return err
	}

	if attachStdin {
		if err := c.monitorTtySize(containerID, os.Stdout); err != nil {
			return fmt.Errorf("Failed to monitor TTY size for container %.12s, error: %s", containerID, err)
		}
	}

	// TODO: move signal handling to the builder?

	signal.Notify(sigch, os.Interrupt)

	go func() {
		statusCode, err := c.client.WaitContainer(containerID)
		// log.Debugf("Wait finished, status %q error %q", statusCode, err)
		if err != nil {
			errch <- err
		} else if statusCode != 0 {
			// Remove errored container
			// TODO: make option to keep them
			if err := c.RemoveContainer(containerID); err != nil {
				log.Error(err)
			}

			errch <- fmt.Errorf("Failed to run container, exit with code %d", statusCode)
		}
		errch <- nil
		return
	}()

	select {
	case err := <-errch:
		// indicate 'finished' so the `attach` goroutine will not give any errors
		finished <- struct{}{}
		if err != nil {
			return err
		}
	case <-sigch:
		// TODO: Removing container twice for some reason
		log.Infof("Received SIGINT, remove current container...")
		if err := c.RemoveContainer(containerID); err != nil {
			log.Errorf("Failed to remove container: %s", err)
		}
		// TODO: send signal to builder.Run() and have a proper cleanup
		os.Exit(2)
	}

	return nil
}

func (c *DockerClient) CommitContainer(s State, message string) (string, error) {
	commitOpts := docker.CommitContainerOptions{
		Container: s.ContainerID,
		Message:   message,
		Run:       &s.Config,
	}

	log.Debugf("Commit container: %# v", pretty.Formatter(commitOpts))

	image, err := c.client.CommitContainer(commitOpts)
	if err != nil {
		return "", err
	}

	// Inspect the image to get the real size
	log.Debugf("Inspect image %s", image.ID)

	if image, err = c.client.InspectImage(image.ID); err != nil {
		return "", err
	}

	size := fmt.Sprintf("%s (+%s)",
		units.HumanSize(float64(image.VirtualSize)),
		units.HumanSize(float64(image.Size)),
	)

	log.WithFields(log.Fields{
		"size": size,
	}).Infof("| Result image is %.12s", image.ID)

	return image.ID, nil
}

func (c *DockerClient) RemoveContainer(containerID string) error {
	log.Infof("| Removing container %.12s", containerID)

	opts := docker.RemoveContainerOptions{
		ID:            containerID,
		Force:         true,
		RemoveVolumes: true,
	}

	return c.client.RemoveContainer(opts)
}

func (c *DockerClient) UploadToContainer(containerID string, stream io.Reader, path string) error {
	log.Infof("| Uploading files to container %.12s", containerID)

	opts := docker.UploadToContainerOptions{
		InputStream:          stream,
		Path:                 path,
		NoOverwriteDirNonDir: false,
	}

	return c.client.UploadToContainer(containerID, opts)
}

func (c *DockerClient) TagImage(imageID, imageName string) error {
	img := imagename.NewFromString(imageName)

	log.Infof("| Tag %.12s -> %s", imageID, img)

	opts := docker.TagImageOptions{
		Repo:  img.NameWithRegistry(),
		Tag:   img.GetTag(),
		Force: true,
	}

	log.Debugf("Tag image %s with options: %# v", imageID, opts)

	return c.client.TagImage(imageID, opts)
}

func (c *DockerClient) PushImage(imageName string) error {
	var (
		img   = imagename.NewFromString(imageName)
		errch = make(chan error)

		pipeReader, pipeWriter = io.Pipe()
		def                    = log.StandardLogger()
		fdOut, isTerminalOut   = term.GetFdInfo(def.Out)
		out                    = def.Out

		opts = docker.PushImageOptions{
			Name:          img.NameWithRegistry(),
			Tag:           img.GetTag(),
			Registry:      img.Registry,
			OutputStream:  pipeWriter,
			RawJSONStream: true,
		}
	)

	if !isTerminalOut {
		out = def.Writer()
	}

	log.Infof("| Push %s", img)

	log.Debugf("Push with options: %# v", opts)

	go func() {
		errch <- jsonmessage.DisplayJSONMessagesStream(pipeReader, out, fdOut, isTerminalOut)
	}()

	if err := c.client.PushImage(opts, c.auth); err != nil {
		return err
	}

	return <-errch
}

func (c *DockerClient) ResolveHostPath(path string) (resultPath string, err error) {
	return dockerclient.ResolveHostPath(path, c.client)
}

func (c *DockerClient) EnsureImage(imageName string) (err error) {

	var img *docker.Image
	if img, err = c.client.InspectImage(imageName); err != nil && err != docker.ErrNoSuchImage {
		return err
	}
	if img != nil {
		return nil
	}

	return c.PullImage(imageName)
}

func (c *DockerClient) EnsureContainer(containerName string, config *docker.Config, purpose string) (containerID string, err error) {

	// Check if container exists
	container, err := c.client.InspectContainer(containerName)

	if _, ok := err.(*docker.NoSuchContainer); !ok && err != nil {
		return "", err
	}
	if container != nil {
		return container.ID, nil
	}

	// No data volume container for this build, create it

	if err := c.EnsureImage(config.Image); err != nil {
		return "", fmt.Errorf("Failed to check image %s, error: %s", config.Image, err)
	}

	log.Infof("| Create container: %s for %s", containerName, purpose)

	opts := docker.CreateContainerOptions{
		Name:   containerName,
		Config: config,
	}

	log.Debugf("Create container options %# v", opts)

	container, err = c.client.CreateContainer(opts)
	if err != nil {
		return "", fmt.Errorf("Failed to create container %s from image %s, error: %s", containerName, config.Image, err)
	}

	return container.ID, err
}
