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
	"fmt"
	log "github.com/Sirupsen/logrus"
	"runtime"
)

type containerFormatter struct {
	isColored bool
}

var isTerminal = log.IsTerminal()

// NewColoredContainerFormatter Returns Formatter that makes your messages outputed without any log-related data.
// Also, this formatter will make all messages colored (RED for now).
// Usefull when you want all stderr messages from container to become more noticible
func NewColoredContainerFormatter() log.Formatter {
	return &containerFormatter{
		isColored: true,
	}
}

// NewMonochromeContainerFormatter Returns Formatter that makes your messages outputed without any log-related data.
func NewMonochromeContainerFormatter() log.Formatter {
	return &containerFormatter{
		isColored: false,
	}
}

func (f *containerFormatter) Format(entry *log.Entry) ([]byte, error) {
	buffer := &bytes.Buffer{}

	isColorTerminal := isTerminal && (runtime.GOOS != "windows")
	isColored := isColorTerminal && f.isColored

	if isColored {
		fmt.Fprintf(buffer, "\x1b[31m%s\x1b[0m", entry.Message)
	} else {
		fmt.Fprintf(buffer, "%s", entry.Message)
	}

	buffer.WriteByte('\n')
	return buffer.Bytes(), nil
}
