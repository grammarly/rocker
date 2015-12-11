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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

type tags struct {
	Name string   `json:"name,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

type history struct {
	Compatibility string `json:"v1Compatibility,omitempty"`
}

type manifests struct {
	Name          string     `json:"name,omitempty"`
	Tag           string     `json:"tag,omitempty"`
	Architecture  string     `json:"architecture,omitempty"`
	History       []*history `json:"history,omitempty"`
	SchemaVersion int        `json:"schemaVersion,omitempty"`
}

type registryTags struct {
	Count    int            `json:"count,omitempty"`
	Next     string         `json:"next,omitempty"`
	Previous string         `json:"previous,omitempty"`
	Results  []*registryTag `json:"results,omitempty"`
}

type registryTag struct {
	Name        string `json:"name,omitempty"`
	FullSize    int    `json:"full_size,omitempty"`
	ID          int    `json:"id,omitempty"`
	Repository  int    `json:"repository,omitempty"`
	Creator     int    `json:"creator,omitempty"`
	LastUpdater int    `json:"last_updater,omitempty"`
	ImageID     string `json:"image_id,omitempty"`
	V2          bool   `json:"v2,omitempty"`
}

// RegistryGet returns docker.Image instance from the information stored in the registry
func RegistryGet(image *ImageName) (img *docker.Image, err error) {
	manifest := manifests{}
	img = &docker.Image{}

	// no cannot get similar info from Hub, just return stub data
	if image.Registry == "" {
		return
	}

	if err = registryGet(fmt.Sprintf("https://%s/v2/%s/manifests/%s", image.Registry, image.Name, image.Tag), &manifest); err != nil {
		return
	}

	if len(manifest.History) == 0 {
		err = fmt.Errorf("Image doesn't have expected format, history record is empty")
		return
	}

	last := manifest.History[0]
	err = json.Unmarshal([]byte(last.Compatibility), img)
	return
}

// RegistryListTags returns the list of images instances obtained from all tags existing in the registry
func RegistryListTags(image *ImageName) (images []*ImageName, err error) {
	if image.Registry != "" {
		return registryListTags(image)
	}

	return registryListTagsDockerHub(image)
}

// registryListTagsDockerHub lists image tags from hub.docker.com
func registryListTagsDockerHub(image *ImageName) (images []*ImageName, err error) {
	name := image.Name
	if !strings.Contains(name, "/") {
		name = "library/" + name
	}

	tg := registryTags{}
	if err = registryGet(fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags/?page_size=9999&page=1", name), &tg); err != nil {
		return
	}

	for _, t := range tg.Results {
		candidate := New(image.NameWithRegistry(), t.Name)
		if image.Contains(candidate) || image.Tag == candidate.Tag {
			images = append(images, candidate)
		}
	}
	return
}

// registryListTags lists image tags from a private docker registry
func registryListTags(image *ImageName) (images []*ImageName, err error) {
	tg := tags{}
	if err = registryGet(fmt.Sprintf("https://%s/v2/%s/tags/list", image.Registry, image.Name), &tg); err != nil {
		return
	}

	for _, t := range tg.Tags {
		candidate := New(image.NameWithRegistry(), t)
		if image.Contains(candidate) || image.Tag == candidate.Tag {
			images = append(images, candidate)
		}
	}
	return
}

// registryGet executes HTTP get to a given registry
func registryGet(url string, obj interface{}) (err error) {
	var res *http.Response
	var body []byte

	res, err = http.Get(url)
	if err != nil {
		err = fmt.Errorf("Request to %s failed with %s\n", url, err)
		return
	}

	if res.StatusCode == 404 {
		err = fmt.Errorf("Not found")
		return
	}

	if body, err = ioutil.ReadAll(res.Body); err != nil {
		err = fmt.Errorf("Response from %s cannot be read due to error %s\n", url, err)
		return
	}

	if err = json.Unmarshal(body, obj); err != nil {
		err = fmt.Errorf("Response from %s cannot be unmarshalled due to error %s, response: %s\n",
			url, err, string(body))
		return
	}

	return
}
