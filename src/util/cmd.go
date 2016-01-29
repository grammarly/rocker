/*-
 * Copyright 2015 Grammarly, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package util

// TODO: this stuff is smelling and should be refactored

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
)

// Cmd is a wrapper over os/exec and provides extra convenience stuff
type Cmd struct {
	Args       []string
	Env        []string
	Dir        string
	Stream     io.Writer
	Reader     *io.PipeReader
	Writer     *io.PipeWriter
	ExitStatus int
	Error      error
}

// Pipe pipes cmd stdout/stderr to a given io.Writer
func (cmd *Cmd) Pipe(writer io.Writer) error {
	scanner := bufio.NewScanner(cmd.Reader)
	for scanner.Scan() {
		writer.Write(scanner.Bytes())
		writer.Write([]byte("\n"))
	}
	return scanner.Err()
}

// String returns debug representation of the Cmd
func (cmd *Cmd) String() string {
	if len(cmd.Env) > 0 {
		return fmt.Sprintf("%s [Env %s] [Dir %s]", strings.Join(cmd.Args, " "), strings.Join(cmd.Env, " "), cmd.Dir)
	}
	return fmt.Sprintf("%s [Dir %s]", strings.Join(cmd.Args, " "), cmd.Dir)
}

// ExecPipe executes the command and returns its output, exit code and error
func ExecPipe(cmd *Cmd) (string, int, error) {
	var output bytes.Buffer
	cmd, err := Exec(cmd)
	if err != nil {
		return "", 0, err
	}
	err = cmd.Pipe(&output)
	if err != nil {
		return "", 0, err
	}
	return output.String(), cmd.ExitStatus, cmd.Error
}

// Exec runs the command and grabs exit code at the end
// If Stream property is present, then it also pipes stdout/stderr to it
func Exec(cmd *Cmd) (*Cmd, error) {
	reader, writer := io.Pipe()

	cmd.Reader = reader
	cmd.Writer = writer

	execCmd := &exec.Cmd{
		Path: cmd.Args[0],
		Args: cmd.Args,
		Env:  cmd.Env,
		Dir:  cmd.Dir,
	}

	execCmd.Stdout = cmd.Writer
	execCmd.Stderr = cmd.Writer

	go func() {
		defer cmd.Writer.Close()
		err := execCmd.Start()
		if err != nil {
			cmd.Error = err
			return
		}
		if err = execCmd.Wait(); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				// The program has exited with an exit code != 0

				// This works on both Unix and Windows. Although package
				// syscall is generally platform dependent, WaitStatus is
				// defined for both Unix and Windows and in both cases has
				// an ExitStatus() method with the same signature.
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					cmd.ExitStatus = status.ExitStatus()
				}
			} else {
				cmd.Error = err
			}
		}
	}()

	if cmd.Stream != nil {
		err := cmd.Pipe(cmd.Stream)
		if err != nil {
			return cmd, err
		}
	}

	return cmd, nil
}
