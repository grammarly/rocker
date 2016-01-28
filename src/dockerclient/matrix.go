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
	"fmt"
	"github.com/grammarly/rocker/src/util"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

const (
	initFile = "/.dockerinit"
)

// IsInMatrix returns true if current process is running inside of a docker container
func IsInMatrix() (bool, error) {
	_, err := os.Stat(initFile)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// MyDockerID returns id of the current container the process is running within, if any
func MyDockerID() (string, error) {
	if _, err := os.Stat("/proc/self/cgroup"); os.IsNotExist(err) {
		return "", nil
	}
	output, exitStatus, err := util.ExecPipe(&util.Cmd{
		Args: []string{"/bin/bash", "-c", `cat /proc/self/cgroup | grep "docker" | sed s/\\//\\n/g | tail -1`},
	})
	if err != nil {
		return "", err
	}
	if exitStatus != 0 {
		return "", fmt.Errorf("Failed to obtain docker id due error: %s", output)
	}

	return strings.Trim(output, "\n"), nil
}

// ResolveHostPath resolves any given path from the current context so
// it is mountable by any container.
//
// If the current process is executed in the container itself, this function
// resolves the given path according to the container's rootfs on the host
// machine. It also considers the mounted directories to the current container, so
// if given path is pointing to the mounted directory, it resolves correctly.
func ResolveHostPath(mountPath string, client *docker.Client) (string, error) {
	// Accept only absolute path
	if !filepath.IsAbs(mountPath) {
		return "", fmt.Errorf("ResolveHostPath accepts only absolute paths, given: %s", mountPath)
	}

	// In case we are running inside of a docker container
	// we have to provide our fs path right from host machine
	isMatrix, err := IsInMatrix()
	if err != nil {
		return "", err
	}
	// Not in a container, return the path as is
	if !isMatrix {
		return mountPath, nil
	}

	myDockerID, err := MyDockerID()
	if err != nil {
		return "", err
	}

	container, err := client.InspectContainer(myDockerID)
	if err != nil {
		return "", err
	}

	// Check if the given path is inside some mounted volumes
	for _, mount := range container.Mounts {
		rel, err := filepath.Rel(mount.Destination, mountPath)
		if err != nil {
			return "", err
		}
		// The easiest way to check whether the `mountPath` is within the `mount.Destination`
		if !strings.HasPrefix(rel, "..") {
			// Resolve the mounted directory to the real host path
			return strings.Replace(mountPath, mount.Destination, mount.Source, 1), nil
		}
	}

	// https://jpetazzo.github.io/assets/2015-03-03-not-so-deep-dive-into-docker-storage-drivers.html
	// aufs: /var/lib/docker/aufs/mnt/$CONTAINER_ID/
	// devicemapper: /var/lib/docker/devicemapper/mnt/$CONTAINER_ID/
	// btrfs: /var/lib/docker/btrfs/subvolumes/$CONTAINER_OR_IMAGE_ID/
	// overlayfs: /var/lib/docker/overlay/$ID_OF_CONTAINER_OR_IMAGE/merged

	// Resolve docker root by using container's resolv.conf file path
	dockerRoot := path.Join(path.Dir(container.ResolvConfPath), "../../")

	// Resolve the container mountpoint depending on the driver used
	if container.Driver == "aufs" {
		mountPath = path.Join(dockerRoot, "aufs/mnt", myDockerID, mountPath)
	} else if container.Driver == "devicemapper" {
		mountPath = path.Join(dockerRoot, "devicemapper/mnt", myDockerID, mountPath)
	} else if container.Driver == "overlay" {
		mountPath = path.Join(dockerRoot, "overlay", myDockerID, "merged", mountPath)
	} else {
		// NOTE: add support for other fs drivers is not a big deal,
		// but need to have a test environment for that.
		return "", fmt.Errorf("%s driver is not supported by rocker when using MOUNT from within a container", container.Driver)
	}

	return mountPath, nil
}
