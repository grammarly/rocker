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
	"encoding/json"
	"fmt"
	"log"
	"rocker/imagename"
	"time"

	"github.com/fatih/color"
)

// RockerImageData provides metadata for images built with Rocker
// It can be attached to a container label called "rocker-data" if
// --meta flag was given to `rocker build`
type RockerImageData struct {
	ImageName  *imagename.ImageName
	Rockerfile string
	Vars       Vars
	Properties Vars
	Created    time.Time
}

// PrettyString returns RockerImageData as a printable string
func (data *RockerImageData) PrettyString() string {
	prettyVars, err := json.MarshalIndent(data.Vars, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	prettyProps, err := json.MarshalIndent(data.Properties, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	green := color.New(color.FgGreen).SprintfFunc()
	yellow := color.New(color.FgYellow).SprintfFunc()
	sep := "=======================================================\n"

	res := fmt.Sprintf("%s%s\n", green(sep),
		green("Image: %s", data.ImageName.String()))

	if !data.Created.IsZero() {
		res = fmt.Sprintf("%sCreated: %s\n", res, data.Created.Format(time.RFC850))
	}

	if data.Properties != nil {
		res = fmt.Sprintf("%sProperties: %s\n", res, prettyProps)
	}

	if data.Vars != nil {
		res = fmt.Sprintf("%sVars: %s\n", res, prettyVars)
	}

	if data.Rockerfile != "" {
		res = fmt.Sprintf("%s%s\n%s\n%s\n%s", res, yellow("Rockerfile:"), yellow(sep), data.Rockerfile, yellow(sep))
	}

	return res
}
