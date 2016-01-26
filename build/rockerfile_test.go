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

package build

import (
	"rocker/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRockerfile_Base(t *testing.T) {
	src := `FROM {{ .BaseImage }}`
	vars := template.Vars{"BaseImage": "ubuntu"}
	r, err := NewRockerfile("test", strings.NewReader(src), vars, template.Funs{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, src, r.Source)
	assert.Equal(t, "FROM ubuntu", r.Content)
}

func TestNewRockerfileFromFile(t *testing.T) {
	r, err := NewRockerfileFromFile("testdata/Rockerfile", template.Vars{}, template.Funs{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, `from "some-java8-image-dev:1"`, r.rootNode.Children[0].Dump())
}

func TestRockerfileCommands(t *testing.T) {
	src := `FROM ubuntu`
	r, err := NewRockerfile("test", strings.NewReader(src), template.Vars{}, template.Funs{})
	if err != nil {
		t.Fatal(err)
	}

	commands := r.Commands()
	assert.Len(t, commands, 1)
	assert.Equal(t, "from", commands[0].name)
	assert.Equal(t, "ubuntu", commands[0].args[0])
}

func TestRockerfileParseOnbuildCommands(t *testing.T) {
	triggers := []string{
		"RUN make",
		"RUN make install",
	}

	commands, err := parseOnbuildCommands(triggers)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, commands, 2)
	assert.Equal(t, "run", commands[0].name)
	assert.Equal(t, []string{"make"}, commands[0].args)
	assert.Equal(t, "run", commands[1].name)
	assert.Equal(t, []string{"make install"}, commands[1].args)
}
