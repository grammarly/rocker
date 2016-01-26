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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeShellarg_Basic(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Testing", EscapeShellarg("Testing"))
	assert.Equal(t, "'Testing;'", EscapeShellarg("Testing;"))
}

func TestEscapeShellarg_Advanced(t *testing.T) {
	t.Parallel()

	assertions := map[string]string{
		"hello\\nworld":   "'hello\\nworld'",
		"hello:world":     "hello:world",
		"--hello=world":   "--hello=world",
		"hello\\tworld":   "'hello\\tworld'",
		"hello\nworld":    "$'hello\\nworld'",
		"\thello\nworld'": "$'\thello\\nworld'\\'",
		"hello  world":    "'hello  world'",
		"hello\\\\'":      "'hello\\\\'\\'",
		"'\\\\'world":     "\\''\\\\'\\''world'",
		"world\\":         "'world\\'",
		"'single'":        "\\''single'\\'",
	}

	for k, v := range assertions {
		assert.Equal(t, v, EscapeShellarg(k))
	}
}
