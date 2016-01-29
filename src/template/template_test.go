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
	"fmt"
	"github.com/grammarly/rocker/src/imagename"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	configTemplateVars = Vars{
		"mykey": "myval",
		"n":     "5",
		"data": map[string]string{
			"foo": "bar",
		},
		"RockerArtifacts": []imagename.Artifact{
			imagename.Artifact{
				Name: imagename.NewFromString("alpine:3.2"),
				Tag:  "3.2",
			},
			imagename.Artifact{
				Name:   imagename.NewFromString("golang:1.5"),
				Tag:    "1.5",
				Digest: "sha256:ead434",
			},
			imagename.Artifact{
				Name:   imagename.NewFromString("data:master"),
				Tag:    "master",
				Digest: "sha256:fafe14",
			},
			imagename.Artifact{
				Name:   imagename.NewFromString("ssh:latest"),
				Tag:    "latest",
				Digest: "sha256:ba41cd",
			},
		},
	}
)

func TestProcess_Basic(t *testing.T) {
	result, err := Process("test", strings.NewReader("this is a test {{.mykey}}"), configTemplateVars, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	// fmt.Printf("Template result: %s\n", result)
	assert.Equal(t, "this is a test myval", result.String(), "template should be rendered")
}

func TestProcess_Seq(t *testing.T) {
	assert.Equal(t, "[1 2 3 4 5]", processTemplate(t, "{{ seq 1 5 1 }}"))
	assert.Equal(t, "[0 1 2 3 4]", processTemplate(t, "{{ seq 0 4 1 }}"))
	assert.Equal(t, "[1 3 5]", processTemplate(t, "{{ seq 1 5 2 }}"))
	assert.Equal(t, "[1 4]", processTemplate(t, "{{ seq 1 5 3 }}"))
	assert.Equal(t, "[1 5]", processTemplate(t, "{{ seq 1 5 4 }}"))
	assert.Equal(t, "[1]", processTemplate(t, "{{ seq 1 5 5 }}"))

	assert.Equal(t, "[1]", processTemplate(t, "{{ seq 1 1 1 }}"))
	assert.Equal(t, "[1]", processTemplate(t, "{{ seq 1 1 5 }}"))

	assert.Equal(t, "[5 4 3 2 1]", processTemplate(t, "{{ seq 5 1 1 }}"))
	assert.Equal(t, "[5 3 1]", processTemplate(t, "{{ seq 5 1 2 }}"))
	assert.Equal(t, "[5 2]", processTemplate(t, "{{ seq 5 1 3 }}"))
	assert.Equal(t, "[5 1]", processTemplate(t, "{{ seq 5 1 4 }}"))
	assert.Equal(t, "[5]", processTemplate(t, "{{ seq 5 1 5 }}"))

	assert.Equal(t, "[1 2 3 4 5]", processTemplate(t, "{{ seq 5 }}"))
	assert.Equal(t, "[1]", processTemplate(t, "{{ seq 1 }}"))
	assert.Equal(t, "[]", processTemplate(t, "{{ seq 0 }}"))
	assert.Equal(t, "[-1 -2 -3 -4 -5]", processTemplate(t, "{{ seq -5 }}"))

	assert.Equal(t, "[1 2 3 4 5]", processTemplate(t, "{{ seq 1 5 }}"))
	assert.Equal(t, "[1]", processTemplate(t, "{{ seq 1 1 }}"))
	assert.Equal(t, "[0]", processTemplate(t, "{{ seq 0 0 }}"))
	assert.Equal(t, "[-1 -2 -3 -4 -5]", processTemplate(t, "{{ seq -1 -5 }}"))

	// Test string param
	assert.Equal(t, "[1 2 3 4 5]", processTemplate(t, "{{ seq .n }}"))
}

func TestProcess_Replace(t *testing.T) {
	assert.Equal(t, "url-com-", processTemplate(t, `{{ replace "url.com." "." "-" -1 }}`))
	assert.Equal(t, "url", processTemplate(t, `{{ replace "url" "*" "l" -1 }}`))
	assert.Equal(t, "krl", processTemplate(t, `{{ replace "url" "u" "k" -1 }}`))
}

func TestProcess_Env(t *testing.T) {
	env := os.Environ()
	kv := strings.SplitN(env[0], "=", 2)
	assert.Equal(t, kv[1], processTemplate(t, fmt.Sprintf("{{ .Env.%s }}", kv[0])))
}

func TestProcess_Dump(t *testing.T) {
	assert.Equal(t, `map[string]string{"foo":"bar"}`, processTemplate(t, "{{ dump .data }}"))
}

func TestProcess_AssertSuccess(t *testing.T) {
	assert.Equal(t, "output", processTemplate(t, "{{ assert true }}output"))
}

func TestProcess_AssertFail(t *testing.T) {
	tpl := "{{ assert .Version }}lololo"
	_, err := Process("test", strings.NewReader(tpl), configTemplateVars, map[string]interface{}{})
	errStr := "Error executing template test, error: template: test:1:3: executing \"test\" at <assert .Version>: error calling assert: Assertion failed"
	assert.Equal(t, errStr, err.Error())
}

func TestProcess_Json(t *testing.T) {
	assert.Equal(t, "key: {\"foo\":\"bar\"}", processTemplate(t, "key: {{ .data | json }}"))
}

func TestProcess_Shellarg(t *testing.T) {
	assert.Equal(t, "echo 'hello world'", processTemplate(t, "echo {{ \"hello world\" | shell }}"))
}

func TestProcess_Yaml(t *testing.T) {
	assert.Equal(t, "key: foo: bar\n", processTemplate(t, "key: {{ .data | yaml }}"))
	assert.Equal(t, "key: myval\n", processTemplate(t, "key: {{ .mykey | yaml }}"))
	assert.Equal(t, "key: |-\n  hello\n  world\n", processTemplate(t, "key: {{ \"hello\\nworld\" | yaml }}"))
}

func TestProcess_YamlIndent(t *testing.T) {
	assert.Equal(t, "key:\n  foo: bar\n", processTemplate(t, "key:\n{{ .data | yaml 1 }}"))
}

func TestProcess_Image_Simple(t *testing.T) {
	tests := []struct {
		tpl     string
		result  string
		message string
	}{
		{"{{ image `debian:7.7` }}", "debian:7.7", "should not alter the tag that is not in artifacts"},
		{"{{ image `debian` `7.7` }}", "debian:7.7", "should be possible to specify tag as a separate argument"},
		{"{{ image `debian` `sha256:afa` }}", "debian@sha256:afa", "should be possible to specify digest as a separate argument"},
	}

	for _, test := range tests {
		assert.Equal(t, test.result, processTemplate(t, test.tpl), test.message)
	}
}

func TestProcess_Image_Advanced(t *testing.T) {
	tests := []struct {
		in          string
		result      string
		shouldMatch bool
		message     string
	}{
		{"debian:7.7", "debian:7.7", false, "should not alter the tag that is not in artifacts"},
		{"debian:7.*", "debian:7.*", false, "should not alter the semver tag that is not in artifacts"},
		{"debian", "debian:latest", false, "should not match anything when no tag given (:latest) and no artifact"},
		{"alpine:3.1", "alpine:3.1", false, "should not match artifact with different version"},
		{"alpine:4.1", "alpine:4.1", false, "should not match artifact with different version"},
		{"alpine:3.*", "alpine:3.2", true, "should match artifact with version wildcard"},
		{"alpine", "alpine:latest", false, "should not match artifact when no tag given (:latest by default)"},
		{"alpine:latest", "alpine:latest", false, "should not match on a :latest tag"},
		{"alpine:snapshot", "alpine:snapshot", false, "should not match on a named tag"},
		{"golang:1.5", "golang@sha256:ead434", true, "should match semver tag and use digest"},
		{"golang:1.*", "golang@sha256:ead434", true, "should match on wildcard semver tag and use digest"},
		{"golang:1", "golang@sha256:ead434", true, "should match on prefix semver tag and use digest"},
		{"golang:1.4", "golang:1.4", false, "should not match on different semver tag"},
		{"golang:master", "golang:master", false, "should not match on a named tag"},
		{"data:1.2", "data:1.2", false, "should not match on a version tag against named artifact"},
		{"data:snapshot", "data:snapshot", false, "should not match on a different named tag against named artifact"},
		{"data:master", "data@sha256:fafe14", true, "should match on a same named tag against named artifact"},
		{"ssh:latest", "ssh@sha256:ba41cd", true, "should match on a :latest tag against :latest artifact"},
		{"ssh", "ssh@sha256:ba41cd", true, "should match on non-tagged tag against :latest artifact"},
		{"ssh:master", "ssh:master", false, "should match with other tag against :latest artifact"},
		{"ssh:1.2", "ssh:1.2", false, "should match with semver tag against :latest artifact"},
	}

	for _, test := range tests {
		tpl := fmt.Sprintf("{{ image `%s` }}", test.in)
		assert.Equal(t, test.result, processTemplate(t, tpl), test.message)
	}

	// Now test the same but with DemandArtifact On
	configTemplateVars["DemandArtifacts"] = true
	defer func() {
		configTemplateVars["DemandArtifacts"] = false
	}()

	for _, test := range tests {
		tpl := fmt.Sprintf("{{ image `%s` }}", test.in)
		if test.shouldMatch {
			assert.Equal(t, test.result, processTemplate(t, tpl), test.message)
		} else {
			err := processTemplateReturnError(t, tpl)
			assert.Error(t, err, fmt.Sprintf("should give an error for test case: %s", test.message))
			if err != nil {
				assert.Contains(t, err.Error(), fmt.Sprintf("Cannot find suitable artifact for image %s", test.in), test.message)
			}
		}
	}
}

func processTemplate(t *testing.T, tpl string) string {
	result, err := Process("test", strings.NewReader(tpl), configTemplateVars, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	return result.String()
}

func processTemplateReturnError(t *testing.T, tpl string) error {
	_, err := Process("test", strings.NewReader(tpl), configTemplateVars, map[string]interface{}{})
	return err
}
