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
	"io"
	"strings"

	"github.com/grammarly/rocker/src/imagename"

	"github.com/docker/docker/pkg/units"
	"github.com/fatih/color"

	"github.com/fsouza/go-dockerclient"
	"github.com/kr/pretty"

	log "github.com/Sirupsen/logrus"
)

var (
	// NoBaseImageSpecifier defines the empty image name, used in the FROM instruction
	NoBaseImageSpecifier = "scratch"

	// MountVolumeImage used for MOUNT volume containers
	MountVolumeImage = "grammarly/scratch:latest"

	// RsyncImage used for EXPORT volume containers
	RsyncImage = "grammarly/rsync-static:1"

	// ExportsPath is the path within EXPORT volume containers
	ExportsPath = "/.rocker_exports"

	// DefaultPathEnv is a system PATH variable to be used in Env substitutions
	// if path is not set by ENV command
	DefaultPathEnv = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

// Config used specify parameters for the builder in New()
type Config struct {
	OutStream     io.Writer
	InStream      io.ReadCloser
	ContextDir    string
	ID            string
	Dockerignore  []string
	ArtifactsPath string
	Pull          bool
	NoGarbage     bool
	Attach        bool
	Verbose       bool
	NoCache       bool
	ReloadCache   bool
	Push          bool
	CacheDir      string
	LogJSON       bool
	BuildArgs     map[string]string
}

// Build is the main object that processes build
type Build struct {
	ProducedSize int64
	VirtualSize  int64

	rockerfile *Rockerfile
	cache      Cache
	cfg        Config
	client     Client
	state      State

	// A little hack to support cross-FROM cache for EXPORTS
	// maybe rethink it later
	exports []string

	currentExportContainerName string
	prevExportContainerID      string

	urlFetcher URLFetcher

	allowedBuildArgs map[string]bool
}

// New creates the new build object
func New(client Client, rockerfile *Rockerfile, cache Cache, cfg Config) *Build {
	b := &Build{
		rockerfile: rockerfile,
		cache:      cache,
		cfg:        cfg,
		client:     client,
		exports:    []string{},

		// Build args allowed by Docker by default:
		// https://docs.docker.com/engine/reference/builder/#/arg
		allowedBuildArgs: map[string]bool{
			"HTTP_PROXY":  true,
			"http_proxy":  true,
			"HTTPS_PROXY": true,
			"https_proxy": true,
			"FTP_PROXY":   true,
			"ftp_proxy":   true,
			"NO_PROXY":    true,
			"no_proxy":    true,
		},
	}

	b.urlFetcher = NewURLFetcherFS(cfg.CacheDir, cfg.NoCache, nil)

	b.state = NewState(b)

	if cfg.BuildArgs != nil {
		b.state.NoCache.BuildArgs = cfg.BuildArgs
	}

	return b
}

// Run runs the build following the given Plan
func (b *Build) Run(plan Plan) (err error) {

	for k := 0; k < len(plan); k++ {
		command := plan[k]

		log.Debugf("Step %d: %# v", k+1, pretty.Formatter(command))

		var doRun bool
		if doRun, err = command.ShouldRun(b); err != nil {
			return err
		}
		if !doRun {
			continue
		}

		// Replace env for the command if appropriate
		if command, ok := command.(EnvReplacableCommand); ok {
			env := b.state.Config.Env
			for key, val := range b.state.NoCache.BuildArgs {
				if !b.allowedBuildArgs[key] {
					// skip build-args that are not in allowed list, meaning they have
					// not been defined by an "ARG" Dockerfile command yet.
					// This is an error condition but only if there is no "ARG" in the entire
					// Dockerfile, so we'll generate any necessary errors after we parsed
					// the entire file (see 'leftoverArgs' processing in evaluator.go )
					continue
				}
				env = append(env, fmt.Sprintf("%s=%s", key, val))
			}
			command.ReplaceEnv(env)
		}

		log.Infof("%s", color.New(color.FgWhite, color.Bold).SprintFunc()(command))

		if b.state, err = command.Execute(b); err != nil {
			return err
		}

		log.Debugf("State after step %d: %# v", k+1, pretty.Formatter(b.state))

		// Here we need to inject ONBUILD commands on the fly,
		// build sub plan and merge it with the main plan.
		// Not very beautiful, because Run uses Plan as the argument
		// and then it builds its own. But.
		if len(b.state.InjectCommands) > 0 {
			commands, err := parseOnbuildCommands(b.state.InjectCommands)
			if err != nil {
				return err
			}
			subPlan, err := NewPlan(commands, false)
			if err != nil {
				return err
			}
			tail := append(subPlan, plan[k+1:]...)
			plan = append(plan[:k+1], tail...)

			b.state.InjectCommands = []string{}
		}
	}

	// check if there are any leftover build-args that were passed but not
	// consumed during build. Return an error, if there are any.
	leftoverArgs := []string{}
	for arg := range b.cfg.BuildArgs {
		if !b.allowedBuildArgs[arg] {
			leftoverArgs = append(leftoverArgs, arg)
		}
	}
	if len(leftoverArgs) > 0 {
		return fmt.Errorf("One or more build-args %v were not consumed, failing build.", leftoverArgs)
	}

	return nil
}

// GetState returns current build state object
func (b *Build) GetState() State {
	return b.state
}

// GetImageID returns last image ID produced by the build
func (b *Build) GetImageID() string {
	return b.state.ImageID
}

func (b *Build) probeCache(s State) (cachedState State, hit bool, err error) {
	cachedState, hit, err = b.probeCacheAndPreserveCommits(s)
	if hit && err == nil {
		cachedState.CleanCommits()
	}
	return
}

func (b *Build) probeCacheAndPreserveCommits(s State) (cachedState State, hit bool, err error) {

	if b.cache == nil || s.NoCache.CacheBusted {
		return s, false, nil
	}

	var s2 *State
	if s2, err = b.cache.Get(s); err != nil {
		return s, false, err
	}
	if s2 == nil {
		s.NoCache.CacheBusted = true
		log.Info(color.New(color.FgYellow).SprintFunc()("| Not cached"))
		return s, false, nil
	}

	if b.cfg.ReloadCache {
		defer b.cache.Del(*s2)
		s.NoCache.CacheBusted = true
		log.Info(color.New(color.FgYellow).SprintFunc()("| Reload cache"))
		return s, false, nil
	}

	var img *docker.Image
	if img, err = b.client.InspectImage(s2.ImageID); err != nil {
		return s, true, err
	}
	if img == nil {
		defer b.cache.Del(*s2)
		s.NoCache.CacheBusted = true
		log.Info(color.New(color.FgYellow).SprintFunc()("| Not cached"))
		return s, false, nil
	}

	// There can be a cached state with no image Size preset
	// (made with earlier rocker version)
	// so we check that here and initialize state's Size and ParentSize
	if s2.Size != img.VirtualSize {
		s2.Size = img.VirtualSize
		s2.ParentSize = s.Size
	}

	fields := log.Fields{}
	if !b.cfg.LogJSON {
		size := fmt.Sprintf("%s (+%s)",
			units.HumanSize(float64(s2.Size)),
			units.HumanSize(float64(s2.Size-s2.ParentSize)),
		)
		fields["size"] = size
	} else {
		fields["size"] = s2.Size
		fields["delta"] = s2.Size - s2.ParentSize
	}

	log.WithFields(fields).Infof(
		color.New(color.FgGreen).SprintfFunc()("| Cached! Take image %.12s", s2.ImageID))

	// Store some stuff to the build
	b.ProducedSize += s2.Size - s2.ParentSize
	b.VirtualSize = s2.Size

	// Keep items that should not be cached from the previous state
	s2.NoCache = s.NoCache

	return *s2, true, nil
}

func (b *Build) getVolumeContainer(path string) (c *docker.Container, err error) {

	name := b.mountsContainerName(path)

	config := &docker.Config{
		Image: MountVolumeImage,
		Volumes: map[string]struct{}{
			path: struct{}{},
		},
	}

	log.Debugf("Make MOUNT volume container %s with options %# v", name, config)

	if _, err = b.client.EnsureContainer(name, config, nil, path); err != nil {
		return nil, err
	}

	log.Infof("| Using container %s for %s", name, path)

	return b.client.InspectContainer(name)
}

func (b *Build) getExportsContainerWithBinds(name string, binds []string) (c *docker.Container, err error) {

	config := &docker.Config{
		Image: RsyncImage,
		Volumes: map[string]struct{}{
			"/opt/rsync/bin": struct{}{},
			ExportsPath:      struct{}{},
		},
		Cmd:        []string{"/opt/rsync/bin/rsync", "-a", "--delete-during", "/.rocker_exports_source/", "/.rocker_exports/"},
		Entrypoint: []string{},
	}

	var hostConfig *docker.HostConfig

	if len(binds) != 0 {
		hostConfig = &docker.HostConfig{
			Binds: binds,
		}
	}

	log.Debugf("Make EXPORT container %s with options %# v", name, config)

	containerID, err := b.client.EnsureContainer(name, config, hostConfig, "exports")
	if err != nil {
		return nil, err
	}

	log.Infof("| Using exports container %s", name)

	return b.client.InspectContainer(containerID)
}

func (b *Build) getExportsContainerAndSync(currentName, previousName string) (c *docker.Container, err error) {
	//If it the first `EXPORT` in the file
	//we don't need to sync data from previous container
	if previousName == "" {
		return b.getExportsContainer(currentName)
	}

	prevContainer, err := b.getExportsContainer(previousName)
	if err != nil {
		return nil, err
	}

	binds := mountsToBinds(prevContainer.Mounts, "_source")

	currContainer, err := b.getExportsContainerWithBinds(currentName, binds)
	if err != nil {
		return nil, err
	}

	log.Infof("| Running in %s: %s", currentName, strings.Join(currContainer.Config.Cmd, " "))
	if err = b.client.RunContainer(currContainer.ID, false); err != nil {
		return nil, err
	}
	return currContainer, nil
}

func (b *Build) getExportsContainer(name string) (c *docker.Container, err error) {
	return b.getExportsContainerWithBinds(name, []string{})
}

// lookupImage looks up for the image by name and returns *docker.Image object (result of the inspect)
// `Pull` config option defines whether we want to update the latest version of the image from the remote registry
// See build.Config struct for more details about other build config options.
//
// If `Pull` is false, it tries to lookup locally by exact matching, e.g. if the image is already
// pulled with that exact name given (no fuzzy semver matching)
//
// Then the function fetches the list of all pulled images and tries to match one of them by the given name.
//
// If `Pull` is set to true or if it cannot find the image locally, it then fetches all image
// tags from the remote registry and finds the best match for the given image name.
//
// If it cannot find the image either locally or in the remote registry, it returns `nil`
//
// In case the given image has sha256 tag, it looks for it locally and pulls if it's not found.
// No semver matching is done for sha256 tagged images.
//
// See also TestBuild_LookupImage_* test cases in build_test.go
func (b *Build) lookupImage(name string) (img *docker.Image, err error) {
	var (
		candidate, remoteCandidate *imagename.ImageName

		imgName = imagename.NewFromString(name)
		pull    = false
		hub     = b.cfg.Pull
		isSha   = imgName.TagIsSha()
	)

	// If hub is true, then there is no sense to inspect the local image
	if !hub || isSha {
		if isOld, warning := imagename.WarnIfOldS3ImageName(name); isOld {
			log.Warn(warning)
		}
		// Try to inspect image as is, without version resolution
		if img, err := b.client.InspectImage(imgName.String()); err != nil || img != nil {
			return img, err
		}
	}

	if isSha {
		// If we are still here and image not found locally, we want to pull it
		candidate = imgName
		hub = false
		pull = true
	}

	if !isSha && !hub {
		// List local images
		var localImages = []*imagename.ImageName{}
		if localImages, err = b.client.ListImages(); err != nil {
			return nil, err
		}
		// Resolve local candidate
		candidate = imgName.ResolveVersion(localImages, true)
	}

	// In case we want to include external images as well, pulling list of available
	// images from the remote registry
	if hub || candidate == nil {
		log.Debugf("Getting list of tags for %s from the registry", imgName)

		var remoteImages []*imagename.ImageName

		if remoteImages, err = b.client.ListImageTags(imgName.String()); err != nil {
			err = fmt.Errorf("Failed to list tags of image %s from the remote registry, error: %s", imgName, err)
			return
		}

		// Since we found the remote image, we want to pull it
		if remoteCandidate = imgName.ResolveVersion(remoteImages, false); remoteCandidate != nil {
			pull = true
			candidate = remoteCandidate

			// If we've found needed image on s3 it should be pulled in the same name style as lookuped image
			candidate.IsOldS3Name = imgName.IsOldS3Name
		}
	}

	// If not candidate found, it's an error
	if candidate == nil {
		err = fmt.Errorf("Image not found: %s (also checked in the remote registry)", imgName)
		return
	}

	if !isSha && imgName.GetTag() != candidate.GetTag() {
		if remoteCandidate != nil {
			log.Infof("Resolve %s --> %s (found remotely)", imgName, candidate.GetTag())
		} else {
			log.Infof("Resolve %s --> %s", imgName, candidate.GetTag())
		}
	}

	if pull {
		if err = b.client.PullImage(candidate.String()); err != nil {
			return
		}
	}

	return b.client.InspectImage(candidate.String())
}
