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

package s3

import (
	log "github.com/Sirupsen/logrus"
)

// Logger is an aws logger that prints entries to Logrus
type Logger struct{}

// Log writes to logrus.Info
func (l *Logger) Log(args ...interface{}) {
	if len(args) == 1 {
		log.Infof(args[0].(string))
	} else {
		log.Infof(args[0].(string), args[1:]...)
	}
}
