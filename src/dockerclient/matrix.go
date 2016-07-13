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
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/grammarly/rocker/src/util"
)

const (
	initFile = "/.dockerinit"
)

// ErrDriverNotSupported is an error type that is returned if it's impossible to
// ResolveHostPath using current fs driver
type ErrDriverNotSupported struct {
	Driver string
}

// Error returns error string
func (e *ErrDriverNotSupported) Error() string {
	return fmt.Sprintf("%s driver is not supported by rocker when using MOUNT from within a container", e.Driver)
}

// ResolveHostPath resolves any given path from the current context so
// it is mountable by any container.
//
// If the current process is executed in the container itself, this function
// resolves the given path according to the container's rootfs on the host
// machine. It also considers the mounted directories to the current container, so
// if given path is pointing to the mounted directory, it resolves correctly.
func ResolveHostPath(mountPath string, client *docker.Client, isUnixSocket bool, unixSocketPath string) (string, error) {
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

	if !isUnixSocket {
		return "", fmt.Errorf("Connection to docker not via unix socket, Not make sense to resolve host path in matrix")
	}

	myDockerID, err := getMyDockerID()
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
			continue
		}
		// The easiest way to check whether the `mountPath` is within the `mount.Destination`
		if !strings.HasPrefix(rel, "..") {
			// Resolve the mounted directory to the real host path
			return strings.Replace(mountPath, mount.Destination, mount.Source, 1), nil
		}
	}

	// NOTE: Not all drivers could be used to do this hack.
	// For now we know that with docker 1.10 it could be done with overlay driver.
	// It also possibly could be done with devicemapper but not with aufs.
	// Overlayfs is good enough for now.

	// Good stuff to read about docker storage drivers:
	//   https://jpetazzo.github.io/assets/2015-03-03-not-so-deep-dive-into-docker-storage-drivers.html

	// Resolve the container mountpoint for overlay storage driver
	if container.Driver == "overlay" {
		var mountDirOnDockerHost string
		if mountDirOnDockerHost, err = getMountPathForOverlay(myDockerID, unixSocketPath); err != nil {
			fmt.Printf("Can't get mount path, %v\n", err)
			return "", err
		}
		mountPath = path.Join(mountDirOnDockerHost, mountPath)
		fmt.Printf("Path on docker host: '%v'\n", mountPath)
		return mountPath, nil
	}

	return "", &ErrDriverNotSupported{container.Driver}
}

// IsInMatrix returns true if current process is running inside of a docker container
func IsInMatrix() (bool, error) {
	_, err := os.Stat(initFile)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// getMyDockerID returns id of the current container the process is running within, if any
func getMyDockerID() (string, error) {
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

func getMountPathForOverlay(containerID string, unixSock string) (string, error) {
	req, err := http.NewRequest("GET", "/containers/"+containerID+"/json", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Thomas Anderson")

	dialer := net.Dialer{}
	conn, err := dialer.Dial("unix", unixSock)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	breader := bufio.NewReader(conn)
	err = req.Write(conn)
	if err != nil {
		return "", err
	}

	var resp *http.Response
	resp, err = http.ReadResponse(breader, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("Can't get mount point. statusCode: '%v', body:'%v'", resp.StatusCode, body)
	}

	return getMountPathForOverlayFromJSON(body)
}

func getMountPathForOverlayFromJSON(jsonData []byte) (string, error) {
	type Data struct {
		GraphDriver struct {
			Data struct {
				MergedDir string `json:"MergedDir"`
			} `json:"Data"`
		} `json:"GraphDriver"`
	}

	var data Data
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return "", err
	}
	return data.GraphDriver.Data.MergedDir, nil
}
