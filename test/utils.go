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

func getGOPATH() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		panic("$GOPATH is not defined")
	}

	return gopath
}

func runCmd(executable string, stdoutWriter io.Writer /* stderr io.Writer,*/, params ...string) error {
	cmd := exec.Command(executable, params...)
	fmt.Printf("Running: %v\n", strings.Join(cmd.Args, " "))
	if stdoutWriter != nil {
		cmd.Stdout = stdoutWriter
	}
	//cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		//fmt.Printf("Failed to run '%v' with arguments '%v'\n", executable, params)
		return err
	}

	return nil
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

func runRockerWithFile(filename string) error {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		panic("$GOPATH is not defined")
	}

	if err := runCmd(gopath+"/bin/rocker", nil, "build", "--no-cache", "-f", filename); err != nil {
		//fmt.Errorf("Failed to run rocker with filename '%v'", filename)
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

func runRockerBuildWithOptions(content string, opts ...string) error {
	filename, err := createTempFile(content)
	if err != nil {
		return err
	}
	defer os.Remove(filename)

	gopath := getGOPATH()

	p := []string{"build", "-f", filename}
	params := append(p, opts...)
	if err := runCmd(gopath+"/bin/rocker", nil, params...); err != nil {
		//fmt.Printf("Failed to run rocker with filename '%v'", filename)
		return err
	}

	return nil
}

func runRockerBuild(content string) error {
	return runRockerBuildWithOptions(content)
}
