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
	"github.com/grammarly/rocker/src/imagename"
	"github.com/grammarly/rocker/src/template"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBuild_NewBuild(t *testing.T) {
	b, _ := makeBuild(t, "FROM ubuntu", Config{})
	assert.IsType(t, &Rockerfile{}, b.rockerfile)
}

func TestBuild_ReplaceEnvVars(t *testing.T) {
	rockerfile := "FROM ubuntu\nENV PATH=$PATH:/cassandra/bin"
	b, c := makeBuild(t, rockerfile, Config{})
	plan := makePlan(t, rockerfile)

	img := &docker.Image{
		ID: "123",
		Config: &docker.Config{
			Env: []string{"PATH=/usr/bin"},
		},
	}

	resultImage := &docker.Image{ID: "789"}

	c.On("InspectImage", "ubuntu:latest").Return(img, nil).Once()

	c.On("CreateContainer", mock.AnythingOfType("State")).Return("456", nil).Run(func(args mock.Arguments) {
		arg := args.Get(0).(State)
		assert.Equal(t, []string{"PATH=/usr/bin:/cassandra/bin"}, arg.Config.Env)
	}).Once()

	c.On("CommitContainer", mock.AnythingOfType("State")).Return(resultImage, nil).Once()

	c.On("RemoveContainer", "456").Return(nil).Once()

	if err := b.Run(plan); err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
}

func TestBuild_LookupImage_ExactExistLocally(t *testing.T) {
	var (
		b, c        = makeBuild(t, "", Config{})
		resultImage = &docker.Image{ID: "789"}
		name        = "ubuntu:latest"
	)

	c.On("InspectImage", name).Return(resultImage, nil).Once()

	result, err := b.lookupImage(name)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, resultImage, result)
	c.AssertExpectations(t)
}

func TestBuild_LookupImage_ExistLocally(t *testing.T) {
	var (
		nilImage *docker.Image

		b, c        = makeBuild(t, "", Config{})
		resultImage = &docker.Image{ID: "789"}
		name        = "ubuntu:latest"

		localImages = []*imagename.ImageName{
			imagename.NewFromString("debian:7.7"),
			imagename.NewFromString("debian:latest"),
			imagename.NewFromString("ubuntu:12.04"),
			imagename.NewFromString("ubuntu:14.04"),
			imagename.NewFromString("ubuntu:latest"),
		}
	)

	c.On("InspectImage", name).Return(nilImage, nil).Once()
	c.On("ListImages").Return(localImages, nil).Once()
	c.On("InspectImage", name).Return(resultImage, nil).Once()

	result, err := b.lookupImage(name)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, resultImage, result)
	c.AssertExpectations(t)
}

func TestBuild_LookupImage_NotExistLocally(t *testing.T) {
	var (
		nilImage *docker.Image

		b, c        = makeBuild(t, "", Config{})
		resultImage = &docker.Image{ID: "789"}
		name        = "ubuntu:latest"

		localImages = []*imagename.ImageName{}

		remoteImages = []*imagename.ImageName{
			imagename.NewFromString("debian:7.7"),
			imagename.NewFromString("debian:latest"),
			imagename.NewFromString("ubuntu:12.04"),
			imagename.NewFromString("ubuntu:14.04"),
			imagename.NewFromString("ubuntu:latest"),
		}
	)

	c.On("InspectImage", name).Return(nilImage, nil).Once()
	c.On("ListImages").Return(localImages, nil).Once()
	c.On("ListImageTags", name).Return(remoteImages, nil).Once()
	c.On("PullImage", name).Return(nil).Once()
	c.On("InspectImage", name).Return(resultImage, nil).Once()

	result, err := b.lookupImage(name)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, resultImage, result)
	c.AssertExpectations(t)
}

func TestBuild_LookupImage_PullAndExist(t *testing.T) {
	var (
		b, c        = makeBuild(t, "", Config{Pull: true})
		resultImage = &docker.Image{ID: "789"}
		name        = "ubuntu:latest"

		remoteImages = []*imagename.ImageName{
			imagename.NewFromString("debian:7.7"),
			imagename.NewFromString("debian:latest"),
			imagename.NewFromString("ubuntu:12.04"),
			imagename.NewFromString("ubuntu:14.04"),
			imagename.NewFromString("ubuntu:latest"),
		}
	)

	c.On("ListImageTags", name).Return(remoteImages, nil).Once()
	c.On("PullImage", name).Return(nil).Once()
	c.On("InspectImage", name).Return(resultImage, nil).Once()

	result, err := b.lookupImage(name)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, resultImage, result)
	c.AssertExpectations(t)
}

func TestBuild_LookupImage_PullAndNotExist(t *testing.T) {
	var (
		b, c = makeBuild(t, "", Config{Pull: true})
		name = "ubuntu:latest"

		remoteImages = []*imagename.ImageName{
			imagename.NewFromString("debian:7.7"),
			imagename.NewFromString("debian:latest"),
			imagename.NewFromString("ubuntu:12.04"),
			imagename.NewFromString("ubuntu:14.04"),
		}
	)

	c.On("ListImageTags", name).Return(remoteImages, nil).Once()

	_, err := b.lookupImage(name)
	assert.EqualError(t, err, "Image not found: ubuntu:latest (also checked in the remote registry)")
	c.AssertExpectations(t)
}

func TestBuild_LookupImage_ShaExistLocally(t *testing.T) {
	for _, pull := range []bool{true, false} {
		t.Logf("Testing with pull=%t", pull)

		var (
			b, c        = makeBuild(t, "", Config{Pull: pull})
			resultImage = &docker.Image{ID: "789"}
			name        = "ubuntu@sha256:afafa"
		)

		c.On("InspectImage", name).Return(resultImage, nil).Once()

		result, err := b.lookupImage(name)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, resultImage, result)
		c.AssertExpectations(t)
	}
}

func TestBuild_LookupImage_ShaNotExistLocally(t *testing.T) {
	for _, pull := range []bool{true, false} {
		t.Logf("Testing with pull=%t", pull)

		var (
			nilImage *docker.Image

			b, c        = makeBuild(t, "", Config{Pull: pull})
			resultImage = &docker.Image{ID: "789"}
			name        = "ubuntu@sha256:afafa"
		)

		c.On("InspectImage", name).Return(nilImage, nil).Once()
		c.On("PullImage", name).Return(nil).Once()
		c.On("InspectImage", name).Return(resultImage, nil).Once()

		result, err := b.lookupImage(name)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, resultImage, result)
		c.AssertExpectations(t)
	}
}

// internal helpers

func makeBuild(t *testing.T, rockerfileContent string, cfg Config) (*Build, *MockClient) {
	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)

	r, err := NewRockerfile(fn.Name(), strings.NewReader(rockerfileContent), template.Vars{}, template.Funs{})
	if err != nil {
		t.Fatal(err)
	}

	cfg.NoCache = true

	c := &MockClient{}
	b := New(c, r, nil, cfg)

	return b, c
}

// Docker client mock

type MockClient struct {
	mock.Mock
}

func (m *MockClient) InspectImage(name string) (*docker.Image, error) {
	args := m.Called(name)
	return args.Get(0).(*docker.Image), args.Error(1)
}

func (m *MockClient) PullImage(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockClient) ListImages() (images []*imagename.ImageName, err error) {
	args := m.Called()
	return args.Get(0).([]*imagename.ImageName), args.Error(1)
}

func (m *MockClient) ListImageTags(name string) (images []*imagename.ImageName, err error) {
	args := m.Called(name)
	return args.Get(0).([]*imagename.ImageName), args.Error(1)
}

func (m *MockClient) RemoveImage(imageID string) error {
	args := m.Called(imageID)
	return args.Error(0)
}

func (m *MockClient) TagImage(imageID, imageName string) error {
	args := m.Called(imageID, imageName)
	return args.Error(0)
}

func (m *MockClient) PushImage(imageName string) (string, error) {
	args := m.Called(imageName)
	return args.String(0), args.Error(1)
}

func (m *MockClient) CreateContainer(state State) (string, error) {
	args := m.Called(state)
	return args.String(0), args.Error(1)
}

func (m *MockClient) RunContainer(containerID string, attach bool) error {
	args := m.Called(containerID, attach)
	return args.Error(0)
}

func (m *MockClient) CommitContainer(state *State) (*docker.Image, error) {
	args := m.Called(*state)
	return args.Get(0).(*docker.Image), args.Error(1)
}

func (m *MockClient) RemoveContainer(containerID string) error {
	args := m.Called(containerID)
	return args.Error(0)
}

func (m *MockClient) UploadToContainer(containerID string, stream io.Reader, path string) error {
	args := m.Called(containerID, stream, path)
	return args.Error(0)
}

func (m *MockClient) ResolveHostPath(path string) (resultPath string, err error) {
	args := m.Called(path)
	return args.String(0), args.Error(1)
}

func (m *MockClient) EnsureImage(imageName string) error {
	args := m.Called(imageName)
	return args.Error(0)
}

func (m *MockClient) EnsureContainer(containerName string, config *docker.Config, hostConfig *docker.HostConfig, purpose string) (containerID string, err error) {
	args := m.Called(containerName, config, hostConfig, purpose)
	return args.String(0), args.Error(1)
}

func (m *MockClient) InspectContainer(containerName string) (container *docker.Container, err error) {
	args := m.Called(containerName)
	return args.Get(0).(*docker.Container), args.Error(1)
}

// type MockCache struct {
// 	mock.Mock
// }

// func (m *MockCache) Get(s State) (s2 *State, err error) {

// }

// func (m *MockCache) Put(s State) error {

// }
