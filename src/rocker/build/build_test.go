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
	"io"
	"rocker/template"
	"runtime"
	"strings"
	"testing"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewBuild(t *testing.T) {
	b, _ := makeBuild(t, "FROM ubuntu", Config{})
	assert.IsType(t, &Rockerfile{}, b.rockerfile)
}

// internal helpers

func makeBuild(t *testing.T, rockerfileContent string, cfg Config) (*Build, *MockClient) {
	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)

	r, err := NewRockerfile(fn.Name(), strings.NewReader(rockerfileContent), template.Vars{}, template.Funs{})
	if err != nil {
		t.Fatal(err)
	}

	c := &MockClient{}
	b := New(c, r, nil, cfg)

	return b, c
}

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

func (m *MockClient) RemoveImage(imageID string) error {
	args := m.Called(imageID)
	return args.Error(0)
}

func (m *MockClient) TagImage(imageID, imageName string) error {
	args := m.Called(imageID, imageName)
	return args.Error(0)
}

func (m *MockClient) PushImage(imageName string) error {
	args := m.Called(imageName)
	return args.Error(0)
}

func (m *MockClient) CreateContainer(state State) (string, error) {
	args := m.Called(state)
	return args.String(0), args.Error(1)
}

func (m *MockClient) RunContainer(containerID string, attach bool) error {
	args := m.Called(containerID, attach)
	return args.Error(0)
}

func (m *MockClient) CommitContainer(state State, message string) (*docker.Image, error) {
	args := m.Called(state, message)
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

func (m *MockClient) EnsureContainer(containerName string, config *docker.Config, purpose string) (containerID string, err error) {
	args := m.Called(containerName, config, purpose)
	return args.String(0), args.Error(1)
}

// type MockCache struct {
// 	mock.Mock
// }

// func (m *MockCache) Get(s State) (s2 *State, err error) {

// }

// func (m *MockCache) Put(s State) error {

// }
