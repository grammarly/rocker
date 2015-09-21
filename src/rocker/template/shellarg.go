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
	"regexp"
	"strings"
)

var (
	complexShellArgRegex     = regexp.MustCompile("(?i:[^a-z\\d_\\/:=-])")
	leadingSingleQuotesRegex = regexp.MustCompile("^(?:'')+")
)

// EscapeShellarg escapes any string so it can be safely passed to a shell
func EscapeShellarg(value string) string {
	// Nothing to escape, return as is
	if !complexShellArgRegex.MatchString(value) {
		return value
	}

	// escape all single quotes
	value = "'" + strings.Replace(value, "'", "'\\''", -1) + "'"

	// remove duplicated single quotes at the beginning
	value = leadingSingleQuotesRegex.ReplaceAllString(value, "")

	// remove non-escaped single-quote if there are enclosed between 2 escaped
	value = strings.Replace(value, "\\'''", "\\'", -1)

	// if the string contains new lines, then use bash $'string' representation
	// to have the newline escape character
	if strings.Contains(value, "\n") {
		value = "$" + strings.Replace(value, "\n", "\\n", -1)
	}

	return value
}
