package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

type rockerBuildFn func(string, io.Writer, ...string) error

func runImageSizeTestWithDockerVersion(t *testing.T, dockerVersion string) {

	rocker, cleanup, err := setupDockerVersionedEnv(t, dockerVersion)
	assert.Nil(t, err, "setup env")

	defer cleanup()

	time.Sleep(2 * time.Second)

	debugf("docker env ready, building test image\n")

	rockerFile := `
FROM busybox:latest
RUN dd if=/dev/zero of=/root/binary1M-1 bs=1 count=1000000
ONBUILD RUN dd if=/dev/zero of=/root/binary1M-2 bs=1 count=1000000
TAG tag1
FROM tag1
RUN dd if=/dev/zero of=/root/binary1M-3 bs=1 count=1000000
RUN echo done`

	rd, wr := io.Pipe()
	jsonRd := json.NewDecoder(rd)

	result := make(chan []int)

	go func() {
		deltas := []int{}
		for {
			m := map[string]interface{}{}
			if err := jsonRd.Decode(&m); err != nil {
				if err == io.EOF {
					break
				}
				debugf("decode error: %s", err)
				result <- []int{}
			}
			debugf("decoded: %#v\n", pretty.Formatter(m))

			size0, ok1 := m["size"]
			delta0, ok2 := m["delta"]

			if ok1 && ok2 {
				size1 := int(size0.(float64))
				delta1 := int(delta0.(float64))
				debugf("size(%v) delta(%v)", size1, delta1)

				deltas = append(deltas, delta1)
			}
		}
		debugf("returning: %v\n", deltas)

		result <- deltas
	}()

	err = rocker(rockerFile, wr)
	assert.Nil(t, err, "build should finish ok")
	debugf("build finished\n")
	wr.Close()

	deltas := <-result

	assert.Equal(t, []int{
		0,       // FROM
		1000000, // RUN dd with binary1M-1
		0,       // ONBUILD RUN dd
		0,       // FROM tag1
		1000000, // onbuild-triggered dd
		1000000, // RUN dd with binary1M-3
		0,       // RUN echo
		2000000, // final delta from tag1
	}, deltas, "deltas should be correct")

}

func TestImageSize_docker_1_9(t *testing.T) {
	runImageSizeTestWithDockerVersion(t, "1.9")
}

func TestImageSize_docker_1_10(t *testing.T) {
	runImageSizeTestWithDockerVersion(t, "1.10")
}

func TestImageSize_docker_1_11(t *testing.T) {
	runImageSizeTestWithDockerVersion(t, "1.11")
}

func runDockerContainer(tag string, cmd []string) (*docker.Container, func(), error) {
	client, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		return nil, nil, fmt.Errorf("error creating client: %s", err.Error())
	}

	containerConfig := docker.Config{
		Image: tag,
		Cmd:   cmd,
		ExposedPorts: map[docker.Port]struct{}{
			docker.Port("2375/tcp"): struct{}{},
		},
	}

	hostConfig := docker.HostConfig{
		Privileged: true,
		PortBindings: map[docker.Port][]docker.PortBinding{
			docker.Port("2375/tcp"): []docker.PortBinding{docker.PortBinding{}},
		},
	}

	container, err := client.CreateContainer(docker.CreateContainerOptions{
		Config:     &containerConfig,
		HostConfig: &hostConfig,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating container: %s", err.Error())
	}

	err = client.StartContainer(container.ID, &hostConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error starting container: %s", err.Error())
	}

	time.Sleep(2 * time.Second)

	container1, err := client.InspectContainer(container.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("error inspecting container: %s", err.Error())
	}

	cleanup := func() {

		debugf("cleaning up a container: %s\n", container1.ID)

		client.KillContainer(docker.KillContainerOptions{
			ID: container1.ID,
		})

		client.RemoveContainer(docker.RemoveContainerOptions{
			ID: container1.ID,
		})
	}

	return container1, cleanup, nil
}

func setupDockerVersionedEnv(t *testing.T, version string) (rockerBuildFn, func(), error) {

	dockerImageTag := "rocker-size-test-docker-" + version

	rockerFile := `
FROM ubuntu:trusty
RUN echo deb http://apt.dockerproject.org/repo ubuntu-trusty main > /etc/apt/sources.list.d/docker.list 
RUN apt-key adv --keyserver keyserver.ubuntu.com --recv F76221572C52609D
RUN apt-get update && apt-get install --yes docker-engine=` + version + `.*
TAG ` + dockerImageTag

	_, err := getImageShaByName(dockerImageTag)
	if err != nil {
		// assume the tag is not found, so build one

		if err := runRockerBuildWithOptions(rockerFile); err != nil {
			t.Fatal("build fail:", dockerImageTag, err)
		}
	}

	cmd := []string{"docker", "daemon", "-D", "-s", "vfs", "-H", "0.0.0.0:2375"}

	c, cleanup, err := runDockerContainer(dockerImageTag, cmd)
	if err != nil {
		t.Fatal("failed to create container", err)
	}

	data, err := json.Marshal(c)
	assert.Nil(t, err, "marshal")

	debugf("created container: %s\n", string(data))

	debugf("container ip: %s\n", c.NetworkSettings.IPAddress)

	rocker := func(rockerFile0 string, stdout io.Writer, opts ...string) error {
		return runRockerBuildWithOptions2(rockerBuildOptions{
			Rockerfile:    rockerFile0,
			GlobalOptions: []string{"-H", "127.0.0.1:" + c.NetworkSettings.Ports[docker.Port("2375/tcp")][0].HostPort, "--json"},
			BuildOptions:  opts,
			Stdout:        stdout,
		})
	}

	return rocker, cleanup, err
}
