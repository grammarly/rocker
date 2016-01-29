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
	"bufio"
	"io"
	"testing"
)

// Writer creates wrapper and returns io.Writer that will prepend [prefix] to every line written
// and write to *testing.T.Log()
func Writer(prefix string, t *testing.T) io.Writer {
	reader, writer := io.Pipe()

	go func(t *testing.T, reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			t.Logf("%s%s", prefix, scanner.Text())
		}
		if scannererr := scanner.Err(); scannererr != nil {
			t.Error(scannererr)
		}
	}(t, reader)

	return writer
}
