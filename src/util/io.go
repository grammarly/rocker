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
	"bufio"
	"fmt"
	"io"
)

// PrefixPipe creates an io wrapper that will add [prefix] to every line written
func PrefixPipe(prefix string, writer io.Writer) io.Writer {
	reader, proxy := io.Pipe()

	go func(prefix string, reader io.Reader, writer io.Writer) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			writer.Write([]byte(prefix + scanner.Text() + "\n"))
		}
		if scannererr := scanner.Err(); scannererr != nil {
			fmt.Fprint(writer, scannererr)
		}
	}(prefix, reader, writer)

	return proxy
}
