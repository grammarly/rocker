package tests

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func runCmd(executable string, stdoutWriter io.Writer /* stderr io.Writer,*/, params ...string) error {
	cmd := exec.Command(executable, params...)
	fmt.Printf("Running: %v\n", strings.Join(cmd.Args, " "))
	//cmd.Stdout = stdoutWriter
	//cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		fmt.Errorf("Failed to run '%v' with arguments '%v'", executable, params)
		return err
	}

	return nil
}

func GetImageShaByName(imageName string) (error, string) {
	var b bytes.Buffer

	if err := runCmd("/usr/local/bin/docker", bufio.NewWriter(&b), " "); err != nil {
		fmt.Println("Can't execute command:", err)
		return err, ""
	}
	fmt.Printf("Image: %v", b)

	return nil, ""
}

func runRocker(filename string) error {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		panic("$GOPATH is not defined")
	}

	if err := runCmd(gopath+"/bin/rocker", nil, "build", "--no-cache", "-f", filename); err != nil {
		fmt.Errorf("Failed to run rocker with filename '%v'", filename)
		return err
	}

	return nil
}

func TestFoot(t *testing.T) {
	GetImageShaByName("ubuntu")
	//runRocker("Rockerfile")
}
