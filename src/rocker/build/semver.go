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

// Light semver implementation, we cannot use 'semver' package because
// it does not export 'version' property that we need here.

package build

import (
	"fmt"
	"regexp"
	"strconv"
)

var semverRegexp = regexp.MustCompile(`^\bv?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[\da-z\-]+(?:\.[\da-z\-]+)*)?(?:\+[\da-z\-]+(?:\.[\da-z\-]+)*)?\b$`)

// Semver represents a light version of 'semver' data structure
type Semver struct {
	Major int
	Minor int
	Patch int
}

// NewSemver parses a semver string into the Semver struct
func NewSemver(str string) (semver *Semver, err error) {
	matches := semverRegexp.FindAllStringSubmatch(str, -1)
	if matches == nil {
		return nil, fmt.Errorf("Failed to parse given version as semver: %s", str)
	}

	semver = &Semver{}

	if semver.Major, err = strconv.Atoi(matches[0][1]); err != nil {
		return nil, err
	}
	if semver.Minor, err = strconv.Atoi(matches[0][2]); err != nil {
		return nil, err
	}
	if semver.Patch, err = strconv.Atoi(matches[0][3]); err != nil {
		return nil, err
	}

	return semver, nil
}
