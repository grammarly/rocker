package tests

import (
	"testing"
)

func TestTagTheSame(t *testing.T) {
	tag := "rocker_integration_test_tag:latest"
	defer removeImage(tag)

	err := runRockerBuildWithOptions("FROM alpine:latest\nTAG " + tag)
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

	if sha1 != sha2 {
		t.Fatalf("Sha's of source image and tag mismatch. sha1: '%s', sha2: '%s'", sha1, sha2)
	}
}
