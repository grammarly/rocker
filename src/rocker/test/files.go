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

package test

import (
	"io/ioutil"
	"os"
	"path"
)

// MakeFiles make files in a given directory
func MakeFiles(baseDir string, files map[string]string) (err error) {
	for name, content := range files {
		fullName := path.Join(baseDir, name)
		dirName := path.Dir(fullName)
		err = os.MkdirAll(dirName, 0755)
		if err != nil {
			return
		}
		err = ioutil.WriteFile(fullName, []byte(content), 0644)
		if err != nil {
			return
		}
	}
	return
}
