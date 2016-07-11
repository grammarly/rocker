package tests

import (
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)

func TestPullSimple(t *testing.T) {
	errChan := make(chan error)
	go func() {
		errChan <- runRockerPull("alpine")
	}()

	select {
	case err := <-errChan:
		assert.Nil(t, err)
	case <-time.After(time.Second * 30):
		t.Fatal("rocker pull timeout")
	}
}

func TestPushAndPull(t *testing.T) {
	randomData := strconv.Itoa(int(time.Now().UnixNano() % int64(100000001)))

	tag := "testrocker/rocker_integration_test_pull:latest" + randomData
	defer removeImage(tag)

	err := runRockerBuildWithOptions("FROM alpine\nRUN echo "+randomData+" > /foobar\nPUSH "+tag, "--push")
	assert.Nil(t, err)

	sha1, err := getImageShaByName(tag)
	assert.Nil(t, err)

	err = removeImage(tag)
	assert.Nil(t, err)

	errChan := make(chan error)
	go func() {
		errChan <- runRockerPull(tag)
	}()

	select {
	case err := <-errChan:
		assert.Nil(t, err)
	case <-time.After(time.Second * 30):
		t.Fatal("rocker pull timeout")
	}

	sha2, err := getImageShaByName(tag)
	assert.Nil(t, err)

	assert.Equal(t, sha1, sha2)
}
