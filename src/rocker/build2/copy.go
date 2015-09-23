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
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/pkg/units"
	"github.com/fsouza/go-dockerclient/vendor/github.com/docker/docker/pkg/fileutils"
	"github.com/kr/pretty"

	log "github.com/Sirupsen/logrus"
)

const buffer32K = 32 * 1024

type upload struct {
	tar   io.ReadCloser
	size  int64
	src   string
	files []*uploadFile
	dest  string
}

type uploadFile struct {
	src  string
	dest string
	size int64
}

func copyFiles(b *Build, args []string, cmdName string) (s State, err error) {

	s = b.state

	if len(args) < 2 {
		return s, fmt.Errorf("Invalid %s format - at least two arguments required", cmdName)
	}

	var (
		tarSum tarsum.TarSum
		src    = args[0 : len(args)-1]
		dest   = filepath.FromSlash(args[len(args)-1]) // last one is always the dest
		u      *upload

		// TODO: read .dockerignore
		excludes = []string{}
	)

	// If destination is not a directory (no leading slash)
	if !strings.HasSuffix(dest, string(os.PathSeparator)) && len(src) > 1 {
		return s, fmt.Errorf("When using %s with more than one source file, the destination must be a directory and end with a /", cmdName)
	}

	if u, err = makeTarStream(b.cfg.ContextDir, dest, cmdName, src, excludes); err != nil {
		return s, err
	}

	// skip COPY if no files matched
	if len(u.files) == 0 {
		log.Infof("| No files matched")
		s.SkipCommit()
		return s, nil
	}

	log.Infof("| Calculating tarsum for %d files (%s total)", len(u.files), units.HumanSize(float64(u.size)))

	if tarSum, err = tarsum.NewTarSum(u.tar, true, tarsum.Version1); err != nil {
		return s, err
	}
	if _, err = io.Copy(ioutil.Discard, tarSum); err != nil {
		return s, err
	}
	u.tar.Close()

	// TODO: useful commit comment?

	message := fmt.Sprintf("%s %s to %s", cmdName, tarSum.Sum(nil), dest)
	s.Commit(message)

	origCmd := s.Config.Cmd
	s.Config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + message}

	if s.ContainerID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	s.Config.Cmd = origCmd

	// We need to make a new tar stream, because the previous one has been
	// read by the tarsum; maybe, optimize this in future
	if u, err = makeTarStream(b.cfg.ContextDir, dest, cmdName, src, excludes); err != nil {
		return s, err
	}

	// Copy to "/" because we made the prefix inside the tar archive
	// Do that because we are not able to reliably create directories inside the container
	if err = b.client.UploadToContainer(s.ContainerID, u.tar, "/"); err != nil {
		return s, err
	}

	return s, nil
}

func makeTarStream(srcPath, dest, cmdName string, includes, excludes []string) (u *upload, err error) {

	u = &upload{
		src:  srcPath,
		dest: dest,
	}

	if u.files, err = listFiles(srcPath, includes, excludes); err != nil {
		return u, err
	}

	// Calculate total size
	for _, f := range u.files {
		u.size += f.size
	}

	sep := string(os.PathSeparator)

	if len(u.files) == 0 {
		return u, nil
	}

	// If destination is not a directory (no leading slash)
	if !strings.HasSuffix(u.dest, sep) {
		// If we transfer a single file and the destination is not a directory,
		// then rename it and remove prefix
		if len(u.files) == 1 {
			u.files[0].dest = strings.TrimLeft(u.dest, sep)
			u.dest = ""
		} else {
			// add leading slash for more then one file
			u.dest += sep
		}
	}

	// Cut the slash prefix from the dest, because it will be the root of the tar
	// the archive will be always uploaded to the root of a container
	if strings.HasPrefix(u.dest, sep) {
		u.dest = u.dest[1:]
	}

	log.Debugf("Making archive prefix=%s %# v", u.dest, pretty.Formatter(u))

	pipeReader, pipeWriter := io.Pipe()
	u.tar = pipeReader

	go func() {
		ta := &tarAppender{
			TarWriter: tar.NewWriter(pipeWriter),
			Buffer:    bufio.NewWriterSize(nil, buffer32K),
			SeenFiles: make(map[uint64]string),
		}

		defer func() {
			if err := ta.TarWriter.Close(); err != nil {
				log.Errorf("Failed to close tar writer, error: %s", err)
			}
			if err := pipeWriter.Close(); err != nil {
				log.Errorf("Failed to close pipe writer, error: %s", err)
			}
		}()

		// write files to tar
		for _, f := range u.files {
			ta.addTarFile(f.src, u.dest+f.dest)
		}
	}()

	return u, nil
}

func listFiles(srcPath string, includes, excludes []string) ([]*uploadFile, error) {

	result := []*uploadFile{}
	seen := map[string]struct{}{}

	// TODO: support urls
	// TODO: support local archives (and maybe a remote archives as well)

	for _, pattern := range includes {

		matches, err := filepath.Glob(filepath.Join(srcPath, pattern))
		if err != nil {
			return result, err
		}

		for _, match := range matches {

			// We need to check if the current match is dir
			// to prefix files inside with it
			matchInfo, err := os.Stat(match)
			if err != nil {
				return result, err
			}

			// Walk through each match since it may be a directory
			err = filepath.Walk(match, func(path string, info os.FileInfo, err error) error {

				relFilePath, err := filepath.Rel(srcPath, path)
				if err != nil {
					return err
				}

				// TODO: ensure explicit include does not get excluded by the following rule
				// TODO: ensure ignoring works correctly, maybe improve .dockerignore to work more like .gitignore?

				skip, err := fileutils.Matches(relFilePath, excludes)
				if err != nil {
					return err
				}
				if skip {
					return nil
				}

				// TODO: read links?

				// skip checking if symlinks point to non-existing file
				// also skip named pipes, because they hanging on open
				if info.Mode()&(os.ModeSymlink|os.ModeNamedPipe) != 0 {
					return nil
				}

				// not interested in dirs, since we walk already
				if info.IsDir() {
					return nil
				}

				if _, ok := seen[relFilePath]; ok {
					return nil
				}
				seen[relFilePath] = struct{}{}

				// cut the wildcard path of the file or use base name
				var resultFilePath string
				if containsWildcards(pattern) {
					common := commonPrefix(pattern, relFilePath)
					resultFilePath = strings.Replace(relFilePath, common, "", 1)
				} else if matchInfo.IsDir() {
					common := commonPrefix(pattern, match)
					resultFilePath = strings.Replace(relFilePath, common, "", 1)
				} else {
					resultFilePath = filepath.Base(relFilePath)
				}

				result = append(result, &uploadFile{
					src:  path,
					dest: resultFilePath,
					size: info.Size(),
				})

				return nil
			})

			if err != nil {
				return result, err
			}
		}
	}

	return result, nil
}

func containsWildcards(name string) bool {
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

func commonPrefix(a, b string) (prefix string) {
	// max length of either a or b
	l := len(a)
	if len(b) > l {
		l = len(b)
	}
	// find common prefix
	for i := 0; i < l; i++ {
		if a[i] != b[i] {
			break
		}
		// not optimal, but I don't care
		prefix = prefix + string(a[i])
	}
	return
}
