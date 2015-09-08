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
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigParse(t *testing.T) {
	t.Parallel()

	fd, err := os.Open("testdata/Rockerfile")
	if err != nil {
		t.Fatal(err)
	}

	node, err := Parse(fd)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Node: %v", node.Dump())

	expected, err := ioutil.ReadFile("testdata/Rockerfile_result")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(expected), node.Dump()+"\n", "invalid AST parsed from Rockerfile")
}

func TestConfigRockerfileAstToString_Base(t *testing.T) {
	t.Parallel()

	fd, err := os.Open("testdata/Rockerfile")
	if err != nil {
		t.Fatal(err)
	}

	node, err := Parse(fd)
	if err != nil {
		t.Fatal(err)
	}

	str, err := RockerfileAstToString(node)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Node String: %v", str)

	expected, err := ioutil.ReadFile("testdata/Rockerfile_string_result")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(expected), str+"\n", "invalid Rockerfile dumped to string")
}

func TestConfigRockerfileAstToString_CmdJson(t *testing.T) {
	t.Parallel()

	node, err := Parse(strings.NewReader("FROM scratch\nCMD [\"-\"]\n"))
	if err != nil {
		t.Fatal(err)
	}

	str, err := RockerfileAstToString(node)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Node String: %v", str)

	assert.Equal(t, "from scratch\ncmd [\"-\"]", str, "invalid Rockerfile dumped to string")
}

func TestConfigRockerfileAstToString_KeyVals(t *testing.T) {
	t.Parallel()

	node, err := Parse(strings.NewReader("FROM scratch\nENV NAME=JOHN\\\n LASTNAME=DOE\nMOUNT a b c\nLABEL ASD QWE SDF"))
	if err != nil {
		t.Fatal(err)
	}

	str, err := RockerfileAstToString(node)
	if err != nil {
		t.Fatal(err)
	}
	// t.Logf("Node String: %v", str)
	// pretty.Println(node)
	// t.Logf("Node: %v", node.Dump())

	assert.Equal(t, "from scratch\nenv NAME=JOHN LASTNAME=DOE\nmount a b c\nlabel ASD=QWE SDF", str, "invalid Rockerfile dumped to string")
}
