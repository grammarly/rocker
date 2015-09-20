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

package build2

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
)

type formatter struct {
	containerID string
	level       log.Level
	delegate    log.Formatter
}

// NewContainerFormatter returns an object that is given to logrus to better format
// contaienr output
func NewContainerFormatter(containerID string, level log.Level) log.Formatter {
	return &formatter{
		containerID: containerID,
		level:       level,
		delegate:    log.StandardLogger().Formatter,
	}
}

// Format formats a message from container
func (f *formatter) Format(entry *log.Entry) ([]byte, error) {
	e := entry.WithFields(log.Fields{
		"container": fmt.Sprintf("%.12s", f.containerID),
	})
	e.Message = entry.Message
	e.Level = f.level
	return f.delegate.Format(e)
}
