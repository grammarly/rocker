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

func TestBuildArg_EnvSubstitute1(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG deployenv=development
ENV DEPLOY_ENVIRONMENT=$deployenv
RUN env`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--no-cache", "--no-garbage"},
		testLines:         []string{"DEPLOY_ENVIRONMENT=development"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_EnvSubstitute2(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG deployenv=development
ENV DEPLOY_ENVIRONMENT=$deployenv
RUN env`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "deployenv=uat", "--no-cache", "--no-garbage"},
		testLines:         []string{"DEPLOY_ENVIRONMENT=uat"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_FallbackEnv_NoValue(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG NPM_TOKEN
RUN echo NPM_TOKEN_VALUE=$NPM_TOKEN`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--no-cache", "--no-garbage"},
		testLines:         []string{"NPM_TOKEN_VALUE="},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_FallbackEnv_ArgDefault(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG NPM_TOKEN=arg-default
RUN echo NPM_TOKEN_VALUE=$NPM_TOKEN`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--no-cache", "--no-garbage"},
		testLines:         []string{"NPM_TOKEN_VALUE=arg-default"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_FallbackEnv_CliOverride_NoEnv(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG NPM_TOKEN=arg-default
RUN echo NPM_TOKEN_VALUE=$NPM_TOKEN`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "NPM_TOKEN=cli-value", "--no-cache", "--no-garbage"},
		testLines:         []string{"NPM_TOKEN_VALUE=cli-value"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_FallbackEnv_CliOverride_HasEnv(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG NPM_TOKEN=arg-default
RUN echo NPM_TOKEN_VALUE=$NPM_TOKEN`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "NPM_TOKEN=cli-value", "--no-cache", "--no-garbage"},
		testLines:         []string{"NPM_TOKEN_VALUE=cli-value"},
		env:               rockerBuildEnv("NPM_TOKEN=env-value"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_FallbackEnv_CliOmitValue_NoEnv(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG NPM_TOKEN=arg-default
RUN echo NPM_TOKEN_VALUE=$NPM_TOKEN`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "NPM_TOKEN", "--no-cache", "--no-garbage"},
		testLines:         []string{"NPM_TOKEN_VALUE="},
		env:               rockerBuildEnv(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_FallbackEnv_CliOmitValue_HasEnv1(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG NPM_TOKEN=arg-default
RUN echo NPM_TOKEN_VALUE=$NPM_TOKEN`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "NPM_TOKEN", "--no-cache", "--no-garbage"},
		testLines:         []string{"NPM_TOKEN_VALUE=env-value"},
		env:               rockerBuildEnv("NPM_TOKEN=env-value"),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuildArg_FallbackEnv_CliOmitValue_ForceNoValue(t *testing.T) {
	rockerfileContent := `
FROM alpine
ARG NPM_TOKEN=arg-default
RUN echo NPM_TOKEN_VALUE=$NPM_TOKEN`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--build-arg", "NPM_TOKEN=", "--no-cache", "--no-garbage"},
		testLines:         []string{"NPM_TOKEN_VALUE="},
		env:               rockerBuildEnv("NPM_TOKEN=env-value"),
	})
	if err != nil {
		t.Fatal(err)
	}
}
