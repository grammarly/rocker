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

func processTemplate(t *testing.T, tpl string) string {
	result, err := Process("test", strings.NewReader(tpl), configTemplateVars, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	return result.String()
}
