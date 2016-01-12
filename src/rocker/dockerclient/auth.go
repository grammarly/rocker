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

import "github.com/fsouza/go-dockerclient"

// GetAuthForRegistry extracts desired docker.AuthConfiguration object from the
// list of docker.AuthConfigurations by registry hostname
func GetAuthForRegistry(auth *docker.AuthConfigurations, registry string) (result docker.AuthConfiguration) {
	if auth == nil {
		return
	}
	// The default registry is "index.docker.io"
	if registry == "" {
		registry = "index.docker.io"
	}
	if result, ok := auth.Configs[registry]; ok {
		return result
	}
	if result, ok := auth.Configs["https://"+registry]; ok {
		return result
	}
	if result, ok := auth.Configs["https://"+registry+"/v1/"]; ok {
		return result
	}
	// not sure /v2/ is needed, but just in case
	if result, ok := auth.Configs["https://"+registry+"/v2/"]; ok {
		return result
	}
	if result, ok := auth.Configs["*"]; ok {
		return result
	}
	return
}
