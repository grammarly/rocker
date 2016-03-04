package tests

import (
	"testing"
)

func TestCacheWorksByDefault(t *testing.T) {
	tag := "rocker-integratin-test:1.2.3"

	err := runRockerBuildWithOptions(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)

	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuildWithOptions(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)
	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	if sha1 != sha2 {
		t.Fail()
	}
}

func TestNoCache(t *testing.T) {
	tag := "rocker-integratin-test:1.2.3"

	err := runRockerBuildWithOptions(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)
	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuildWithOptions(`
FROM alpine
RUN touch /tmp/foo
TAG `+tag, "--no-cache")
	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	if sha1 == sha2 {
		t.Fatalf("Sha of images are equal but shouldn't. sha1: %s, sha2: %s", sha1, sha2)
	}
}
