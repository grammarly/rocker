package tests

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/grammarly/rocker/src/test"
	"github.com/kr/pretty"
	"github.com/kr/text"
	"github.com/mitchellh/go-homedir"
)

var (
	outputShaRe = regexp.MustCompile("Successfully built (sha256:[a-f0-9]+)")
)

func runCmd(command string, args ...string) (string, error) {
	return runCmdWithOptions(cmdOptions{
		command: command,
		args:    args,
	})
}

type cmdOptions struct {
	command string
	args    []string
	workdir string
	stdout  io.Writer
	env     []string
}

func runCmdWithOptions(opts cmdOptions) (string, error) {
	cmd := exec.Command(opts.command, opts.args...)

	if opts.env != nil {
		cmd.Env = opts.env
	}

	if *verbosityLevel >= 1 {
		fmt.Printf("Running: %+v\n", opts)
	}

	if opts.workdir != "" {
		cmd.Dir = opts.workdir
	}

	var (
		buf bytes.Buffer
		w   = []io.Writer{&buf}
	)

	if opts.stdout != nil {
		w = append(w, opts.stdout)
	}

	if *verbosityLevel >= 2 {
		if opts.stdout != os.Stdout {
			w = append(w, os.Stdout)
		}
		cmd.Stderr = os.Stderr
	}

	cmd.Stdout = io.MultiWriter(w...)

	if err := cmd.Run(); err != nil {
		return "", &errCmdRun{
			args: cmd.Args,
			err:  err,
			out:  buf.String(),
			wd:   cmd.Dir,
		}
	}

	return buf.String(), nil
}

func removeImage(imageName string) error {
	_, err := runCmd("docker", "rmi", imageName)
	return err
}

func getImageShaByName(imageName string) (string, error) {
	out, err := runCmd("docker", "images", "-q", imageName)
	if err != nil {
		return "", fmt.Errorf("Failed to get image SHA by name %s, error: %s", imageName, err)
	}

	sha := strings.Trim(out, "\n")

	if len(sha) < 12 {
		return "", fmt.Errorf("Too short sha (should be at least 12 chars) got: %q", sha)
	}

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
	_, err := runCmd(getRockerBinaryPath(), "pull", image)
	return err
}

func runRockerBuild(content string, params ...string) error {
	return runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: content,
		buildParams:       params,
	})
}

func runRockerBuildWithFile(filename string, params ...string) error {
	return runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileName: filename,
		buildParams:    params,
	})
}

func runRockerBuildWd(wd string, params ...string) error {
	return runRockerBuildWithOptions(rockerBuildOptions{
		workdir:     wd,
		buildParams: params,
	})
}

type rockerBuildOptions struct {
	rockerfileName    string
	rockerfileContent string
	globalParams      []string
	buildParams       []string
	testLines         []string
	env               []string
	workdir           string
	stdout            io.Writer
	sha               *string
}

func runRockerBuildWithOptions(opts rockerBuildOptions) error {
	if opts.rockerfileName != "" && opts.rockerfileContent != "" {
		return fmt.Errorf("runRockerBuildWithOptions fail, cannot have both `rockerfile` and `rockerfileContent`, got %v", opts)
	}

	if opts.rockerfileContent != "" {
		var err error
		if opts.rockerfileName, err = createTempFile(opts.rockerfileContent); err != nil {
			return err
		}
		defer os.RemoveAll(opts.rockerfileName)

	} else if opts.rockerfileName == "" {
		// if no `rockerfileContent` nor `rockerfile` specified, try to make default `rockerfile`
		if opts.workdir == "" {
			return fmt.Errorf("Workdir should be passed if none `rockerfile` and `rockerfileContent` specified, got %v", opts)
		}
		opts.rockerfileName = path.Join(opts.workdir, "Rockerfile")
	}

	if opts.rockerfileName != "" {
		rockerfileContent, err := ioutil.ReadFile(opts.rockerfileName)
		if err != nil {
			return fmt.Errorf("Failed to read rockerfile, error: %s", err)
		}
		opts.rockerfileContent = string(rockerfileContent)
	}

	args := append(opts.globalParams, "build", "-f", opts.rockerfileName)
	args = append(args, opts.buildParams...)

	output, err := runCmdWithOptions(cmdOptions{
		command: getRockerBinaryPath(),
		args:    args,
		env:     opts.env,
		workdir: opts.workdir,
		stdout:  opts.stdout,
	})

	if err != nil {
		if e, ok := err.(*errCmdRun); ok {
			return &errRockerBuildRun{
				cmdErr:            e,
				rockerfileContent: string(opts.rockerfileContent),
			}
		}

		return fmt.Errorf("Failed to run rocker build, error: %s", err)
	}

	if opts.sha != nil {
		if match := outputShaRe.FindStringSubmatch(output); match != nil {
			*opts.sha = match[1]
		} else {
			return fmt.Errorf("Expected rocker build to return image SHA, got nothing.\n\nRocker build output:\n%s", output)
		}
	}

	if len(opts.testLines) > 0 {
		linesMap := map[string]int{}
		for _, l := range strings.Split(output, "\n") {
			linesMap[l] = linesMap[l] + 1
		}

		for _, l := range opts.testLines {
			if linesMap[l] == 0 {
				return fmt.Errorf("Expected rocker build output to contain the following output line:\n%s\n\nRocker build output:\n%s", l, output)
			}
		}
	}

	return nil
}

func runTimeout(name string, timeout time.Duration, f func() error) error {
	errChan := make(chan error)
	go func() {
		errChan <- f()
	}()

	select {
	case err := <-errChan:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("%s timeout %s", name, timeout)
	}
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

func makeTempDir(t *testing.T, prefix string, files map[string]string) string {
	// We produce tmp dirs within home to make integration tests work within
	// Mac OS and VirtualBox
	home, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}

	baseTmpDir := path.Join(home, ".rocker-integ-tmp")

	if err := os.MkdirAll(baseTmpDir, 0755); err != nil {
		log.Fatal(err)
	}

	tmpDir, err := ioutil.TempDir(baseTmpDir, prefix)
	if err != nil {
		t.Fatal(err)
	}
	if err := test.MakeFiles(tmpDir, files); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	if *verbosityLevel >= 2 {
		fmt.Printf("temp directory: %s\n", tmpDir)
		fmt.Printf("  with files: %# v\n", pretty.Formatter(files))
	}
	return tmpDir
}

func randomString() string {
	return strconv.Itoa(int(time.Now().UnixNano() % int64(100000001)))
}

func debugf(format string, args ...interface{}) {
	if *verbosityLevel >= 2 {
		fmt.Printf(format, args...)
	}
}

func rockerBuildEnv(envs ...string) []string {
	baseEnv := []string{
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
	}
	return append(baseEnv, envs...)
}

type errCmdRun struct {
	args []string
	err  error
	out  string
	wd   string
}

func (e *errCmdRun) Error() string {
	return fmt.Sprintf(`Failed to run command: %s
Error: %s
Workdir: %s
Output:
%s`, strings.Join(e.args, " "), e.err, e.wd, e.out)
}

type errRockerBuildRun struct {
	cmdErr            *errCmdRun
	rockerfileContent string
}

func (e *errRockerBuildRun) Error() string {
	sep := "\n---------------------------------\n"
	return fmt.Sprintf("Failed to run rocker build:\n\nRockerfile:%s%s%sCmd Error:\n%s",
		sep, e.rockerfileContent, sep, text.Indent(e.cmdErr.Error(), "           "))
}
