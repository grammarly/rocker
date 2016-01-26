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

package dockerclient

import (
	"encoding/json"
	"fmt"
	"github.com/grammarly/rocker/src/imagename"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/fsouza/go-dockerclient"

	log "github.com/Sirupsen/logrus"
)

type tags struct {
	Name string   `json:"name,omitempty"`
	Tags []string `json:"tags,omitempty"`
}

type bearer struct {
	Realm   string
	Service string
	Scope   string
}

// RegistryListTags returns the list of images instances obtained from all tags existing in the registry
func RegistryListTags(image *imagename.ImageName, auth *docker.AuthConfigurations) (images []*imagename.ImageName, err error) {
	var (
		name     = image.Name
		registry = image.Registry
	)

	regAuth, err := GetAuthForRegistry(auth, image)
	if err != nil {
		return nil, fmt.Errorf("Failed to get auth token for registry: %s, make sure you are properly logged in using `docker login` or have AWS credentials set in case of using ECR", image)
	}

	// XXX: AWS ECR Registry API v2 does not support listing tags
	// wo we just return a single image tag if it exists and no wildcards used
	if image.IsECR() {
		log.Debugf("ECR detected %s", registry)
		if !image.IsStrict() {
			return nil, fmt.Errorf("Amazon ECR does not support tags listing, therefore image wildcards are not supported, sorry: %s", image)
		}
		if exists, err := ecrImageExists(image, regAuth); err != nil {
			return nil, err
		} else if exists {
			log.Debugf("ECR image %s found in the registry", image)
			images = append(images, image)
		}
		return
	}

	if registry == "" {
		registry = "registry-1.docker.io"
		if !strings.Contains(name, "/") {
			name = "library/" + name
		}
	}

	var (
		tg  = tags{}
		url = fmt.Sprintf("https://%s/v2/%s/tags/list?page_size=9999&page=1", registry, name)
	)

	log.Debugf("Listing image tags from the remote registry %s", url)

	if err := registryGet(url, regAuth, &tg); err != nil {
		return nil, err
	}

	log.Debugf("Got %d tags from the remote registry for image %s", len(tg.Tags), image)

	for _, t := range tg.Tags {
		candidate := imagename.New(image.NameWithRegistry(), t)
		if image.Contains(candidate) || image.Tag == candidate.Tag {
			images = append(images, candidate)
		}
	}

	return
}

// registryGet executes HTTP get to a given registry
func registryGet(uri string, auth docker.AuthConfiguration, obj interface{}) (err error) {
	var (
		client = &http.Client{}
		req    *http.Request
		res    *http.Response
		body   []byte
	)

	if req, err = http.NewRequest("GET", uri, nil); err != nil {
		return
	}

	var (
		b       *bearer
		authTry bool
	)

	for {
		if res, err = client.Do(req); err != nil {
			return fmt.Errorf("Request to %s failed with %s\n", uri, err)
		}

		b = parseBearer(res.Header.Get("Www-Authenticate"))
		log.Debugf("Got HTTP %d for %s; tried auth: %t; has Bearer: %t, auth username: %q", res.StatusCode, uri, authTry, b != nil, auth.Username)

		if res.StatusCode == 401 && !authTry && b != nil {
			token, err := getAuthToken(b, auth)
			if err != nil {
				return fmt.Errorf("Failed to authenticate to registry %s, error: %s", uri, err)
			}

			req.Header.Add("Authorization", "Bearer "+token)

			authTry = true
			continue
		}

		break
	}

	if res.StatusCode != 200 {
		// TODO: maybe more descriptive error
		return fmt.Errorf("GET %s status code %d", uri, res.StatusCode)
	}

	if body, err = ioutil.ReadAll(res.Body); err != nil {
		return fmt.Errorf("Response from %s cannot be read due to error %s\n", uri, err)
	}

	if err = json.Unmarshal(body, obj); err != nil {
		return fmt.Errorf("Response from %s cannot be unmarshalled due to error %s, response: %s\n",
			uri, err, string(body))
	}

	return
}

func getAuthToken(b *bearer, auth docker.AuthConfiguration) (token string, err error) {
	type authRespType struct {
		Token string
	}

	var (
		req  *http.Request
		res  *http.Response
		body []byte

		client   = &http.Client{}
		authResp = &authRespType{}
	)

	uri, err := url.Parse(b.Realm)
	if err != nil {
		return "", fmt.Errorf("Failed to parse real url %s, error %s", b.Realm, err)
	}

	// Add query params to the ream uri
	q := uri.Query()
	q.Set("service", b.Service)
	q.Set("scope", b.Scope)
	uri.RawQuery = q.Encode()

	if req, err = http.NewRequest("GET", uri.String(), nil); err != nil {
		return "", err
	}

	if auth.Username != "" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	log.Debugf("Getting auth token from %s", uri)

	if res, err = client.Do(req); err != nil {
		return "", fmt.Errorf("Failed to authenticate by realm url %s, error %s", uri, err)
	}

	if res.StatusCode != 200 {
		// TODO: maybe more descriptive error
		return "", fmt.Errorf("GET %s status code %d", uri, res.StatusCode)
	}

	if body, err = ioutil.ReadAll(res.Body); err != nil {
		return "", fmt.Errorf("Response from %s cannot be read due to error %s\n", uri, err)
	}

	if err := json.Unmarshal(body, authResp); err != nil {
		return "", fmt.Errorf("Response from %s cannot be unmarshalled due to error %s, response: %s\n",
			uri, err, string(body))
	}

	return authResp.Token, nil
}

func ecrImageExists(image *imagename.ImageName, auth docker.AuthConfiguration) (exists bool, err error) {
	var (
		req    *http.Request
		res    *http.Response
		client = &http.Client{}
	)

	uri := fmt.Sprintf("https://%s/v2/%s/manifests/%s", image.Registry, image.Name, image.Tag)

	if req, err = http.NewRequest("GET", uri, nil); err != nil {
		return false, err
	}

	req.SetBasicAuth(auth.Username, auth.Password)

	log.Debugf("Request ECR image %s with basic auth %s:****", uri, auth.Username)

	if res, err = client.Do(req); err != nil {
		return false, fmt.Errorf("Failed to authenticate by realm url %s, error %s", uri, err)
	}

	log.Debugf("Got status %d", res.StatusCode)

	if res.StatusCode == 404 {
		return false, nil
	}

	if res.StatusCode != 200 {
		// TODO: maybe more descriptive error
		return false, fmt.Errorf("GET %s status code %d", uri, res.StatusCode)
	}

	return true, nil
}

func parseBearer(hdr string) *bearer {
	if !strings.HasPrefix(hdr, "Bearer ") {
		return nil
	}

	b := &bearer{}
	hdr = strings.TrimPrefix(hdr, "Bearer ")

	// e.g.
	// Www-Authenticate: Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:me/alpine:pull"
	for _, pair := range strings.Split(hdr, ",") {
		kv := strings.SplitN(pair, "=", 2)
		key, value := kv[0], strings.Trim(kv[1], "\"")

		switch key {
		case "realm":
			b.Realm = value
		case "service":
			b.Service = value
		case "scope":
			b.Scope = value
		}
	}

	return b
}
