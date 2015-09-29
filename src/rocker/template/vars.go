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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-yaml/yaml"
)

// Vars describes the data structure of the build variables
type Vars map[string]interface{}

// Merge the current Vars structure with the list of other Vars structs
func (vars Vars) Merge(varsList ...Vars) Vars {
	for _, mergeWith := range varsList {
		for k, v := range mergeWith {
			vars[k] = v
		}
	}
	return vars
}

// IsSet returns true if the given key is set
func (vars Vars) IsSet(key string) bool {
	_, ok := vars[key]
	return ok
}

// ToStrings converts Vars to a slice of strings line []string{"KEY=VALUE"}
func (vars Vars) ToStrings() (result []string) {
	for k, v := range vars {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(result)
	return result
}

// ToMapOfInterface casts Vars to map[string]interface{}
func (vars Vars) ToMapOfInterface() map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range vars {
		result[k] = v
	}
	return result
}

// MarshalJSON serialize Vars to JSON
func (vars Vars) MarshalJSON() ([]byte, error) {
	return json.Marshal(vars.ToStrings())
}

// UnmarshalJSON unserialize Vars from JSON string
func (vars *Vars) UnmarshalJSON(data []byte) (err error) {
	// try unmarshal map to keep backward compatibility
	maps := map[string]interface{}{}
	if err = json.Unmarshal(data, &maps); err == nil {
		*vars = (Vars)(maps)
		return nil
	}
	// unmarshal slice of strings
	strings := []string{}
	if err = json.Unmarshal(data, &strings); err != nil {
		return err
	}
	if *vars, err = VarsFromStrings(strings); err != nil {
		return err
	}
	return nil
}

// VarsFromStrings parses Vars through ParseKvPairs and then loads content from files
// for vars values with "@" prefix
func VarsFromStrings(pairs []string) (vars Vars, err error) {
	vars = ParseKvPairs(pairs)
	for k, v := range vars {
		// We care only about strings
		switch v := v.(type) {
		case string:
			// Read variable content from a file if "@" prefix is given
			if strings.HasPrefix(v, "@") {
				f := v[1:]
				if vars[k], err = loadFileContent(f); err != nil {
					return vars, fmt.Errorf("Failed to read file '%s' for variable %s, error: %s", f, k, err)
				}
			}
			// Unescape "\@"
			if strings.HasPrefix(v, "\\@") {
				vars[k] = v[1:]
			}
		}
	}
	return vars, nil
}

// VarsFromFile reads variables from either JSON or YAML file
func VarsFromFile(filename string) (vars Vars, err error) {

	if filename, err = resolveFileName(filename); err != nil {
		return nil, err
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	vars = Vars{}

	switch filepath.Ext(filename) {
	case ".yaml", ".yml", ".":
		if err := yaml.Unmarshal(data, &vars); err != nil {
			return nil, err
		}
	case ".json":
		if err := json.Unmarshal(data, &vars); err != nil {
			return nil, err
		}
	}

	return vars, nil
}

// VarsFromFileMulti reads multiple files and merge vars
func VarsFromFileMulti(files []string) (vars Vars, err error) {
	varsList := make([]Vars, len(files))
	for i, f := range files {
		if varsList[i], err = VarsFromFile(f); err != nil {
			return nil, err
		}
	}
	return Vars{}.Merge(varsList...), nil
}

// ParseKvPairs parses Vars from a slice of strings e.g. []string{"KEY=VALUE"}
func ParseKvPairs(pairs []string) (vars Vars) {
	vars = make(Vars)
	for _, varPair := range pairs {
		tmp := strings.SplitN(varPair, "=", 2)
		vars[tmp[0]] = tmp[1]
	}
	return vars
}

func loadFileContent(f string) (content string, err error) {
	if f, err = resolveFileName(f); err != nil {
		return "", err
	}
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func resolveFileName(f string) (string, error) {
	if f == "~" || strings.HasPrefix(f, "~/") {
		f = strings.Replace(f, "~", os.Getenv("HOME"), 1)
	}
	if !filepath.IsAbs(f) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		f = path.Join(wd, f)
	}
	return f, nil
}

// Code borrowed from https://github.com/docker/docker/blob/df0e0c76831bed08cf5e08ac9a1abebf6739da23/builder/support.go
var (
	// `\\\\+|[^\\]|\b|\A` - match any number of "\\" (ie, properly-escaped backslashes), or a single non-backslash character, or a word boundary, or beginning-of-line
	// `\$` - match literal $
	// `[[:alnum:]_]+` - match things like `$SOME_VAR`
	// `{[[:alnum:]_]+}` - match things like `${SOME_VAR}`
	tokenVarsInterpolation = regexp.MustCompile(`(\\|\\\\+|[^\\]|\b|\A)\$([[:alnum:]_]+|{[[:alnum:]_]+})`)
	// this intentionally punts on more exotic interpolations like ${SOME_VAR%suffix} and lets the shell handle those directly
)

// ReplaceString handle vars replacement
func (vars Vars) ReplaceString(str string) string {
	for _, match := range tokenVarsInterpolation.FindAllString(str, -1) {
		idx := strings.Index(match, "\\$")
		if idx != -1 {
			if idx+2 >= len(match) {
				str = strings.Replace(str, match, "\\$", -1)
				continue
			}

			prefix := match[:idx]
			stripped := match[idx+2:]
			str = strings.Replace(str, match, prefix+"$"+stripped, -1)
			continue
		}

		match = match[strings.Index(match, "$"):]
		matchKey := strings.Trim(match, "${}")

		if val, ok := vars[matchKey].(string); ok {
			str = strings.Replace(str, match, val, -1)
		}
	}

	return str
}
