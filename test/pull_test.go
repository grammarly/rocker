package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

const pullPushTimeout = 60 * time.Second

func TestPull_Simple(t *testing.T) {
	err := runTimeout("rocker pull", pullPushTimeout, func() error {
		return runRockerPull("alpine")
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPull_PushAndPull(t *testing.T) {
	randomData := randomString()

	auth, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		t.Fatal("Failed to obtain docker auth configuration from a file (for testing PULL), you may need to 'docker login':", err)
	}

	var (
		username string
		registry = "index.docker.io"
	)

	if c, ok := auth.Configs["https://"+registry+"/v1/"]; ok {
		username = c.Username
	} else if c, ok := auth.Configs["https://"+registry+"/v2/"]; ok {
		username = c.Username
	}

	if username == "" {
		t.Fatalf("Cannot find docker login for registry %s, make sure you did 'docker login' properly.", registry)
	}

	tag := fmt.Sprintf("%s/rocker_integration_test_pull:latest-%s", username, randomData)
	defer removeImage(tag)

	err = runTimeout("rocker build", pullPushTimeout, func() error {
		return runRockerBuild("FROM alpine\nRUN echo "+randomData+" > /foobar\nPUSH "+tag, "--push")
	})
	if err != nil {
		t.Fatal(err)
	}

	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	if err := removeImage(tag); err != nil {
		t.Fatal(err)
	}

	err = runTimeout("rocker pull", pullPushTimeout, func() error {
		return runRockerPull(tag)
	})
	if err != nil {
		t.Fatal(err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, sha1, sha2, "pushed and pulled images should have equal sha")
}
