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
	"fmt"
	"os"
	"os/signal"

	"rocker/util"

	"github.com/docker/docker/pkg/term"
	"github.com/fsouza/go-dockerclient"
)

func (builder *Builder) runAndCommit(cmd []string, comment string) error {
	// set Cmd manually, this is special case only for Dockerfiles
	origCmd := builder.Config.Cmd
	clearFunc := builder.temporaryCmd(cmd)
	defer clearFunc()

	hit, err := builder.probeCache()
	if err != nil {
		return err
	}
	if hit {
		return nil
	}

	containerID, err := builder.createContainer("")
	if err != nil {
		return fmt.Errorf("Failed to create container, error: %s", err)
	}
	defer func() {
		if err2 := builder.removeContainer(containerID); err2 != nil && err == nil {
			err = err2
		}
	}()

	if err := builder.runContainer(containerID); err != nil {
		return fmt.Errorf("Failed to run container %s, error: %s", containerID, err)
	}

	return builder.commitContainer(containerID, origCmd, comment)
}

func (builder *Builder) createContainer(name string) (string, error) {
	volumesFrom := builder.getMountContainerIds()
	binds := builder.getBinds()

	builder.Config.Image = builder.imageID

	opts := docker.CreateContainerOptions{
		Name:   name,
		Config: builder.Config,
		HostConfig: &docker.HostConfig{
			Binds:       binds,
			VolumesFrom: volumesFrom,
		},
	}

	container, err := builder.Docker.CreateContainer(opts)
	if err != nil {
		return "", err
	}

	fmt.Fprintf(builder.OutStream, "[Rocker]  ---> Running in %.12s (image id = %.12s)\n", container.ID, builder.imageID)

	return container.ID, nil
}

func (builder *Builder) removeContainer(containerID string) error {
	fmt.Fprintf(builder.OutStream, "[Rocker] Removing intermediate container %.12s\n", containerID)
	// TODO: always force?
	return builder.Docker.RemoveContainer(docker.RemoveContainerOptions{ID: containerID, Force: true})
}

func (builder *Builder) runContainer(containerID string) error {
	return builder.runContainerAttachStdin(containerID, false)
}

func (builder *Builder) runContainerAttachStdin(containerID string, attachStdin bool) error {
	success := make(chan struct{})

	attachOpts := docker.AttachToContainerOptions{
		Container:    containerID,
		OutputStream: util.PrefixPipe("[Docker] ", builder.OutStream),
		ErrorStream:  util.PrefixPipe("[Docker] ", builder.OutStream),
		Stdout:       true,
		Stderr:       true,
		Stream:       true,
		Success:      success,
	}

	if attachStdin {
		if !builder.isTerminalIn {
			return fmt.Errorf("Cannot attach to a container on non tty input")
		}
		oldState, err := term.SetRawTerminal(builder.fdIn)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(builder.fdIn, oldState)

		attachOpts.InputStream = builder.InStream
		attachOpts.OutputStream = builder.OutStream
		attachOpts.ErrorStream = builder.OutStream
		attachOpts.Stdin = true
		attachOpts.RawTerminal = true
	}

	go func() {
		if err := builder.Docker.AttachToContainer(attachOpts); err != nil {
			fmt.Fprintf(builder.OutStream, "Got error while attaching to container %s: %s\n", containerID, err)
		}
	}()

	success <- <-success

	if err := builder.Docker.StartContainer(containerID, &docker.HostConfig{}); err != nil {
		return err
	}

	if attachStdin {
		if err := builder.monitorTtySize(containerID); err != nil {
			return fmt.Errorf("Failed to monitor TTY size for container %s, error: %s", containerID, err)
		}
	}

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)

	errch := make(chan error)

	go func() {
		statusCode, err := builder.Docker.WaitContainer(containerID)
		if err != nil {
			errch <- err
		} else if statusCode != 0 {
			errch <- fmt.Errorf("Failed to run container, exit with code %d", statusCode)
		}
		errch <- nil
		return
	}()

	select {
	case err := <-errch:
		if err != nil {
			return err
		}
	case <-sigch:
		fmt.Fprintf(builder.OutStream, "[Rocker] Received SIGINT, remove current container...\n")
		if err := builder.removeContainer(containerID); err != nil {
			fmt.Fprintf(builder.OutStream, "[Rocker] Failed to remove container: %s\n", err)
		}
		// TODO: send signal to builder.Build() and have a proper cleanup
		os.Exit(2)
	}

	return nil
}

func (builder *Builder) commitContainer(containerID string, autoCmd []string, comment string) (err error) {

	if containerID == "" {
		clearFunc := builder.temporaryCmd([]string{"/bin/sh", "-c", "#(nop) " + comment})
		defer clearFunc()

		hit, err := builder.probeCache()
		if err != nil {
			return err
		}
		if hit {
			return nil
		}

		containerID, err = builder.createContainer("")
		if err != nil {
			return err
		}

		defer func() {
			if err2 := builder.removeContainer(containerID); err2 != nil && err == nil {
				err = err2
			}
		}()
	}

	// clone the struct
	autoConfig := *builder.Config
	autoConfig.Cmd = autoCmd

	commitOpts := docker.CommitContainerOptions{
		Container: containerID,
		Message:   "",
		Run:       &autoConfig,
	}

	image, err := builder.Docker.CommitContainer(commitOpts)
	if err != nil {
		return err
	}

	builder.imageID = image.ID

	return nil
}

func (builder *Builder) ensureContainer(containerName string, config *docker.Config, purpose string) (*docker.Container, error) {
	// Check if container exists
	container, err := builder.Docker.InspectContainer(containerName)

	// No data volume container for this build, create it
	if _, ok := err.(*docker.NoSuchContainer); ok {

		if err := builder.ensureImage(config.Image, purpose); err != nil {
			return container, fmt.Errorf("Failed to check image %s, error: %s", config.Image, err)
		}

		fmt.Fprintf(builder.OutStream, "[Rocker] Create container: %s for %s\n", containerName, purpose)

		createOpts := docker.CreateContainerOptions{
			Name:   containerName,
			Config: config,
		}

		container, err = builder.Docker.CreateContainer(createOpts)
		if err != nil {
			return container, fmt.Errorf("Failed to create container %s from image %s, error: %s", containerName, config.Image, err)
		}
	} else if err == nil {
		fmt.Fprintf(builder.OutStream, "[Rocker] Use existing container: %s for %s\n", containerName, purpose)
	}

	return container, err
}
