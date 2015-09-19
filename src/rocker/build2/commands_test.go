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
	"testing"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

func TestCommandFrom_Existing(t *testing.T) {
	b, c := makeBuild(t, "", BuildConfig{})
	cmd := &CommandFrom{ConfigCommand{
		args: []string{"existing"},
	}}

	img := &docker.Image{
		ID: "123",
		Config: &docker.Config{
			Hostname: "localhost",
		},
	}

	c.On("InspectImage", "existing").Return(img, nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, "123", state.imageID)
	assert.Equal(t, "localhost", state.container.Hostname)
}

func TestCommandFrom_PullExisting(t *testing.T) {
	b, c := makeBuild(t, "", BuildConfig{Pull: true})
	cmd := &CommandFrom{ConfigCommand{
		args: []string{"existing"},
	}}

	img := &docker.Image{
		ID: "123",
		Config: &docker.Config{
			Hostname: "localhost",
		},
	}

	c.On("PullImage", "existing").Return(nil).Once()
	c.On("InspectImage", "existing").Return(img, nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, "123", state.imageID)
	assert.Equal(t, "localhost", state.container.Hostname)
}

func TestCommandFrom_NotExisting(t *testing.T) {
	b, c := makeBuild(t, "", BuildConfig{})
	cmd := &CommandFrom{ConfigCommand{
		args: []string{"not-existing"},
	}}

	var nilImg *docker.Image

	img := &docker.Image{
		ID:     "123",
		Config: &docker.Config{},
	}

	c.On("InspectImage", "not-existing").Return(nilImg, nil).Once()
	c.On("PullImage", "not-existing").Return(nil).Once()
	c.On("InspectImage", "not-existing").Return(img, nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, "123", state.imageID)
}

func TestCommandFrom_AfterPullNotExisting(t *testing.T) {
	b, c := makeBuild(t, "", BuildConfig{})
	cmd := &CommandFrom{ConfigCommand{
		args: []string{"not-existing"},
	}}

	var nilImg *docker.Image

	c.On("InspectImage", "not-existing").Return(nilImg, nil).Twice()
	c.On("PullImage", "not-existing").Return(nil).Once()

	_, err := cmd.Execute(b)
	c.AssertExpectations(t)
	assert.Equal(t, "FROM: Failed to inspect image after pull: not-existing", err.Error())
}
