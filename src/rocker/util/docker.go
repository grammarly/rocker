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

package util

import (
	"fmt"
	"os"
	"strings"
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
	output, exitStatus, err := ExecPipe(&Cmd{
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
