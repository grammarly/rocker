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

package template

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVarsToStrings(t *testing.T) {
	t.Parallel()

	type assertion struct {
		vars        Vars
		expectation []string
	}

	tests := []assertion{
		assertion{
			Vars{"FOO": "bar", "XYZ": "oqoq"},
			[]string{"FOO=bar", "XYZ=oqoq"},
		},
		assertion{
			Vars{"": "bar", "XYZ": "oqoq"},
			[]string{"=bar", "XYZ=oqoq"},
		},
		assertion{
			Vars{"asd": "qwe"},
			[]string{"asd=qwe"},
		},
		assertion{
			Vars{"asd": "", "haha": "hehe"},
			[]string{"asd=", "haha=hehe"},
		},
	}

	for _, a := range tests {
		result := a.vars.ToStrings()
		assert.Equal(t, len(a.vars), len(result), "resulting number of strings not match number of vars keys")
		for _, expectation := range a.expectation {
			assert.Contains(t, result, expectation, "failed to narrow down vars to list of strings")
		}
	}
}

func TestVarsFromStrings(t *testing.T) {
	t.Parallel()

	type assertion struct {
		input       []string
		expectation Vars
	}

	tests := []assertion{
		assertion{
			[]string{"FOO=bar", "XYZ=oqoq"},
			Vars{"FOO": "bar", "XYZ": "oqoq"},
		},
		assertion{
			[]string{"=bar", "XYZ=oqoq"},
			Vars{"": "bar", "XYZ": "oqoq"},
		},
		assertion{
			[]string{"asd=qwe"},
			Vars{"asd": "qwe"},
		},
		assertion{
			[]string{"asd=", "haha=hehe"},
			Vars{"asd": "", "haha": "hehe"},
		},
	}

	for _, a := range tests {
		result := VarsFromStrings(a.input)
		assert.Equal(t, len(a.input), len(result), "resulting number of strings not match number of vars keys")
	}
}

func TestVarsReplaceString(t *testing.T) {
	t.Parallel()

	type assertion struct {
		vars        Vars
		input       string
		expectation string
	}

	tests := []assertion{
		assertion{
			Vars{"FOO": "bar"},
			"Hello, this is $FOO",
			"Hello, this is bar",
		},
		assertion{
			Vars{"FOO": "bar"},
			"Hello, this is ${FOO}",
			"Hello, this is bar",
		},
		assertion{
			Vars{"FOO": ""},
			"Hello, this is $FOO",
			"Hello, this is ",
		},
		assertion{
			Vars{"GREETING": "Hello", "NAME": "Hadiyah"},
			"$GREETING,\n$NAME!",
			"Hello,\nHadiyah!",
		},
		assertion{
			Vars{},
			"$GREETING,\n$NAME!",
			"$GREETING,\n$NAME!",
		},
	}

	for _, a := range tests {
		result := a.vars.ReplaceString(a.input)
		assert.Equal(t, a.expectation, result, "failed to substitute variables to a string")
	}
}

func TestVarsJsonMarshal(t *testing.T) {
	v := Vars{"foo": "bar", "asd": "qwe"}
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, `["asd=qwe","foo=bar"]`, string(data), "bad Vars encoded to json")

	v2 := Vars{"asd": "qwe", "foo": "bar"}
	data2, err := json.Marshal(v2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, `["asd=qwe","foo=bar"]`, string(data2), "bad Vars encoded to json (order)")

	v3 := Vars{}
	if err := json.Unmarshal(data2, &v3); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(v3), "bad decoded vars length")
	assert.Equal(t, "qwe", v3["asd"], "bad decoded vars element")
	assert.Equal(t, "bar", v3["foo"], "bad decoded vars element")

	// Test unmarshal map to keep backward capatibility
	m := map[string]string{
		"foo": "bar",
		"asd": "qwe",
	}
	data3, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	v4 := Vars{}
	if err := json.Unmarshal(data3, &v4); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(v4), "bad decoded vars length")
	assert.Equal(t, "qwe", v4["asd"], "bad decoded vars element")
	assert.Equal(t, "bar", v4["foo"], "bad decoded vars element")
}
