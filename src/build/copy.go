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
	"archive/tar"
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/pkg/units"
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

func addFiles(b *Build, args []string) (s State, err error) {

	s = b.state

	if len(args) < 2 {
		return s, fmt.Errorf("Invalid ADD format - at least two arguments required")
	}

	var (
		src  = args[0 : len(args)-1]
		dest = filepath.FromSlash(args[len(args)-1]) // last one is always the dest
	)

	// If destination is not a directory (no trailing slash)
	hasTrailingSlash := strings.HasSuffix(dest, string(os.PathSeparator))
	if !hasTrailingSlash && len(src) > 1 {
		return s, fmt.Errorf("When using ADD with more than one source file, the destination must be a directory and end with a /")
	}

	if !filepath.IsAbs(dest) {
		dest = filepath.Join(s.Config.WorkingDir, dest)
		// Add the trailing slash back if we had it before
		if hasTrailingSlash {
			dest += string(os.PathSeparator)
		}
	}

	uf := b.urlFetcher

	for _, arg := range args {
		if !isURL(arg) {
			continue
		}

		if _, err = uf.Get(arg); err != nil {
			return s, err
		}
	}

	return copyFiles(b, args, "ADD")

}

func copyFiles(b *Build, args []string, cmdName string) (s State, err error) {

	s = b.state

	if len(args) < 2 {
		return s, fmt.Errorf("Invalid %s format - at least two arguments required", cmdName)
	}

	var (
		tarSum   tarsum.TarSum
		src      = args[0 : len(args)-1]
		dest     = filepath.FromSlash(args[len(args)-1]) // last one is always the dest
		u        *upload
		excludes = s.NoCache.Dockerignore
	)

	// If destination is not a directory (no trailing slash)
	hasTrailingSlash := strings.HasSuffix(dest, string(os.PathSeparator))
	if !hasTrailingSlash && len(src) > 1 {
		return s, fmt.Errorf("When using %s with more than one source file, the destination must be a directory and end with a /", cmdName)
	}

	if !filepath.IsAbs(dest) {
		dest = filepath.Join(s.Config.WorkingDir, dest)
		// Add the trailing slash back if we had it before
		if hasTrailingSlash {
			dest += string(os.PathSeparator)
		}
	}

	if u, err = makeTarStream(b.cfg.ContextDir, dest, cmdName, src, excludes, b.urlFetcher); err != nil {
		return s, err
	}

	// skip COPY if no files matched
	if len(u.files) == 0 {
		log.Infof("| No files matched")
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

	// Check cache
	s, hit, err := b.probeCache(s)
	if err != nil {
		return s, err
	}
	if hit {
		return s, nil
	}

	origCmd := s.Config.Cmd
	s.Config.Cmd = []string{"/bin/sh", "-c", "#(nop) " + message}

	if s.NoCache.ContainerID, err = b.client.CreateContainer(s); err != nil {
		return s, err
	}

	s.Config.Cmd = origCmd

	// We need to make a new tar stream, because the previous one has been
	// read by the tarsum; maybe, optimize this in future
	if u, err = makeTarStream(b.cfg.ContextDir, dest, cmdName, src, excludes, b.urlFetcher); err != nil {
		return s, err
	}

	// Copy to "/" because we made the prefix inside the tar archive
	// Do that because we are not able to reliably create directories inside the container
	if err = b.client.UploadToContainer(s.NoCache.ContainerID, u.tar, "/"); err != nil {
		return s, err
	}

	return s, nil
}

func makeTarStream(srcPath, dest, cmdName string, includes, excludes []string, urlFetcher URLFetcher) (u *upload, err error) {

	u = &upload{
		src:  srcPath,
		dest: dest,
	}

	if u.files, err = listFiles(srcPath, includes, excludes, cmdName, urlFetcher); err != nil {
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

	// If we transfer a single item
	if len(includes) == 1 {
		var (
			item            = filepath.Clean(includes[0])
			itemPath        = filepath.Join(srcPath, item)
			hasLeadingSlash = strings.HasSuffix(u.dest, sep)
			hasWildcards    = containsWildcards(item)
			itemIsDir       = false
			addSep          = false
			stripDir        = false
		)

		if stat, err := os.Stat(itemPath); err == nil && stat.IsDir() {
			itemIsDir = true
		}

		// The destination is not a directory (no leading slash) add it to the end
		if !hasLeadingSlash {
			addSep = true
		}

		// If the item copied is a directory, we have to strip its name
		// e.g. COPY asd[/1,2] /lib  -->  /lib[/1,2]  but not /lib/asd[/1,2]
		if itemIsDir {
			stripDir = true
		} else if !hasWildcards && !hasLeadingSlash {
			// If we've got a single file that was explicitly pointed in the source item
			// we need to replace its name with the destination
			// e.g. COPY src/foo.txt /app/bar.txt
			u.files[0].dest = strings.TrimLeft(u.dest, sep)
			u.dest = ""
			addSep = false
		}

		if stripDir {
			for i := range u.files {
				relDest, err := filepath.Rel(item, u.files[i].dest)
				if err != nil {
					return u, err
				}
				u.files[i].dest = relDest
			}
		}

		if addSep {
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

func listFiles(srcPath string, includes, excludes []string, cmdName string, urlFetcher URLFetcher) ([]*uploadFile, error) {

	log.Debugf("searching patterns, %# v\n", pretty.Formatter(includes))

	result := []*uploadFile{}
	seen := map[string]struct{}{}

	// TODO: support local archives (and maybe a remote archives as well)

	excludes, patDirs, exceptions, err := fileutils.CleanPatterns(excludes)
	if err != nil {
		return nil, err
	}

	// TODO: here we remove some exclude patterns, how about patDirs?
	excludes, nestedPatterns := findNestedPatterns(excludes)

	for _, pattern := range includes {

		if isURL(pattern) {
			if cmdName == "COPY" {
				return nil, fmt.Errorf("can't use url in COPY command: '%s'", pattern)
			}

			if urlFetcher == nil {
				return nil, fmt.Errorf("want to list a downloaded url '%s', but URLFetcher is not present", pattern)
			}

			ui, err := urlFetcher.GetInfo(pattern)
			if err != nil {
				return nil, err
			}

			result = append(result, &uploadFile{
				src:  ui.FileName,
				dest: ui.BaseName,
				size: ui.Size,
			})
			continue
		}

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

				if err != nil {
					return err
				}

				relFilePath, err := filepath.Rel(srcPath, path)
				if err != nil {
					return err
				}

				// TODO: ensure ignoring works correctly, maybe improve .dockerignore to work more like .gitignore?

				skip := false
				skipNested := false

				// Here we want to keep files that are specified explicitly in the includes,
				// no matter what. For example, .dockerignore can have some wildcard items
				// specified, by in COPY we want explicitly add a file, that could be ignored
				// otherwise using a wildcard or directory COPY
				if pattern != relFilePath {
					if skip, err = fileutils.OptimizedMatches(relFilePath, excludes, patDirs); err != nil {
						return err
					}
					if skipNested, err = matchNested(relFilePath, nestedPatterns); err != nil {
						return err
					}
				}

				if skip || skipNested {
					if !exceptions && info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				// TODO: read links?

				// not interested in dirs, since we walk already
				if info.IsDir() {
					return nil
				}

				if _, ok := seen[relFilePath]; ok {
					return nil
				}
				seen[relFilePath] = struct{}{}

				// cut the wildcard path of the file or use base name

				var (
					resultFilePath string
					baseChunks     = splitPath(pattern)
					destChunks     = splitPath(relFilePath)
					lastChunk      = baseChunks[len(baseChunks)-1]
				)

				if containsWildcards(lastChunk) {
					// In case there is `foo/bar/*` source path we need to make a
					// destination files without `foo/bar/` prefix
					resultFilePath = filepath.Join(destChunks[len(baseChunks)-1:]...)
				} else if matchInfo.IsDir() {
					// If source is a directory, keep as is
					resultFilePath = relFilePath
				} else {
					// The source has referred to a file
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

func splitPath(path string) []string {
	return strings.Split(path, string(os.PathSeparator))
}

type nestedPattern struct {
	prefix  string
	pattern string
}

func (p nestedPattern) Match(path string) (bool, error) {
	if !strings.HasPrefix(path, p.prefix) {
		return false, nil
	}
	return filepath.Match(p.pattern, filepath.Base(path))
}

func matchNested(path string, patterns []nestedPattern) (bool, error) {
	for _, p := range patterns {
		if m, err := p.Match(path); err != nil || m {
			return m, err
		}
	}
	return false, nil
}

func findNestedPatterns(excludes []string) (newExcludes []string, nested []nestedPattern) {
	newExcludes = []string{}
	nested = []nestedPattern{}
	for _, e := range excludes {
		i := strings.Index(e, "**/")
		// keep exclude
		if i < 0 {
			newExcludes = append(newExcludes, e)
			continue
		}
		// make a nested pattern
		nested = append(nested, nestedPattern{e[:i], e[i+3:]})
	}
	return newExcludes, nested
}
