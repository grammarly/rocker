package tests

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"
)

func setDockerVersionedEnv(t *testing.T, version string) {

	dockerImageTag := generateTag(version)

	rockerFile := `
FROM ubuntu:trusty
RUN echo deb http://apt.dockerproject.org/repo ubuntu-trusty main > /etc/apt/sources.list.d/docker.list 
RUN apt-key adv --keyserver keyserver.ubuntu.com --recv F76221572C52609D
RUN apt-get update && apt-get install docker-engine=` + version + `.*
TAG ubuntu-docker-` + dockerTag

	// run docker container
	// runCmd("docker", stdoutWriter, "run", "-i", "--privileged", "ubuntu-docker-1.9", "docker", "daemon", "-D", "-H", "0.0.0.0:12345")
	pid := runCmd("docker", stdoutWriter, "run", "--rm", "-i", "--privileged", "test-docker-1.10", "docker", "daemon", "-D", "-H", "0.0.0.0:12345")

	cleanup := func() {
		kill(pid)
	}

	rocker := func() {
		runRockerBuildWdWithOptions("-H", "host:12345")
	}

	return pid, rockerCommand, cleanup, err
}

func TestImageSize_docker_1_9(t *testing.T) {
	pid, rockerCommand, cleanup, err := setDockerVersionedEnv("1.9")

	defer cleanup()

	rockerFile := `
FROM alpine:latest
RUN dd if=/dev/zero of=/root/binary1M-1 bs=1M count=1
TAG ` + tag1 + `
RUN dd if=/dev/zero of=/root/binary1M-2 bs=1M count=1
TAG ` + tag2

	err, stdout := rockerCommand(tmpDir, "")
	assert.notNil(err, "build finished ok")

	string.Match(stdout, `size 12340123 bytes`)

	string.Match(stdout, `result size 12340123 bytes`)

}

func TestImageSize_docker_1_10(t *testing.T) {
	pid, rockerCommand, cleanup, err := setDockerVersionedEnv("1.10")

	defer cleanup()

	rockerFile := `
FROM alpine:latest
RUN dd if=/dev/zero of=/root/binary1M-1 bs=1M count=1
TAG ` + tag1 + `
RUN dd if=/dev/zero of=/root/binary1M-2 bs=1M count=1
TAG ` + tag2

	err, stdout := rockerCommand(tmpDir, "")
	assert.notNil(err, "build finished ok")

	string.Match(stdout, `size 12340123 bytes`)

	string.Match(stdout, `result size 12340123 bytes`)

}
