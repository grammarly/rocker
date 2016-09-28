package tests

import (
	"encoding/json"

	"github.com/stretchr/testify/assert"

	"github.com/fsouza/go-dockerclient"

	"testing"
)

func TestHealthcheck_Simple(t *testing.T) {
	container := healthcheckBuildInspect(t, `
FROM alpine
HEALTHCHECK --interval=5m --timeout=3s \
  CMD curl -f http://localhost/ || exit 1
`)

	assert.Equal(t, []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}, container.Config.Healthcheck.Test)
	assert.Equal(t, "5m0s", container.Config.Healthcheck.Interval.String())
	assert.Equal(t, "3s", container.Config.Healthcheck.Timeout.String())
}

func TestHealthcheck_Cmd(t *testing.T) {
	container := healthcheckBuildInspect(t, `
FROM alpine
HEALTHCHECK --interval=5m --timeout=3s \
  CMD ["/bin/bash", "-c", "curl -f http://localhost/ || exit 1"]
`)

	assert.Equal(t, []string{"CMD", "/bin/bash", "-c", "curl -f http://localhost/ || exit 1"}, container.Config.Healthcheck.Test)
	assert.Equal(t, "5m0s", container.Config.Healthcheck.Interval.String())
	assert.Equal(t, "3s", container.Config.Healthcheck.Timeout.String())
}

func TestHealthcheck_None(t *testing.T) {
	container := healthcheckBuildInspect(t, `
FROM alpine
HEALTHCHECK NONE
`)

	assert.Equal(t, []string{"NONE"}, container.Config.Healthcheck.Test)
	assert.Equal(t, "0s", container.Config.Healthcheck.Interval.String())
	assert.Equal(t, "0s", container.Config.Healthcheck.Timeout.String())
}

func TestHealthcheck_Override1(t *testing.T) {
	container := healthcheckBuildInspect(t, `
FROM alpine
HEALTHCHECK --interval=5m --timeout=3s \
  CMD curl -f http://localhost/ || exit 1
HEALTHCHECK NONE
`)

	assert.Equal(t, []string{"NONE"}, container.Config.Healthcheck.Test)
	assert.Equal(t, "0s", container.Config.Healthcheck.Interval.String())
	assert.Equal(t, "0s", container.Config.Healthcheck.Timeout.String())
}

func TestHealthcheck_Override2(t *testing.T) {
	container := healthcheckBuildInspect(t, `
FROM alpine
HEALTHCHECK NONE
HEALTHCHECK --interval=5m --timeout=3s \
  CMD curl -f http://localhost/ || exit 1
`)

	assert.Equal(t, []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}, container.Config.Healthcheck.Test)
	assert.Equal(t, "5m0s", container.Config.Healthcheck.Interval.String())
	assert.Equal(t, "3s", container.Config.Healthcheck.Timeout.String())
}

func healthcheckBuildInspect(t *testing.T, rockerfileContent string) *docker.Container {
	var sha string

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--no-cache"},
		sha:               &sha,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer removeImage(sha)

	out, err := runCmd("docker", "inspect", sha)
	if err != nil {
		t.Fatal(err)
	}

	var inspectOutput []docker.Container

	if err := json.Unmarshal([]byte(out), &inspectOutput); err != nil {
		t.Fatal(err)
	}

	if len(inspectOutput) != 1 {
		t.Fatal("Expected `docker inspect` to return exactly one container")
	}

	return &inspectOutput[0]
}
