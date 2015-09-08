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
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"text/template"
)

// ProcessConfigTemplate renders config through the template processor.
// vars and additional functions are acceptable.
func ProcessConfigTemplate(name string, reader io.Reader, vars map[string]interface{}, funcs map[string]interface{}) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	// read template
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("Error reading template %s, error: %s", name, err)
	}
	funcMap := map[string]interface{}{
		"seq":     seq,
		"replace": replace,
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

// strings replace helper
func replace(str, repl, symbol string) string {
	return strings.Replace(str, repl, symbol, -1)
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
