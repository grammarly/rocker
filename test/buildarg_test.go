package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildArg_Simple(t *testing.T) {
	rockerfileContent := `
FROM alpine
RUN export | grep proxy`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "http_proxy=http://host:3128", "--no-cache", "--no-garbage"},
		testLines:         []string{"export http_proxy='http://host:3128'"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_CacheHit(t *testing.T) {
	var sha1, sha2 string

	rockerfileContent := `
FROM alpine
ENV test_cache=` + randomString() + `
RUN export | grep proxy`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "http_proxy=http://host:3128"},
		sha:               &sha1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer removeImage(sha1)

	err = runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "http_proxy=http://host:3128"},
		sha:               &sha2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer removeImage(sha2)

	assert.Equal(t, sha1, sha2, "same build-arg should not invalidate cache")
}

func TestBuildArg_CacheInvalidate(t *testing.T) {
	var sha1, sha2 string

	rockerfileContent := `
FROM alpine
ENV test_cache=` + randomString() + `
RUN export | grep proxy`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "http_proxy=http://host:3128"},
		sha:               &sha1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer removeImage(sha1)

	err = runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "http_proxy=http://host:3129"}, // note different port
		sha:               &sha2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer removeImage(sha2)

	assert.NotEqual(t, sha1, sha2, "different build-arg values should invalidate cache")
}
