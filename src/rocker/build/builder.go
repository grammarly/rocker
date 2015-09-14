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

// Package build does build of given Rockerfile
package build

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"rocker/imagename"
	"rocker/parser"
	"rocker/template"

	"github.com/docker/docker/pkg/term"
	"github.com/fsouza/go-dockerclient"
)

const (
	// busybox used for cache data volume containers
	busyboxImage = "busybox:buildroot-2013.08.1"
	rsyncImage   = "grammarly/rsync-static:1"

	exportsVolume = "/.rocker_exports"
)

var (
	// PassEnvVars is the list of ENV variables to pass to a Rockerfile
	PassEnvVars = []string{"GIT_SSH_KEY"}
)

// Builder is the main builder object. It holds configuration options and
// intermedate state while looping through a build commands.
type Builder struct {
	Rockerfile        string
	RockerfileContent string
	ContextDir        string
	Id                string
	OutStream         io.Writer
	InStream          io.ReadCloser
	Docker            *docker.Client
	Config            *docker.Config
	Auth              *docker.AuthConfiguration
	UtilizeCache      bool
	Push              bool
	NoReuse           bool
	Verbose           bool
	Attach            bool
	Vars              Vars
	CliVars           Vars
	AddMeta           bool
	Print             bool

	rootNode           *parser.Node
	i                  int
	imageID            string
	mounts             []builderMount
	allMounts          []builderMount
	dockerfile         *parser.Node
	cacheBusted        bool
	exportDirs         []string
	intermediateImages []string
	exportsContainerID string
	lastExportImageID  string
	gitIgnored         bool
	isTerminalIn       bool
	isTerminalOut      bool
	fdIn               uintptr
	fdOut              uintptr
	metaAdded          bool
	recentTags         []*imagename.ImageName
}

type builderMount struct {
	cache       bool
	origSrc     string
	src         string
	dest        string
	containerID string
}

func (mount builderMount) String() string {
	if mount.src != "" {
		return mount.src + ":" + mount.dest
	}
	return mount.dest + ":" + mount.containerID
}

// Build runs the build of given Rockerfile and returns image id
func (builder *Builder) Build() (imageID string, err error) {
	// Do initial cleanup, you know, just to be sure
	// Previous builds could be ended up abnormally
	if err := builder.cleanup(); err != nil {
		return "", err
	}

	// Initialize auth configuration
	if builder.Auth == nil {
		builder.Auth = &docker.AuthConfiguration{}
	}

	// Initialize in/out file descriptors
	if builder.InStream != nil {
		fd, isTerminal := term.GetFdInfo(builder.InStream)
		builder.fdIn = fd
		builder.isTerminalIn = isTerminal
	}
	if builder.OutStream != nil {
		fd, isTerminal := term.GetFdInfo(builder.OutStream)
		builder.fdOut = fd
		builder.isTerminalOut = isTerminal
	}

	// Wrap this into function to have deferred functions run before
	// we do final checks
	run := func() (err error) {
		fd, err := os.Open(builder.Rockerfile)
		if err != nil {
			return fmt.Errorf("Failed to open file %s, error: %s", builder.Rockerfile, err)
		}
		defer fd.Close()

		data, err := template.ProcessConfigTemplate(builder.Rockerfile, fd, builder.Vars.ToMapOfInterface(), map[string]interface{}{})
		if err != nil {
			return err
		}
		builder.RockerfileContent = data.String()

		if builder.Print {
			fmt.Print(builder.RockerfileContent)
			os.Exit(0)
		}

		if builder.ContextDir == "" {
			builder.ContextDir = filepath.Dir(builder.Rockerfile)
		}

		if _, err := os.Stat(builder.ContextDir); err != nil {
			return err
		}

		if err := builder.checkDockerignore(); err != nil {
			return err
		}

		rootNode, err := parser.Parse(strings.NewReader(builder.RockerfileContent))
		if err != nil {
			return err
		}

		builder.rootNode = rootNode
		builder.dockerfile = &parser.Node{}

		defer func() {
			if err2 := builder.cleanup(); err2 != nil && err == nil {
				err = err2
			}
		}()

		for builder.i = 0; builder.i < len(builder.rootNode.Children); builder.i++ {
			oldImageID := builder.imageID

			if err := builder.dispatch(builder.i, builder.rootNode.Children[builder.i]); err != nil {
				return err
			}

			if builder.imageID != oldImageID && builder.imageID != "" {
				fmt.Fprintf(builder.OutStream, "[Rocker]  ---> %.12s\n", builder.imageID)
			}
		}

		if err := builder.runDockerfile(); err != nil {
			return err
		}

		return nil
	}

	if err := run(); err != nil {
		return "", err
	}

	if builder.imageID == "" {
		return "", fmt.Errorf("No image was generated. Is your Rockerfile empty?")
	}

	fmt.Fprintf(builder.OutStream, "[Rocker] Successfully built %.12s\n", builder.imageID)

	return builder.imageID, nil
}

// dispatch runs a particular command
func (builder *Builder) dispatch(stepN int, node *parser.Node) (err error) {
	cmd := node.Value
	attrs := node.Attributes
	original := node.Original
	args := []string{}
	flags := parseFlags(node.Flags)

	// fill in args and substitute vars
	for n := node.Next; n != nil; n = n.Next {
		// TODO: we also may want to collect ENV variables to use in EXPORT for example
		n.Value = builder.Vars.ReplaceString(n.Value)
		args = append(args, n.Value)
	}

	switch cmd {

	case "mount", "run", "export", "import", "tag", "push", "require", "var", "include", "attach", "from":
		// we do not have to eval RUN ourselves if we have no mounts
		if cmd == "run" && len(builder.mounts) == 0 {
			break
		}
		// also skip initial FROM command
		if cmd == "from" && builder.imageID == "" {
			break
		}
		// run dockerfile we have collected so far
		// except if we have met INCLUDE
		if cmd != "include" {
			if err := builder.runDockerfile(); err != nil {
				return err
			}
		}

		// do not want to report processing FROM command (unnecessary)
		if cmd != "from" {
			fmt.Fprintf(builder.OutStream, "[Rocker] %s %s\n", strings.ToUpper(cmd), strings.Join(args, " "))
		}

		switch cmd {
		case "mount":
			return builder.cmdMount(args, attrs, flags, original)
		case "export":
			return builder.cmdExport(args, attrs, flags, original)
		case "import":
			return builder.cmdImport(args, attrs, flags, original)
		case "run":
			return builder.cmdRun(args, attrs, flags, original)
		case "tag":
			return builder.cmdTag(args, attrs, flags, original)
		case "push":
			return builder.cmdPush(args, attrs, flags, original)
		case "require":
			return builder.cmdRequire(args, attrs, flags, original)
		case "var":
			return builder.cmdVar(args, attrs, flags, original)
		case "include":
			return builder.cmdInclude(args, attrs, flags, original)
		case "attach":
			return builder.cmdAttach(args, attrs, flags, original)
		case "from":
			// We don't need previous image
			// TODO: check it will be not deleted if tagged
			builder.intermediateImages = append(builder.intermediateImages, builder.imageID)
			builder.reset()
		}

	// use it for warnings if .git is not ignored
	case "add", "copy":
		addAll := false
		if len(args) > 0 {
			for _, arg := range args[:len(args)-1] {
				allArg := arg == "/" || arg == "." || arg == "./" || arg == "*" || arg == "./*"
				addAll = addAll || allArg
			}
		}
		hasGitInRoot := false
		if _, err := os.Stat(builder.ContextDir + "/.git"); err == nil {
			hasGitInRoot = true
		}
		if hasGitInRoot && !builder.gitIgnored && addAll {
			fmt.Fprintf(builder.OutStream,
				"[Rocker] *** WARNING .git is not ignored in .dockerignore; not ignoring .git will beat caching of: %s\n", original)
		}
	}

	// TODO: cancel build?

	// collect dockerfile
	builder.pushToDockerfile(node)

	return nil
}

// reset does reset the builder state; it is used in between different FROMs
// it doest not reset completely, some properties are shared across FROMs
func (builder *Builder) reset() {
	builder.mounts = []builderMount{}
	builder.imageID = ""
	builder.dockerfile = &parser.Node{}
	builder.Config = &docker.Config{}
	builder.cacheBusted = false
	builder.metaAdded = false
	return
}

// pushToDockerfile collects commands that will falled back to a `docker build`
func (builder *Builder) pushToDockerfile(node *parser.Node) {
	builder.dockerfile.Children = append(builder.dockerfile.Children, node)
}

// addMount adds a mount structure to the state
func (builder *Builder) addMount(mount builderMount) {
	builder.mounts = append(builder.mounts, mount)
	builder.allMounts = append(builder.allMounts, mount)
}

// removeLastMount pops mount structure from the state
func (builder *Builder) removeLastMount() {
	if len(builder.mounts) == 0 {
		return
	}
	builder.mounts = builder.mounts[0 : len(builder.mounts)-1]
}

// rockerfileName returns basename of current Rockerfile
func (builder *Builder) rockerfileName() string {
	return filepath.Base(builder.Rockerfile)
}

// rockerfileRelativePath returns the path of the current Rockerfile relative to the context dir
// TODO: whyrockerfileRelativePath() returns the basename instead? Need to test it
func (builder *Builder) rockerfileRelativePath() string {
	return filepath.Base(builder.Rockerfile)
}

// dockerfileName generates the name of Dockerfile that will be written to a context dir
// and then thrown to a `docker build` fallback
func (builder *Builder) dockerfileName() string {
	// Here we cannot puth temporary Dockerfile into tmp directory
	// That's how docker ignore technique works - it does not remove the direcotry itself, sadly
	dockerfileName := builder.getTmpPrefix() + "_" + builder.rockerfileName()
	if builder.imageID == "" {
		return dockerfileName + "_init"
	}
	return dockerfileName + "_" + fmt.Sprintf("%.12s", builder.imageID)
}

// getTmpPrefix returns the prefix for all of rocker's tmp files that will be written
// to the currect directory
func (builder *Builder) getTmpPrefix() string {
	return ".rockertmp"
}

// getIdentifier returns the sequence that is unique to the current Rockerfile
func (builder *Builder) getIdentifier() string {
	if builder.Id != "" {
		return builder.Id
	}
	return builder.ContextDir + ":" + builder.Rockerfile
}

// mountsContainerName returns the name of volume container that will be used for a particular MOUNT
func (builder *Builder) mountsContainerName(destinations []string) string {
	// TODO: should mounts be reused between different FROMs ?
	mountID := builder.getIdentifier() + ":" + strings.Join(destinations, ":")
	return fmt.Sprintf("rocker_mount_%.6x", md5.Sum([]byte(mountID)))
}

// exportsContainerName return the name of volume container that will be used for EXPORTs
func (builder *Builder) exportsContainerName() string {
	mountID := builder.getIdentifier()
	return fmt.Sprintf("rocker_exports_%.6x", md5.Sum([]byte(mountID)))
}

// cleanup cleans all tmp files produced by the build
func (builder *Builder) cleanup() error {
	// All we have to do is remove tmpDir
	// This will disable us to do parallel builds, but much easier to implement!
	os.RemoveAll(path.Join(builder.ContextDir, builder.getTmpPrefix()))
	return nil
}
