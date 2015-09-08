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

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecBase(t *testing.T) {
	cmd, err := Exec(&Cmd{
		Args: []string{"testdata/prog"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err = cmd.Pipe(&buf)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "1 stdout\n1 stderr\n2 stdout\n2 stderr\n3 stdout\n3 stderr\n4 stdout\n4 stderr\n5 stdout\n5 stderr\n", buf.String())
	assert.Equal(t, 0, cmd.ExitStatus)
}

func TestExecStream(t *testing.T) {
	var buf bytes.Buffer

	cmd, err := Exec(&Cmd{
		Args:   []string{"testdata/prog"},
		Stream: &buf,
	})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "1 stdout\n1 stderr\n2 stdout\n2 stderr\n3 stdout\n3 stderr\n4 stdout\n4 stderr\n5 stdout\n5 stderr\n", buf.String())
	assert.Equal(t, 0, cmd.ExitStatus)
}

func TestExecPipe(t *testing.T) {
	output, exitStatus, err := ExecPipe(&Cmd{
		Args: []string{"testdata/prog"},
	})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "1 stdout\n1 stderr\n2 stdout\n2 stderr\n3 stdout\n3 stderr\n4 stdout\n4 stderr\n5 stdout\n5 stderr\n", output)
	assert.Equal(t, 0, exitStatus)
}

func TestExecPipeError(t *testing.T) {
	output, exitStatus, err := ExecPipe(&Cmd{
		Args: []string{"klwemlwkemw"},
	})

	assert.Equal(t, "", output, "expected error to be")
	assert.Equal(t, "fork/exec klwemlwkemw: no such file or directory", err.Error(), "expected error to be")
	assert.Equal(t, 0, exitStatus)
}

func TestExecPipeExitStatus(t *testing.T) {
	output, exitStatus, err := ExecPipe(&Cmd{
		Args: []string{"testdata/prog", "1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "1 stdout\n1 stderr\n2 stdout\n2 stderr\n3 stdout\n3 stderr\n4 stdout\n4 stderr\n5 stdout\n5 stderr\n", output)
	assert.Equal(t, 1, exitStatus)
}
