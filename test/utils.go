package tests

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func runCmd(executable string, stdoutWriter io.Writer /* stderr io.Writer,*/, params ...string) error {
	cmd := exec.Command(executable, params...)
	if *verbosityLevel >= 1 {
		fmt.Printf("Running: %v\n", strings.Join(cmd.Args, " "))
	}

	if stdoutWriter != nil {
		cmd.Stdout = stdoutWriter
	} else if *verbosityLevel >= 2 {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func removeImage(imageName string) error {
	return runCmd("docker", nil, "rmi", imageName)
}

func getImageShaByName(imageName string) (string, error) {
	var b bytes.Buffer

	if err := runCmd("docker", bufio.NewWriter(&b), "images", "-q", imageName); err != nil {
		fmt.Println("Can't execute command:", err)
		return "", err
	}

	sha := strings.Trim(b.String(), "\n")

	if len(sha) < 12 {
		return "", errors.New("Too short sha")
	}

	//fmt.Printf("Image: %v, size: %d\n", sha, len(sha))

	return sha, nil
}
func getRockerBinaryPath() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		panic("$GOPATH is not defined")
	}
	return gopath + "/bin/rocker"
}

func runRockerPull(image string) error {
	if err := runCmd(getRockerBinaryPath(), nil, "pull", image); err != nil {
		return err
	}

	return nil
}
func runRockerWithFile(filename string) error {
	if err := runCmd(getRockerBinaryPath(), nil, "build", "--no-cache", "-f", filename); err != nil {
		return err
	}

	return nil
}

func createTempFile(content string) (string, error) {
	tmpfile, err := ioutil.TempFile("/tmp/", "rocker_integration_test_")
	if err != nil {
		return "", err
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		return "", err
	}
	if err := tmpfile.Close(); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

func runRockerBuildWithFile(filename string, opts ...string) error {
	p := []string{"build", "-f", filename}
	params := append(p, opts...)

	if err := runCmd(getRockerBinaryPath(), nil, params...); err != nil {
		return err
	}

	return nil
}
func runRockerBuildWithOptions(content string, opts ...string) error {
	filename, err := createTempFile(content)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filename)

	p := []string{"build", "-f", filename}
	params := append(p, opts...)
	if err := runCmd(getRockerBinaryPath(), nil, params...); err != nil {
		return err
	}

	return nil
}

func runRockerBuild(content string) error {
	return runRockerBuildWithOptions(content)
}
