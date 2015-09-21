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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-yaml/yaml"
	"github.com/kr/pretty"
)

// Process renders config through the template processor.
// vars and additional functions are acceptable.
func Process(name string, reader io.Reader, vars Vars, funcs map[string]interface{}) (*bytes.Buffer, error) {

	var buf bytes.Buffer
	// read template
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("Error reading template %s, error: %s", name, err)
	}

	// merge OS environment variables with the given Vars map
	// todo: maybe, we need to make it configurable
	vars["Env"] = ParseKvPairs(os.Environ())

	// Populate functions
	funcMap := map[string]interface{}{
		"seq":    seq,
		"dump":   dump,
		"assert": assertFn,
		"json":   jsonFn,
		"shell":  EscapeShellarg,
		"yaml":   yamlFn,

		// strings functions
		"compare":      strings.Compare,
		"contains":     strings.Contains,
		"containsAny":  strings.ContainsAny,
		"count":        strings.Count,
		"equalFold":    strings.EqualFold,
		"hasPrefix":    strings.HasPrefix,
		"hasSuffix":    strings.HasSuffix,
		"index":        strings.Index,
		"indexAny":     strings.IndexAny,
		"join":         strings.Join,
		"lastIndex":    strings.LastIndex,
		"lastIndexAny": strings.LastIndexAny,
		"repeat":       strings.Repeat,
		"replace":      strings.Replace,
		"split":        strings.Split,
		"splitAfter":   strings.SplitAfter,
		"splitAfterN":  strings.SplitAfterN,
		"splitN":       strings.SplitN,
		"title":        strings.Title,
		"toLower":      strings.ToLower,
		"toTitle":      strings.ToTitle,
		"toUpper":      strings.ToUpper,
		"trim":         strings.Trim,
		"trimLeft":     strings.TrimLeft,
		"trimPrefix":   strings.TrimPrefix,
		"trimRight":    strings.TrimRight,
		"trimSpace":    strings.TrimSpace,
		"trimSuffix":   strings.TrimSuffix,
	}
	for k, f := range funcs {
		funcMap[k] = f
	}

	tmpl, err := template.New(name).Funcs(funcMap).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("Error parsing template %s, error: %s", name, err)
	}

	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("Error executing template %s, error: %s", name, err)
	}

	return &buf, nil
}

// seq produces a sequence slice of a given length. See README.md for more info.
func seq(args ...interface{}) ([]int, error) {
	l := len(args)
	if l == 0 || l > 3 {
		return nil, fmt.Errorf("seq helper expects from 1 to 3 arguments, %d given", l)
	}
	intArgs := make([]int, l)
	for i, v := range args {
		n, err := interfaceToInt(v)
		if err != nil {
			return nil, err
		}
		intArgs[i] = n
	}
	return doSeq(intArgs[0], intArgs[1:]...)
}

func doSeq(n int, args ...int) ([]int, error) {
	var (
		from, to, step int

		i = 0
	)

	switch len(args) {
	// {{ seq To }}
	case 0:
		// {{ seq 0 }}
		if n == 0 {
			return []int{}, nil
		}
		if n > 0 {
			// {{ seq 15 }}
			from, to, step = 1, n, 1
		} else {
			// {{ seq -15 }}
			from, to, step = -1, n, 1
		}
	// {{ seq From To }}
	case 1:
		from, to, step = n, args[0], 1

	// {{ seq From To Step }}
	case 2:
		from, to, step = n, args[0], args[1]
	}

	if step <= 0 {
		return nil, fmt.Errorf("step should be a positive integer, `%#v` given", step)
	}

	// reverse order
	if from > to {
		res := make([]int, ((from-to)/step)+1)
		for k := from; k >= to; k = k - step {
			res[i] = k
			i++
		}
		return res, nil
	}

	// straight order
	res := make([]int, ((to-from)/step)+1)
	for k := from; k <= to; k = k + step {
		res[i] = k
		i++
	}
	return res, nil
}

func dump(v interface{}) string {
	return fmt.Sprintf("% #v", pretty.Formatter(v))
}

func assertFn(v interface{}) (string, error) {
	t, _ := isTrue(reflect.ValueOf(v))
	if t {
		return "", nil
	}
	return "", fmt.Errorf("Assertion failed")
}

func jsonFn(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func yamlFn(v interface{}) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func interfaceToInt(v interface{}) (int, error) {
	switch v.(type) {
	case int:
		return v.(int), nil
	case string:
		n, err := strconv.ParseInt(v.(string), 10, 64)
		if err != nil {
			return 0, err
		}
		return (int)(n), nil
	default:
		return 0, fmt.Errorf("Cannot receive %#v, int or string is expected", v)
	}
}

// isTrue reports whether the value is 'true', in the sense of not the zero of its type,
// and whether the value has a meaningful truth value.
//
// NOTE: Borrowed from Go sources: http://golang.org/src/text/template/exec.go
// Copyright (c) 2012 The Go Authors. All rights reserved.
func isTrue(val reflect.Value) (truth, ok bool) {
	if !val.IsValid() {
		// Something like var x interface{}, never set. It's a form of nil.
		return false, true
	}
	switch val.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		truth = val.Len() > 0
	case reflect.Bool:
		truth = val.Bool()
	case reflect.Complex64, reflect.Complex128:
		truth = val.Complex() != 0
	case reflect.Chan, reflect.Func, reflect.Ptr, reflect.Interface:
		truth = !val.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		truth = val.Int() != 0
	case reflect.Float32, reflect.Float64:
		truth = val.Float() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		truth = val.Uint() != 0
	case reflect.Struct:
		truth = true // Struct values are always true.
	default:
		return
	}
	return truth, true
}
