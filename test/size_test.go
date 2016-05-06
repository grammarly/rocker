package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

type rockerBuildFn func(string, io.Writer, ...string) error

func runImageSizeTestWithDockerVersion(t *testing.T, dockerVersion string) {

	cleanup, rocker, err := setupDockerVersionedEnv(t, dockerVersion)
	assert.Nil(t, err, "setup env")

	defer cleanup()

	time.Sleep(2 * 1e9)

	fmt.Fprintf(os.Stdout, "docker env ready, building test image\n")

	rockerFile := `
FROM busybox:latest
RUN dd if=/dev/zero of=/root/binary1M-1 bs=1 count=1000000
TAG tag1
RUN dd if=/dev/zero of=/root/binary1M-2 bs=1 count=1000000
TAG tag2`

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
				fmt.Printf("decode error: %s", err)
				return
			}
			fmt.Printf("decoded: %#v\n", pretty.Formatter(m))

			size0, ok1 := m["size"]
			delta0, ok2 := m["delta"]

			if ok1 && ok2 {
				size1 := int(size0.(float64))
				delta1 := int(delta0.(float64))
				fmt.Printf("size(%v) delta(%v)", size1, delta1)

				deltas = append(deltas, delta1)
			}
		}
		fmt.Printf("returning: %s\n", deltas)

		result <- deltas
	}()

	err = rocker(rockerFile, wr)
	assert.Nil(t, err, "build should finish ok")
	fmt.Printf("build finished\n")
	wr.Close()

	deltas := <-result

	assert.Equal(t, deltas, []int{1000000, 1000000, 2000000}, "deltas should be correct")

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

func runDockerContainer(tag string, cmd []string, binds []string) (*docker.Container, error, func()) {
	client, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		log.Fatal("error creating docker client: %s", err)
		return nil, err, nil
	}

	containerConfig := docker.Config{
		Image: tag,
		Cmd:   cmd,
	}

	hostConfig := docker.HostConfig{
		Binds:      binds,
		Privileged: true,
	}

	container, err := client.CreateContainer(docker.CreateContainerOptions{
		Config: &containerConfig,
	})
	if err != nil {
		panic("create container: " + err.Error())
	}

	err = client.StartContainer(container.ID, &hostConfig)
	if err != nil {
		panic("start")
	}

	container1, err := client.InspectContainer(container.ID)
	if err != nil {
		panic("inspect")
	}

	cleanup := func() {

		fmt.Fprintf(os.Stdout, "cleaning up a container: %s\n", container1.ID)

		client.KillContainer(docker.KillContainerOptions{
			ID: container1.ID,
		})

		client.RemoveContainer(docker.RemoveContainerOptions{
			ID: container1.ID,
		})
	}

	return container1, err, cleanup
}

func setupDockerVersionedEnv(t *testing.T, version string) (func(), rockerBuildFn, error) {

	dockerImageTag := "test-docker-" + version

	rockerFile := `
FROM ubuntu:trusty
RUN echo deb http://apt.dockerproject.org/repo ubuntu-trusty main > /etc/apt/sources.list.d/docker.list 
RUN apt-key adv --keyserver keyserver.ubuntu.com --recv F76221572C52609D
RUN apt-get update && apt-get install --yes docker-engine=` + version + `.*
TAG ` + dockerImageTag

	if err := runRockerBuildWithOptions(rockerFile); err != nil {
		t.Fatal("build fail:", dockerImageTag, err)
	}

	cmd := []string{"docker", "daemon", "-D", "-s", "overlay", "-H", "0.0.0.0:12345"}

	// docker 1.11 fails to create containers inside container with overlayfs driver,
	// so we mount some host directory on
	// /var/lib/docker inside docker-version container
	tempDir := makeTempDir(t, "var-lib-docker-"+version, nil)

	c, err, cleanup1 := runDockerContainer(dockerImageTag, cmd, []string{tempDir + ":/var/lib/docker"})
	if err != nil {
		t.Fatal("failed to create container", err)
	}

	cleanup2 := func() {
		defer cleanup1()
		defer os.RemoveAll(tempDir)
	}

	data, err := json.Marshal(c)
	assert.Nil(t, err, "marshal")

	fmt.Fprintf(os.Stdout, "created container: %s\n", string(data))

	fmt.Fprintf(os.Stdout, "container ip: %s\n", c.NetworkSettings.IPAddress)

	rocker := func(rockerFile0 string, stdout io.Writer, opts ...string) error {
		return runRockerBuildWithOptions2(rockerBuildOptions{
			Rockerfile:    rockerFile0,
			GlobalOptions: []string{"-H", c.NetworkSettings.IPAddress + ":12345", "--json"},
			BuildOptions:  opts,
			Stdout:        stdout,
		})
	}

	return cleanup2, rocker, err
}
