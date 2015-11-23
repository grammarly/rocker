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

package imagename

import (
	"fmt"
	"strings"

	"time"
)

// Artifact represents the artifact that is the result of image build
// It holds information about the pushed image and may be saved as a file
type Artifact struct {
	Name        *ImageName `yaml:"Name"`
	Pushed      bool       `yaml:"Pushed"`
	Tag         string     `yaml:"Tag"`
	Digest      string     `yaml:"Digest"`
	ImageID     string     `yaml:"ImageID"`
	Addressable string     `yaml:"Addressable"`
	BuildTime   time.Time  `yaml:"BuildTime"`
}

// Artifacts is a collection of Artifact entities
type Artifacts struct {
	RockerArtifacts []Artifact `yaml:"RockerArtifacts"`
}

// GetFileName constructs the base file name out of the image info
func (a *Artifact) GetFileName() string {
	imageName := strings.Replace(a.Name.Name, "/", "_", -1)
	return fmt.Sprintf("%s_%s.yml", imageName, a.Name.GetTag())
}

// Len returns the length of image tags
func (a *Artifacts) Len() int {
	return len(a.RockerArtifacts)
}

// Less returns true if item by index[i] is created after of item[j]
func (a *Artifacts) Less(i, j int) bool {
	return a.RockerArtifacts[i].Name.Tag > a.RockerArtifacts[j].Name.Tag
}

// Swap swaps items by indices [i] and [j]
func (a *Artifacts) Swap(i, j int) {
	a.RockerArtifacts[i], a.RockerArtifacts[j] = a.RockerArtifacts[j], a.RockerArtifacts[i]
}
