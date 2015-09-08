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

package util

import (
	"fmt"
	"path"
	"strings"
)

// ResolvePath resolves the subPath from baseDir such that the resultig path cannot
// go outside the baseDir
func ResolvePath(baseDir, subPath string) (resultPath string, err error) {
	resultPath = path.Join(baseDir, subPath)

	// path.Join cleans the path and removes trailing slash if it's not the root path
	// but we want to preserve trailing slash instead
	if subPath[len(subPath)-1:] == "/" && resultPath[len(resultPath)-1:] != "/" {
		resultPath = resultPath + "/"
	}

	if resultPath == baseDir {
		return resultPath, nil
	}

	if !strings.HasPrefix(resultPath, baseDir+"/") {
		return resultPath, fmt.Errorf("Invalid path: %s", subPath)
	}

	return resultPath, nil
}
