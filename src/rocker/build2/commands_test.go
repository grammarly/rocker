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
	"reflect"
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/mock"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

// =========== Testing FROM ===========

func TestCommandFrom_Existing(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
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
	assert.Equal(t, "123", state.ImageID)
	assert.Equal(t, "localhost", state.Config.Hostname)
}

func TestCommandFrom_PullExisting(t *testing.T) {
	b, c := makeBuild(t, "", Config{Pull: true})
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
	assert.Equal(t, "123", state.ImageID)
	assert.Equal(t, "localhost", state.Config.Hostname)
}

func TestCommandFrom_NotExisting(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
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
	assert.Equal(t, "123", state.ImageID)
}

func TestCommandFrom_AfterPullNotExisting(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
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

// =========== Testing RUN ===========

func TestCommandRun_Simple(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandRun{ConfigCommand{
		args: []string{"whoami"},
	}}

	origCmd := []string{"/bin/program"}
	b.state.Config.Cmd = origCmd
	b.state.ImageID = "123"

	c.On("CreateContainer", mock.AnythingOfType("State")).Return("456", nil).Run(func(args mock.Arguments) {
		arg := args.Get(0).(State)
		assert.Equal(t, []string{"/bin/sh", "-c", "whoami"}, arg.Config.Cmd)
	}).Once()

	c.On("RunContainer", "456", false).Return(nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, origCmd, b.state.Config.Cmd)
	assert.Equal(t, origCmd, state.Config.Cmd)
	assert.Equal(t, "123", state.ImageID)
	assert.Equal(t, "456", state.ContainerID)
}

// =========== Testing COMMIT ===========

func TestCommandCommit_Simple(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandCommit{}

	origCommitMsg := []string{"a", "b"}
	b.state.ContainerID = "456"
	b.state.CommitMsg = origCommitMsg

	c.On("CommitContainer", mock.AnythingOfType("State"), "a; b").Return("789", nil).Once()
	c.On("RemoveContainer", "456").Return(nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, origCommitMsg, b.state.CommitMsg)
	assert.Equal(t, []string{}, state.CommitMsg)
	assert.Equal(t, []string(nil), state.Config.Cmd)
	assert.Equal(t, "789", state.ImageID)
	assert.Equal(t, "", state.ContainerID)
}

func TestCommandCommit_NoContainer(t *testing.T) {
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandCommit{}

	origCommitMsg := []string{"a", "b"}
	b.state.CommitMsg = origCommitMsg

	c.On("CreateContainer", mock.AnythingOfType("State")).Return("456", nil).Run(func(args mock.Arguments) {
		arg := args.Get(0).(State)
		assert.Equal(t, []string{"/bin/sh", "-c", "#(nop) a; b"}, arg.Config.Cmd)
	}).Once()

	c.On("CommitContainer", mock.AnythingOfType("State"), "a; b").Return("789", nil).Once()
	c.On("RemoveContainer", "456").Return(nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	c.AssertExpectations(t)
	assert.Equal(t, origCommitMsg, b.state.CommitMsg)
	assert.Equal(t, []string{}, state.CommitMsg)
	assert.Equal(t, "789", state.ImageID)
	assert.Equal(t, "", state.ContainerID)
}

func TestCommandCommit_NoCommitMsgs(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandCommit{}

	_, err := cmd.Execute(b)
	assert.Nil(t, err)
}

// TODO: test skip commit

// =========== Testing ENV ===========

func TestCommandEnv_Simple(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandEnv{ConfigCommand{
		args: []string{"type", "web", "env", "prod"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"ENV type=web env=prod"}, state.CommitMsg)
	assert.Equal(t, []string{"type=web", "env=prod"}, state.Config.Env)
}

func TestCommandEnv_Advanced(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandEnv{ConfigCommand{
		args: []string{"type", "web", "env", "prod"},
	}}

	b.state.Config.Env = []string{"env=dev", "version=1.2.3"}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"ENV type=web env=prod"}, state.CommitMsg)
	assert.Equal(t, []string{"env=prod", "version=1.2.3", "type=web"}, state.Config.Env)
}

// =========== Testing LABEL ===========

func TestCommandLabel_Simple(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandLabel{ConfigCommand{
		args: []string{"type", "web", "env", "prod"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	expectedLabels := map[string]string{
		"type": "web",
		"env":  "prod",
	}

	t.Logf("Result labels: %# v", pretty.Formatter(state.Config.Labels))

	assert.Equal(t, []string{"LABEL type=web env=prod"}, state.CommitMsg)
	assert.True(t, reflect.DeepEqual(state.Config.Labels, expectedLabels), "bad result labels")
}

func TestCommandLabel_Advanced(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandLabel{ConfigCommand{
		args: []string{"type", "web", "env", "prod"},
	}}

	b.state.Config.Labels = map[string]string{
		"env":     "dev",
		"version": "1.2.3",
	}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	expectedLabels := map[string]string{
		"type":    "web",
		"version": "1.2.3",
		"env":     "prod",
	}

	t.Logf("Result labels: %# v", pretty.Formatter(state.Config.Labels))

	assert.Equal(t, []string{"LABEL type=web env=prod"}, state.CommitMsg)
	assert.True(t, reflect.DeepEqual(state.Config.Labels, expectedLabels), "bad result labels")
}

// =========== Testing MAINTAINER ===========

func TestCommandMaintainer_Simple(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandMaintainer{ConfigCommand{
		args: []string{"terminator"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, state.CommitMsg, 1)
	assert.Equal(t, COMMIT_SKIP, state.CommitMsg[0])
}

// =========== Testing WORKDIR ===========

func TestCommandWorkdir_Simple(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandWorkdir{ConfigCommand{
		args: []string{"/app"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "/app", state.Config.WorkingDir)
}

func TestCommandWorkdir_Relative_HasRoot(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandWorkdir{ConfigCommand{
		args: []string{"www"},
	}}

	b.state.Config.WorkingDir = "/home"

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "/home/www", state.Config.WorkingDir)
}

func TestCommandWorkdir_Relative_NoRoot(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandWorkdir{ConfigCommand{
		args: []string{"www"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "/www", state.Config.WorkingDir)
}

// =========== Testing CMD ===========

func TestCommandCmd_Simple(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandCmd{ConfigCommand{
		args: []string{"apt-get", "install"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"/bin/sh", "-c", "apt-get install"}, state.Config.Cmd)
}

func TestCommandCmd_Json(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandCmd{ConfigCommand{
		args:  []string{"apt-get", "install"},
		attrs: map[string]bool{"json": true},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"apt-get", "install"}, state.Config.Cmd)
}

// =========== Testing ENTRYPOINT ===========

func TestCommandEntrypoint_Simple(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandEntrypoint{ConfigCommand{
		args: []string{"/bin/sh"},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"/bin/sh", "-c", "/bin/sh"}, state.Config.Entrypoint)
}

func TestCommandEntrypoint_Json(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandEntrypoint{ConfigCommand{
		args:  []string{"/bin/bash", "-c"},
		attrs: map[string]bool{"json": true},
	}}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"/bin/bash", "-c"}, state.Config.Entrypoint)
}

func TestCommandEntrypoint_Remove(t *testing.T) {
	b, _ := makeBuild(t, "", Config{})
	cmd := &CommandEntrypoint{ConfigCommand{
		args: []string{},
	}}

	b.state.Config.Entrypoint = []string{"/bin/sh", "-c"}

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string(nil), state.Config.Entrypoint)
}

// =========== Testing COPY ===========

func TestCommandCopy_Simple(t *testing.T) {
	// TODO: do we need to check the dest is always a directory?
	b, c := makeBuild(t, "", Config{})
	cmd := &CommandCopy{ConfigCommand{
		args: []string{"testdata/Rockerfile", "/Rockerfile"},
	}}

	c.On("CreateContainer", mock.AnythingOfType("State")).Return("456", nil).Run(func(args mock.Arguments) {
		arg := args.Get(0).(State)
		// TODO: a better check
		assert.True(t, len(arg.Config.Cmd) > 0)
	}).Once()

	c.On("UploadToContainer", "456", mock.AnythingOfType("*io.PipeReader"), "/").Return(nil).Once()

	state, err := cmd.Execute(b)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("state: %# v", pretty.Formatter(state))

	c.AssertExpectations(t)
	assert.Equal(t, "456", state.ContainerID)
}

// TODO: test Cleanup
