package tests

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const pullPushTimeout = 30 * time.Second

func TestPull_Simple(t *testing.T) {
	err := runTimeout("rocker pull", pullPushTimeout, func() error {
		return runRockerPull("alpine")
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPull_PushAndPull(t *testing.T) {
	randomData := strconv.Itoa(int(time.Now().UnixNano() % int64(100000001)))

	tag := "testrocker/rocker_integration_test_pull:latest" + randomData
	defer removeImage(tag)

	err := runTimeout("rocker build", pullPushTimeout, func() error {
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
