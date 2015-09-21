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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/tarsum"
	"github.com/fsouza/go-dockerclient/vendor/github.com/docker/docker/pkg/archive"
	"github.com/fsouza/go-dockerclient/vendor/github.com/docker/docker/pkg/fileutils"
)

func copyFiles(b *Build, args []string, cmdName string) (s State, err error) {

	s = b.state

	if len(args) < 2 {
		return s, fmt.Errorf("Invalid %s format - at least two arguments required", cmdName)
	}

	// TODO: do we need to check the dest is always a directory?

	var (
		tar    io.ReadCloser
		tarSum tarsum.TarSum
		src    = args[0 : len(args)-1]
		dest   = filepath.FromSlash(args[len(args)-1]) // last one is always the dest

		// TODO: read .dockerignore
		excludes = []string{}
	)

	if tar, err = makeTarStream(b.cfg.ContextDir, src, excludes); err != nil {
		return s, err
	}

	if tarSum, err = tarsum.NewTarSum(tar, true, tarsum.Version1); err != nil {
		return s, err
	}
	if _, err = io.Copy(ioutil.Discard, tarSum); err != nil {
		return s, err
	}
	tar.Close()

	message := fmt.Sprintf("%s %s to %s", cmdName, tarSum.Sum(nil), dest)
	s.commitMsg = append(s.commitMsg, message)

	origCmd := s.config.Cmd
	s.config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + message}

	if s.containerID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	s.config.Cmd = origCmd

	// We need to make a new tar stream, because the previous one has been
	// read by the tarsum; maybe, optimize this in future
	if tar, err = makeTarStream(b.cfg.ContextDir, src, excludes); err != nil {
		return s, err
	}
	defer tar.Close()

	if err = b.client.UploadToContainer(s.containerID, tar, dest); err != nil {
		return s, err
	}

	return s, nil
}

func makeTarStream(srcPath string, includes, excludes []string) (tar io.ReadCloser, err error) {

	if includes, err = expandIncludes(srcPath, includes, excludes); err != nil {
		return nil, err
	}

	tarOpts := &archive.TarOptions{
		IncludeFiles:    includes,
		ExcludePatterns: excludes,
		Compression:     archive.Uncompressed,
		NoLchown:        true,
	}

	return archive.TarWithOptions(srcPath, tarOpts)
}

func expandIncludes(srcPath string, includes, excludes []string) (result []string, err error) {
	result = []string{}

	for _, filePath := range includes {

		matches, err := filepath.Glob(filepath.Join(srcPath, filePath))
		if err != nil {
			return result, err
		}

		for _, match := range matches {

			relFilePath, err := filepath.Rel(srcPath, match)
			if err != nil {
				return result, err
			}

			skip, err := fileutils.Matches(relFilePath, excludes)
			if err != nil {
				return result, err
			}
			if skip {
				continue
			}

			f, err := os.Stat(match)
			if err != nil {
				return result, err
			}

			// skip checking if symlinks point to non-existing file
			// also skip named pipes, because they hanging on open
			if f.Mode()&(os.ModeSymlink|os.ModeNamedPipe) != 0 {
				continue
			}

			if !f.IsDir() {
				currentFile, err := os.Open(filePath)
				if err != nil && os.IsPermission(err) {
					return result, fmt.Errorf("no permission to read from '%s'", filePath)
				}
				currentFile.Close()
			}

			result = append(result, relFilePath)
		}
	}

	return result, nil
}
