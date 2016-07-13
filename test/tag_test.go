package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTag_Different(t *testing.T) {
	tag := "rocker_integration_test_tag:latest"
	defer removeImage(tag)

	err := runRockerBuild("FROM alpine:latest\nRUN touch /foobar\nTAG "+tag, "--no-cache")
	if err != nil {
		t.Fatal(err)
	}

	sha1, err := getImageShaByName("alpine:latest")
	if err != nil {
		t.Fatal(err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEqual(t, sha1, sha2, "Sha's of source image and tag are identical but shouldn't")
}
func TestTag_TheSame(t *testing.T) {
	tag := "rocker_integration_test_tag:latest"
	defer removeImage(tag)

	err := runRockerBuild("FROM alpine:latest\nTAG "+tag, "--no-cache")
	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha1, err := getImageShaByName("alpine:latest")
	if err != nil {
		t.Fatal(err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, sha1, sha2, "Sha's of source image and tag mismatch")
}
