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
	"encoding/base64"
	"fmt"
	"github.com/grammarly/rocker/src/imagename"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/fsouza/go-dockerclient"
)

type ecrAuthCache struct {
	tokens map[string]docker.AuthConfiguration
	mu     sync.Mutex
}

var (
	_ecrAuthCache = ecrAuthCache{
		tokens: map[string]docker.AuthConfiguration{},
	}
)

// GetAuthForRegistry extracts desired docker.AuthConfiguration object from the
// list of docker.AuthConfigurations by registry hostname
func GetAuthForRegistry(auth *docker.AuthConfigurations, image *imagename.ImageName) (result docker.AuthConfiguration, err error) {

	registry := image.Registry

	// The default registry is "index.docker.io"
	if registry == "" || registry == "registry-1.docker.io" {
		registry = "index.docker.io"
	}
	// Optionally override auth took via aws-sdk (through ENV vars)
	if image.IsECR() {
		if awsRegAuth, err := GetECRAuth(registry); err != nil && err != credentials.ErrNoValidProvidersFoundInChain {
			return result, err
		} else if awsRegAuth.Username != "" {
			return awsRegAuth, nil
		}
	}

	if auth == nil {
		return
	}

	if result, ok := auth.Configs[registry]; ok {
		return result, nil
	}
	if result, ok := auth.Configs["https://"+registry]; ok {
		return result, nil
	}
	if result, ok := auth.Configs["https://"+registry+"/v1/"]; ok {
		return result, nil
	}
	// not sure /v2/ is needed, but just in case
	if result, ok := auth.Configs["https://"+registry+"/v2/"]; ok {
		return result, nil
	}
	if result, ok := auth.Configs["*"]; ok {
		return result, nil
	}
	return
}

// GetECRAuth requests AWS ECR API to get docker.AuthConfiguration token
func GetECRAuth(registry string) (result docker.AuthConfiguration, err error) {
	_ecrAuthCache.mu.Lock()
	defer _ecrAuthCache.mu.Unlock()

	if token, ok := _ecrAuthCache.tokens[registry]; ok {
		return token, nil
	}

	defer func() {
		_ecrAuthCache.tokens[registry] = result
	}()

	// TODO: take region from the registry hostname?
	cfg := &aws.Config{
		Region: aws.String("us-east-1"),
	}

	if log.StandardLogger().Level >= log.DebugLevel {
		cfg.LogLevel = aws.LogLevel(aws.LogDebugWithRequestErrors)
	}

	split := strings.Split(registry, ".")

	svc := ecr.New(session.New(), cfg)
	params := &ecr.GetAuthorizationTokenInput{
		RegistryIds: []*string{aws.String(split[0])},
	}

	res, err := svc.GetAuthorizationToken(params)
	if err != nil {
		return result, err
	}

	if len(res.AuthorizationData) == 0 {
		return result, nil
	}

	data, err := base64.StdEncoding.DecodeString(*res.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return result, err
	}

	userpass := strings.Split(string(data), ":")
	if len(userpass) != 2 {
		return result, fmt.Errorf("Cannot parse token got from ECR: %s", string(data))
	}

	result = docker.AuthConfiguration{
		Username:      userpass[0],
		Password:      userpass[1],
		ServerAddress: *res.AuthorizationData[0].ProxyEndpoint,
	}

	return
}
