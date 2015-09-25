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
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TODO: maybe move some stuff from copy.go here

var (
	DockerignoreCommendRegexp = regexp.MustCompile("\\s*#.*")
)

func ReadDockerignoreFile(file string) ([]string, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	return ReadDockerignore(fd)
}

func ReadDockerignore(r io.Reader) ([]string, error) {
	var (
		scanner = bufio.NewScanner(r)
		result  = []string{}
	)

	for scanner.Scan() {
		// Strip comments
		line := scanner.Text()
		line = DockerignoreCommendRegexp.ReplaceAllString(line, "")
		// Eliminate leading and trailing whitespace.
		pattern := strings.TrimSpace(line)
		if pattern == "" {
			continue
		}
		pattern = filepath.Clean(pattern)
		result = append(result, pattern)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
