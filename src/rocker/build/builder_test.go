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

// This is a suite of integration tests for rocker/build
// I have no idea of how to isolate it and run without Docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"rocker/dockerclient"
	"rocker/test"
	"rocker/util"

	"github.com/stretchr/testify/assert"

	"github.com/fsouza/go-dockerclient"
)

func TestBuilderBuildBasic(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuild_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
ENTRYPOINT ls /
RUN touch /testing`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:     client,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	result, err := runContainer(t, client, &docker.Config{
		Image: imageID,
	}, nil)

	t.Logf("Got result: %s", result)

	assert.Contains(t, result, "testing", "expected result (ls) to contain testing file")
}

func TestBuilderBuildTag(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildTag_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
TAG testing
RUN touch /testing
PUSH quay.io/testing_project`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:     client,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	result, err := runContainer(t, client, &docker.Config{
		Image: imageID,
		Cmd:   []string{"ls", "/"},
	}, nil)

	t.Logf("Got result: %s", result)

	assert.Equal(t, "true", "true", "failed")
}

func TestBuilderBuildTagLabels(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildTagLabels_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	rockerfileContent := `FROM busybox:buildroot-2013.08.1
TAG testing
RUN touch /testing
LABEL foo=bar
PUSH quay.io/testing_project`

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": rockerfileContent,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	vars := VarsFromStrings([]string{"asd=qwe"})

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		CliVars:    vars,
		Docker:     client,
		AddMeta:    true,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	inspect, err := client.InspectImage(imageID)
	if err != nil {
		t.Fatal(err)
	}

	// test inherited labels
	assert.Equal(t, "bar", inspect.Config.Labels["foo"])

	// test rockerfile content
	data := &RockerImageData{}
	if err := json.Unmarshal([]byte(inspect.Config.Labels["rocker-data"]), data); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, rockerfileContent, data.Rockerfile)

	// test vars
	assert.Equal(t, vars, data.Vars)
}

func TestBuilderBuildMounts(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildTag_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
MOUNT /app/node_modules /app/bower_components
RUN ls /app > /out
CMD cat /out`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:     client,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	// Cleanup mount containers
	defer func() {
		for _, mountContainerID := range builder.getAllMountContainerIds() {
			if err := client.RemoveContainer(docker.RemoveContainerOptions{
				ID:            mountContainerID,
				RemoveVolumes: true,
				Force:         true,
			}); err != nil {
				t.Log(err)
			}
		}
	}()

	result, err := runContainer(t, client, &docker.Config{
		Image: imageID,
	}, nil)

	t.Logf("Got result: %s", result)

	assert.Equal(t, "bower_components\nnode_modules\n", result, "expected both volumes to be mounted")
	assert.Equal(t, 1, len(builder.getMountContainerIds()), "expected only one volume container to be created")
}

func TestBuilderMountFromHost(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderMountFromHost_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
MOUNT .:/src
RUN echo "hello" > /src/test`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:     client,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	content, err := ioutil.ReadFile(tempDir + "/test")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "hello\n", string(content))
}

func TestBuilderBuildVars(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildVars_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
RUN echo "version:$version" > /version`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Vars:       VarsFromStrings([]string{"version=125"}),
		// Push:       true,
		Docker: client,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	result, err := runContainer(t, client, &docker.Config{
		Image: imageID,
		Cmd:   []string{"cat", "/version"},
	}, nil)

	assert.Equal(t, "version:125\n", result, "failed")
}

func TestBuilderBuildMultiple(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildMultiple_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/index.js":    "console.log('hello')",
		"/data/README": "hello",
		"/Rockerfile": `
FROM busybox:buildroot-2013.08.1
ADD . /app
MOUNT /app/node_modules
RUN echo "hehe" > /app/node_modules/some_module && \
		cd /app/node_modules && \
		ln -sf some_module link_to_some_module
EXPORT /app
FROM busybox:buildroot-2013.08.1
IMPORT /app
		`,
	})
	if err != nil {
		t.Fatal(err)
	}

	imageIDs := make(map[string]struct{})
	mounts := make(map[string]struct{})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	run := func() (imageID string, err error) {

		builder := &Builder{
			Rockerfile:   tempDir + "/Rockerfile",
			UtilizeCache: true,
			OutStream:    util.PrefixPipe("[TEST] ", os.Stdout),
			Docker:       client,
		}

		defer func() {
			for _, mountContainerID := range builder.getAllMountContainerIds() {
				if mountContainerID != "" {
					mounts[mountContainerID] = struct{}{}
				}
			}
		}()

		imageID, err = builder.Build()
		if err != nil {
			return "", err
		}
		t.Logf("Got imageID: %s", imageID)

		imageIDs[imageID] = struct{}{}

		for _, imageID := range builder.intermediateImages {
			imageIDs[imageID] = struct{}{}
		}

		return imageID, nil
	}

	defer func() {
		for imageID := range imageIDs {
			if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
				t.Log(err)
			}
		}
	}()

	// Cleanup mount containers
	defer func() {
		for mountContainerID := range mounts {
			if err := client.RemoveContainer(docker.RemoveContainerOptions{
				ID:            mountContainerID,
				RemoveVolumes: true,
				Force:         true,
			}); err != nil {
				t.Log(err)
			}
		}
	}()

	imageID1, err := run()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("============================================================")

	imageID2, err := run()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, imageID1, imageID2, "expected images to be equal (valid caching behavior)")
}

func TestBuilderBuildContainerVolume(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildContainerVolume_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
MOUNT /cache
RUN echo "hello" >> /cache/output.log
RUN cp /cache/output.log /result_cache.log
CMD cat /result_cache.log`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	// Step 1

	runUtilizeCache := func(utilizeCache bool) (result string, err error) {
		builder := &Builder{
			Rockerfile:   tempDir + "/Rockerfile",
			OutStream:    util.PrefixPipe("[TEST] ", os.Stdout),
			UtilizeCache: utilizeCache,
			Docker:       client,
		}

		imageID, err := builder.Build()
		if err != nil {
			return "", err
		}
		t.Logf("Got imageID: %s", imageID)

		// Cleanup mount containers
		defer func() {
			for _, mountContainerID := range builder.getAllMountContainerIds() {
				if err := client.RemoveContainer(docker.RemoveContainerOptions{
					ID:            mountContainerID,
					RemoveVolumes: true,
					Force:         true,
				}); err != nil {
					t.Log(err)
				}
			}
		}()

		defer func() {
			if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
				t.Log(err)
			}
		}()

		// Step 2

		builder2 := &Builder{
			Rockerfile:   tempDir + "/Rockerfile",
			OutStream:    util.PrefixPipe("[TEST] ", os.Stdout),
			UtilizeCache: utilizeCache,
			Docker:       client,
		}

		imageID2, err := builder2.Build()
		if err != nil {
			return "", err
		}
		t.Logf("Got imageID2: %s", imageID2)

		defer func() {
			if err := client.RemoveImageExtended(imageID2, docker.RemoveImageOptions{Force: true}); err != nil {
				t.Log(err)
			}
		}()

		return runContainer(t, client, &docker.Config{
			Image: imageID2,
		}, nil)
	}

	result1, err := runUtilizeCache(true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "hello\n", result1, "failed")

	result2, err := runUtilizeCache(false)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "hello\nhello\n", result2, "failed")
}

func TestBuilderBuildAddCache(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildAddCache_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/data/README": "hello",
		"/Rockerfile": `
FROM busybox:buildroot-2013.08.1
ADD . /src
RUN ls -la /src
`,
	})
	if err != nil {
		t.Fatal(err)
	}

	var imageIDs []string

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	run := func() (imageID string, err error) {

		builder := &Builder{
			Rockerfile:   tempDir + "/Rockerfile",
			UtilizeCache: true,
			OutStream:    util.PrefixPipe("[TEST] ", os.Stdout),
			Docker:       client,
		}

		imageID, err = builder.Build()
		if err != nil {
			return "", err
		}
		t.Logf("Got imageID: %s", imageID)

		imageIDs = append(imageIDs, imageID)

		return imageID, nil
	}

	defer func() {
		for _, imageID := range imageIDs {
			if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
				t.Log(err)
			}
		}
	}()

	imageID1, err := run()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	imageID2, err := run()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, imageID1, imageID2, "expected images to be equal (valid caching behavior)")
}

func TestBuilderBuildRequire(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildRequire_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
REQUIRE version
RUN echo "$version" > /testing
CMD cat /testing`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	run := func(vars []string) (string, error) {
		builder := &Builder{
			Rockerfile: tempDir + "/Rockerfile",
			OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
			Docker:     client,
			Vars:       VarsFromStrings(vars),
		}

		imageID, err := builder.Build()
		if err != nil {
			return "", err
		}
		t.Logf("Got imageID: %s", imageID)

		defer func() {
			if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
				t.Log(err)
			}
		}()

		return runContainer(t, client, &docker.Config{
			Image: imageID,
		}, nil)
	}

	_, err1 := run([]string{})
	result, err2 := run([]string{"version=123"})

	assert.Equal(t, "Var $version is required but not set", err1.Error())
	assert.Nil(t, err2, "expected second run to not give error")
	assert.Equal(t, "123\n", result)
}

func TestBuilderBuildVar(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildVar_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
VAR test=true
RUN touch /testing
RUN if [ "$test" == "true" ] ; then echo "done test" > /testing; fi
CMD cat /testing`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	run := func(vars []string) (string, error) {
		builder := &Builder{
			Rockerfile: tempDir + "/Rockerfile",
			OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
			Docker:     client,
			Vars:       VarsFromStrings(vars),
		}

		imageID, err := builder.Build()
		if err != nil {
			return "", err
		}
		t.Logf("Got imageID: %s", imageID)

		defer func() {
			if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
				t.Log(err)
			}
		}()

		return runContainer(t, client, &docker.Config{
			Image: imageID,
		}, nil)
	}

	result1, err := run([]string{})
	if err != nil {
		t.Fatal(err)
	}

	result2, err := run([]string{"test=false"})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "done test\n", result1)
	assert.Equal(t, "", result2)
}

func TestBuilderBuildAttach(t *testing.T) {
	t.Skip()

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildAttach_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
CMD ["/bin/sh"]
ATTACH --name=test-attach ["ls", "-la"]`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		InStream:   os.Stdin,
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:     client,
		Attach:     true,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()
}

func TestBuilderEnsureImage(t *testing.T) {
	t.Skip()

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		OutStream: util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:    client,
		Auth:      &docker.AuthConfiguration{},
	}

	image := "busybox:buildroot-2013.08.1"

	if err := builder.ensureImage(image, "testing"); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "", "")
}

func TestBuilderEnsureContainer(t *testing.T) {
	t.Skip()

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		OutStream: util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:    client,
		Auth:      &docker.AuthConfiguration{},
	}

	containerConfig := &docker.Config{
		Image: "grammarly/rsync-static:1",
	}
	containerName := "rocker_TestBuilderEnsureContainer"

	defer func() {
		if err := client.RemoveContainer(docker.RemoveContainerOptions{ID: containerName, Force: true}); err != nil {
			t.Fatal(err)
		}
	}()

	if _, err := builder.ensureContainer(containerName, containerConfig, "testing"); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "", "")
}

func TestBuilderBuildGitWarning(t *testing.T) {
	t.Skip()

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildGitWarning_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/.git/HEAD": "hello",
		"/testing":   "hello2",
		"/Rockerfile": `FROM busybox:buildroot-2013.08.1
ADD . /`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:     client,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	result, err := runContainer(t, client, &docker.Config{
		Image: imageID,
	}, nil)

	t.Logf("Got result: %q", result)

	assert.Contains(t, result, "testing", "expected result (ls) to contain testing file")
}

func TestBuilderBuildInclude(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderBuildInclude_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/nodejs": `
RUN touch /test/bin/nodejs
RUN touch /test/bin/npm
`,
		"/java": `
RUN touch /test/bin/java
RUN touch /test/bin/gradle
`,
		"/Rockerfile": `
FROM busybox:buildroot-2013.08.1
RUN mkdir -p /test/bin
INCLUDE nodejs
INCLUDE java
CMD ["ls", "/test/bin"]
`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:     client,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	result, err := runContainer(t, client, &docker.Config{
		Image: imageID,
	}, nil)

	t.Logf("Got result: %q", result)

	assert.Equal(t, "gradle\njava\nnodejs\nnpm\n", result, "expected result (ls) to contain included files")
}

func TestBuilderImportFromScratch(t *testing.T) {

	tempDir, err := ioutil.TempDir("/tmp", "rocker_TestBuilderImportFromScratch_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = test.MakeFiles(tempDir, map[string]string{
		"/Rockerfile": `
FROM busybox:buildroot-2013.08.1
RUN mkdir -p /zzz && echo "hi" > /zzz/lalala
EXPORT zzz /

FROM scratch
IMPORT zzz /
CMD ["true"]
`,
	})

	// we will need docker client to cleanup and do some cross-checks
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	builder := &Builder{
		Rockerfile: tempDir + "/Rockerfile",
		OutStream:  util.PrefixPipe("[TEST] ", os.Stdout),
		Docker:     client,
	}

	imageID, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Got imageID: %s", imageID)

	defer func() {
		if err := client.RemoveImageExtended(imageID, docker.RemoveImageOptions{Force: true}); err != nil {
			t.Log(err)
		}
	}()

	// Create data volume container with scratch image
	c, err := client.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: imageID,
			Volumes: map[string]struct{}{
				"/zzz": struct{}{},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, RemoveVolumes: true, Force: true}); err != nil {
			t.Log(err)
		}
	}()

	result, err := runContainer(t, client, &docker.Config{
		Image: "busybox:buildroot-2013.08.1",
		Cmd:   []string{"/bin/sh", "-c", "cat /zzz/lalala"},
	}, &docker.HostConfig{
		VolumesFrom: []string{c.ID},
	})

	t.Logf("Got result: %q", result)

	assert.Equal(t, "hi\n", result)
}

func runContainer(t *testing.T, client *docker.Client, config *docker.Config, hostConfig *docker.HostConfig) (result string, err error) {
	if config == nil {
		config = &docker.Config{}
	}
	if hostConfig == nil {
		hostConfig = &docker.HostConfig{}
	}

	opts := docker.CreateContainerOptions{
		Config:     config,
		HostConfig: hostConfig,
	}

	container, err := client.CreateContainer(opts)
	if err != nil {
		return "", err
	}

	// remove container after testing
	defer func() {
		if err2 := client.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, Force: true}); err2 != nil && err == nil {
			err = err2
		}
	}()

	success := make(chan struct{})
	var buf bytes.Buffer

	attachOpts := docker.AttachToContainerOptions{
		Container:    container.ID,
		OutputStream: &buf,
		ErrorStream:  &buf,
		Stream:       true,
		Stdout:       true,
		Stderr:       true,
		Success:      success,
	}
	go client.AttachToContainer(attachOpts)

	success <- <-success

	err = client.StartContainer(container.ID, &docker.HostConfig{})
	if err != nil {
		return "", err
	}

	statusCode, err := client.WaitContainer(container.ID)
	if err != nil {
		return "", err
	}

	if statusCode != 0 {
		return "", fmt.Errorf("Failed to run container, exit with code %d", statusCode)
	}

	return buf.String(), nil
}
