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
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"rocker/dockerclient"
	"rocker/imagename"
	"rocker/parser"
	"rocker/util"

	"github.com/fsouza/go-dockerclient"
)

// cmdRun implements RUN command
// If there were no MOUNTs before, rocker falls back to `docker build` to run it
func (builder *Builder) cmdRun(args []string, attributes map[string]bool, flags map[string]string, original string) (err error) {
	cmd := handleJSONArgs(args, attributes)

	if !attributes["json"] {
		cmd = append([]string{"/bin/sh", "-c"}, cmd...)
	}

	return builder.runAndCommit(cmd, "run")
}

// cmdMount implements MOUNT command
// TODO: document behavior of cmdMount
func (builder *Builder) cmdMount(args []string, attributes map[string]bool, flags map[string]string, original string) error {
	if len(args) == 0 {
		return fmt.Errorf("Command is missing value: %s", original)
	}

	// TODO: read flags
	useCache := false

	newMounts := []*builderMount{}
	newVolumeMounts := []*builderMount{}

	for _, arg := range args {
		var mount builderMount
		if strings.Contains(arg, ":") {
			pair := strings.SplitN(arg, ":", 2)
			mount = builderMount{cache: useCache, src: pair[0], dest: pair[1]}
		} else {
			mount = builderMount{cache: useCache, dest: arg}
		}

		if mount.src == "" {
			newVolumeMounts = append(newVolumeMounts, &mount)
		} else {
			// Process relative paths in volumes
			if strings.HasPrefix(mount.src, "~") {
				mount.src = strings.Replace(mount.src, "~", os.Getenv("HOME"), 1)
			}
			if !path.IsAbs(mount.src) {
				mount.src = path.Join(builder.ContextDir, mount.src)
			}
			mount.origSrc = mount.src

			var err error

			if mount.src, err = dockerclient.ResolveHostPath(mount.src, builder.Docker); err != nil {
				return err
			}
		}

		newMounts = append(newMounts, &mount)
	}

	// For volume mounts we need to create (or use existing) volume container
	if len(newVolumeMounts) > 0 {
		// Collect destinations and sort them alphabetically
		// so changing the order on MOUNT commend does not have any effect
		dests := make([]string, len(newVolumeMounts))
		containerVolumes := make(map[string]struct{})

		for i, mount := range newVolumeMounts {
			dests[i] = mount.dest
			containerVolumes[mount.dest] = struct{}{}
		}
		sort.Strings(dests)

		volumeContainerName := builder.mountsContainerName(dests)

		containerConfig := &docker.Config{
			Image:   busyboxImage,
			Volumes: containerVolumes,
			Labels: map[string]string{
				"Volumes":    strings.Join(dests, ":"),
				"Rockerfile": builder.Rockerfile,
				"ImageId":    builder.imageID,
			},
		}

		container, err := builder.ensureContainer(volumeContainerName, containerConfig, strings.Join(dests, ","))
		if err != nil {
			return err
		}

		// Assing volume container to the list of volume mounts
		for _, mount := range newVolumeMounts {
			mount.containerID = container.ID
		}
	}

	mountIds := make([]string, len(newMounts))

	for i, mount := range newMounts {
		builder.addMount(*mount)
		mountIds[i] = mount.String()
	}

	// TODO: check is useCache flag enabled, so we have to make checksum of the directory

	if err := builder.commitContainer("", builder.Config.Cmd, fmt.Sprintf("MOUNT %q", mountIds)); err != nil {
		return err
	}

	return nil
}

// cmdExport implements EXPORT command
// TODO: document behavior of cmdExport
func (builder *Builder) cmdExport(args []string, attributes map[string]bool, flags map[string]string, original string) error {
	if len(args) == 0 {
		return fmt.Errorf("Command is missing value: %s", original)
	}
	// If only one argument was given to EXPORT, use basename of a file
	// EXPORT /my/dir/file.tar --> /EXPORT_VOLUME/file.tar
	if len(args) < 2 {
		args = []string{args[0], "/"}
	}

	dest := args[len(args)-1] // last one is always the dest

	// EXPORT /my/dir my_dir --> /EXPORT_VOLUME/my_dir
	// EXPORT /my/dir /my_dir --> /EXPORT_VOLUME/my_dir
	// EXPORT /my/dir stuff/ --> /EXPORT_VOLUME/stuff/my_dir
	// EXPORT /my/dir /stuff/ --> /EXPORT_VOLUME/stuff/my_dir
	// EXPORT /my/dir/* / --> /EXPORT_VOLUME/stuff/my_dir

	exportsContainerID, err := builder.makeExportsContainer()
	if err != nil {
		return err
	}

	// prepare builder mount
	builder.addMount(builderMount{
		dest:        exportsVolume,
		containerID: exportsContainerID,
	})
	defer builder.removeLastMount()

	cmdDestPath, err := util.ResolvePath(exportsVolume, dest)
	if err != nil {
		return fmt.Errorf("Invalid EXPORT destination: %s", dest)
	}

	// TODO: rsync doesn't work as expected if ENTRYPOINT is inherited by parent image
	//       STILL RELEVANT?

	// build the command
	cmd := []string{"/opt/rsync/bin/rsync", "-a", "--delete-during"}
	cmd = append(cmd, args[0:len(args)-1]...)
	cmd = append(cmd, cmdDestPath)

	// For caching
	builder.addLabels(map[string]string{
		"rocker-exportsContainerId": exportsContainerID,
	})

	// Configure container temporarily, only for this execution
	resetFunc := builder.temporaryConfig(func() {
		builder.Config.Entrypoint = []string{}
	})
	defer resetFunc()

	fmt.Fprintf(builder.OutStream, "[Rocker]  run: %s\n", strings.Join(cmd, " "))

	if err := builder.runAndCommit(cmd, "import"); err != nil {
		return err
	}

	builder.lastExportImageID = builder.imageID

	return nil
}

// cmdImport implements IMPORT command
// TODO: document behavior of cmdImport
func (builder *Builder) cmdImport(args []string, attributes map[string]bool, flags map[string]string, original string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("Command is missing value: %s", original)
	}
	if builder.lastExportImageID == "" {
		return fmt.Errorf("You have to EXPORT something first in order to: %s", original)
	}
	if builder.exportsContainerID == "" {
		return fmt.Errorf("Something went wrong, missing exports container: %s", original)
	}
	// If only one argument was given to IMPORT, use the same path for destination
	// IMPORT /my/dir/file.tar --> ADD ./EXPORT_VOLUME/my/dir/file.tar /my/dir/file.tar
	if len(args) < 2 {
		args = []string{args[0], "/"}
	}
	dest := args[len(args)-1] // last one is always the dest

	// prepare builder mount
	builder.addMount(builderMount{
		dest:        exportsVolume,
		containerID: builder.exportsContainerID,
	})
	defer builder.removeLastMount()

	// TODO: rsync doesn't work as expected if ENTRYPOINT is inherited by parent image
	//       STILL RELEVANT?

	cmd := []string{"/opt/rsync/bin/rsync", "-a"}
	for _, arg := range args[0 : len(args)-1] {
		argResolved, err := util.ResolvePath(exportsVolume, arg)
		if err != nil {
			return fmt.Errorf("Invalid IMPORT source: %s", arg)
		}
		cmd = append(cmd, argResolved)
	}
	cmd = append(cmd, dest)

	// For caching
	builder.addLabels(map[string]string{
		"rocker-lastExportImageId": builder.lastExportImageID,
	})

	// Configure container temporarily, only for this execution
	resetFunc := builder.temporaryConfig(func() {
		builder.Config.Entrypoint = []string{}
	})
	defer resetFunc()

	fmt.Fprintf(builder.OutStream, "[Rocker]  run: %s\n", strings.Join(cmd, " "))

	return builder.runAndCommit(cmd, "import")
}

// cmdTag implements TAG command
// TODO: document behavior of cmdTag
func (builder *Builder) cmdTag(args []string, attributes map[string]bool, flags map[string]string, original string) (err error) {
	builder.recentTags = []*imagename.ImageName{}
	if len(args) == 0 {
		return fmt.Errorf("Command is missing value: %s", original)
	}
	image := imagename.NewFromString(args[0])

	// Save rockerfile to label, sot it can be inspected later
	if builder.AddMeta && !builder.metaAdded {
		data := &RockerImageData{
			ImageName:  image,
			Rockerfile: builder.RockerfileContent,
			Vars:       builder.CliVars,
			Properties: Vars{},
		}

		if hostname, _ := os.Hostname(); hostname != "" {
			data.Properties["hostname"] = hostname
		}
		if user, _ := user.Current(); user != nil {
			data.Properties["system_login"] = user.Username
			data.Properties["system_user"] = user.Name
		}

		json, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("Failed to marshal rocker data, error: %s", err)
		}

		builder.addLabels(map[string]string{
			"rocker-data": string(json),
		})

		fmt.Fprintf(builder.OutStream, "[Rocker]  add rocker-data label\n")

		if err := builder.commitContainer("", builder.Config.Cmd, "LABEL rocker-data"); err != nil {
			return err
		}

		builder.metaAdded = true
	}

	doTag := func(tag string) error {
		img := &imagename.ImageName{
			Registry: image.Registry,
			Name:     image.Name,
			Tag:      tag,
		}
		builder.recentTags = append(builder.recentTags, img)

		fmt.Fprintf(builder.OutStream, "[Rocker]  Tag %.12s -> %s\n", builder.imageID, img)

		err := builder.Docker.TagImage(builder.imageID, docker.TagImageOptions{
			Repo:  img.NameWithRegistry(),
			Tag:   img.GetTag(),
			Force: true,
		})
		if err != nil {
			return fmt.Errorf("Failed to set tag %s to image %s", img, builder.imageID)
		}
		return nil
	}

	// By default, tag with current branch name if tag is not specified
	// do not use :latest unless it was set explicitly
	if !image.HasTag() {
		if builder.Vars.IsSet("branch") && builder.Vars["branch"].(string) != "" {
			image.Tag = builder.Vars["branch"].(string)
		}
		// Additionally, tag image with current git sha
		if builder.Vars.IsSet("commit") && builder.Vars["commit"] != "" {
			if err := doTag(fmt.Sprintf("%.7s", builder.Vars["commit"])); err != nil {
				return err
			}
		}
	}

	// Do the asked tag
	if err := doTag(image.GetTag()); err != nil {
		return err
	}

	// Optionally make a semver aliases
	if _, ok := flags["semver"]; ok && image.HasTag() {
		ver, err := NewSemver(image.GetTag())
		if err != nil {
			return fmt.Errorf("--semver flag expects tag to be in semver format, error: %s", err)
		}
		if err := doTag(fmt.Sprintf("%d.%d", ver.Major, ver.Minor)); err != nil {
			return err
		}
		if err := doTag(fmt.Sprintf("%d", ver.Major)); err != nil {
			return err
		}
	}

	builder.recentTags = append(builder.recentTags, image)

	return nil
}

// cmdPush implements PUSH command
// TODO: document behavior of cmdPush
func (builder *Builder) cmdPush(args []string, attributes map[string]bool, flags map[string]string, original string) (err error) {
	if err := builder.cmdTag(args, attributes, flags, original); err != nil {
		return fmt.Errorf("Failed to tag image, error: %s", err)
	}

	if !builder.Push {
		fmt.Fprintf(builder.OutStream, "[Rocker] *** just tagged; pass --push flag to actually push to a registry\n")
		return nil
	}

	for _, image := range builder.recentTags {
		fmt.Fprintf(builder.OutStream, "[Rocker]  Push %.12s -> %s\n", builder.imageID, image)
		if err := builder.pushImage(*image); err != nil {
			return err
		}
	}

	return nil
}

// cmdRequire implements REQUIRE command
// TODO: document behavior of cmdRequire
func (builder *Builder) cmdRequire(args []string, attributes map[string]bool, flags map[string]string, original string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("Command is missing value: %s", original)
	}
	for _, requireVar := range args {
		if !builder.Vars.IsSet(requireVar) {
			return fmt.Errorf("Var $%s is required but not set", requireVar)
		}
	}
	return nil
}

// cmdVar implements VAR command
// it is deprecated due to templating functionality, see: https://github.com/grammarly/rocker#templating
func (builder *Builder) cmdVar(args []string, attributes map[string]bool, flags map[string]string, original string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("Command is missing value: %s", original)
	}
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		value := args[i+1]
		if !builder.Vars.IsSet(key) {
			builder.Vars[key] = value
		}
	}
	return nil
}

// cmdInclude implements INCLUDE command
// TODO: document behavior of cmdInclude
func (builder *Builder) cmdInclude(args []string, attributes map[string]bool, flags map[string]string, original string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("Command is missing value: %s", original)
	}

	module := args[0]
	contextDir := filepath.Dir(builder.Rockerfile)
	resultPath := filepath.Clean(path.Join(contextDir, module))

	// TODO: protect against going out of working directory?

	stat, err := os.Stat(resultPath)
	if err != nil {
		return err
	}
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("Expected included resource to be a regular file: %s (%s)", module, original)
	}

	fd, err := os.Open(resultPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	includedNode, err := parser.Parse(fd)
	if err != nil {
		return err
	}

	for _, node := range includedNode.Children {
		if node.Value == "include" {
			return fmt.Errorf("Nesting includes is not allowed: \"%s\" in %s", original, resultPath)
		}
	}

	// inject included commands info root node at current execution position
	after := append(includedNode.Children, builder.rootNode.Children[builder.i+1:]...)
	builder.rootNode.Children = append(builder.rootNode.Children[:builder.i], after...)
	builder.i--

	return nil
}

// cmdAttach implements ATTACH command
// TODO: document behavior of cmdAttach
func (builder *Builder) cmdAttach(args []string, attributes map[string]bool, flags map[string]string, original string) (err error) {
	// simply ignore this command if we don't wanna attach
	if !builder.Attach {
		fmt.Fprintf(builder.OutStream, "[Rocker] Skipping ATTACH; use --attach option to get inside\n")
		return nil
	}

	cmd := handleJSONArgs(args, attributes)

	if len(cmd) > 0 {
		if !attributes["json"] {
			cmd = append([]string{"/bin/sh", "-c"}, cmd...)
		}
	} else {
		cmd = builder.Config.Cmd
	}

	// Mount exports container if there is one
	if builder.exportsContainerID != "" {
		builder.addMount(builderMount{
			dest:        exportsVolume,
			containerID: builder.exportsContainerID,
		})
		defer builder.removeLastMount()
	}

	var name string
	if _, ok := flags["name"]; ok {
		if flags["name"] == "" {
			return fmt.Errorf("flag --name needs a value: %s", original)
		}
		name = flags["name"]
	}

	if _, ok := flags["hostname"]; ok && flags["hostname"] == "" {
		return fmt.Errorf("flag --hostname needs a value: %s", original)
	}

	// Configure container temporarily, only for this execution
	resetFunc := builder.temporaryConfig(func() {
		if _, ok := flags["hostname"]; ok {
			builder.Config.Hostname = flags["hostname"]
		}
		builder.Config.Cmd = cmd
		builder.Config.Entrypoint = []string{}
		builder.Config.Tty = true
		builder.Config.OpenStdin = true
		builder.Config.StdinOnce = true
		builder.Config.AttachStdin = true
		builder.Config.AttachStderr = true
		builder.Config.AttachStdout = true
	})
	defer resetFunc()

	containerID, err := builder.createContainer(name)
	if err != nil {
		return fmt.Errorf("Failed to create container, error: %s", err)
	}
	defer func() {
		if err2 := builder.removeContainer(containerID); err2 != nil && err == nil {
			err = err2
		}
	}()

	if err := builder.runContainerAttachStdin(containerID, true); err != nil {
		return fmt.Errorf("Failed to run attached container %s, error: %s", containerID, err)
	}

	return nil
}
