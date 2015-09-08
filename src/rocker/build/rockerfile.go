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
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"rocker/parser"
)

// Parse parses a Rockerfile from an io.Reader and returns AST data structure
func Parse(rockerfileContent io.Reader) (*parser.Node, error) {
	node, err := parser.Parse(rockerfileContent)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// RockerfileAstToString returns printable AST of the node
func RockerfileAstToString(node *parser.Node) (str string, err error) {
	str += node.Value

	isKeyVal := node.Value == "env" || node.Value == "label"

	if len(node.Flags) > 0 {
		str += " " + strings.Join(node.Flags, " ")
	}

	if node.Attributes["json"] {
		args := []string{}
		for n := node.Next; n != nil; n = n.Next {
			args = append(args, n.Value)
		}
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(args); err != nil {
			return str, err
		}
		str += " " + strings.TrimSpace(buf.String())
		return str, nil
	}

	for _, n := range node.Children {
		children, err := RockerfileAstToString(n)
		if err != nil {
			return str, err
		}
		str += children + "\n"
	}

	if node.Next != nil {
		for n, i := node.Next, 0; n != nil; n, i = n.Next, i+1 {
			if len(n.Children) > 0 {
				children, err := RockerfileAstToString(n)
				if err != nil {
					return str, err
				}
				str += " " + children
			} else if isKeyVal && i%2 != 0 {
				str += "=" + n.Value
			} else {
				str += " " + n.Value
			}
		}
	}

	return strings.TrimSpace(str), nil
}

func handleJSONArgs(args []string, attributes map[string]bool) []string {
	if len(args) == 0 {
		return []string{}
	}

	if attributes != nil && attributes["json"] {
		return args
	}

	// literal string command, not an exec array
	return []string{strings.Join(args, " ")}
}

func parseFlags(flags []string) map[string]string {
	result := make(map[string]string)
	for _, flag := range flags {
		key := flag[2:]
		value := ""

		index := strings.Index(key, "=")
		if index >= 0 {
			value = key[index+1:]
			key = key[:index]
		}

		result[key] = value
	}
	return result
}
