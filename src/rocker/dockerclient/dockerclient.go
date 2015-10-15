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

// Package dockerclient provides utilities for embedding docker client
// functionality to other tools. It provides configurable docker client
// connection functions, config struct, integration with codegangsta/cli.
package dockerclient

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/fsouza/go-dockerclient"
)

var (
	// DefaultEndpoint is the default address of Docker socket
	DefaultEndpoint = "unix:///var/run/docker.sock"
)

// Config represents docker client connection parameters
type Config struct {
	Host      string
	Tlsverify bool
	Tlscacert string
	Tlscert   string
	Tlskey    string
}

// NewConfig returns new config with resolved options from current ENV
func NewConfig() *Config {
	certPath := os.Getenv("DOCKER_CERT_PATH")
	if certPath == "" {
		usr, _ := user.Current()
		certPath = filepath.Join(usr.HomeDir, ".docker")
	}
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = DefaultEndpoint
	}
	// why NewConfigFromCli default value is not working
	return &Config{
		Host:      host,
		Tlsverify: os.Getenv("DOCKER_TLS_VERIFY") == "1" || os.Getenv("DOCKER_TLS_VERIFY") == "yes",
		Tlscacert: certPath + "/ca.pem",
		Tlscert:   certPath + "/cert.pem",
		Tlskey:    certPath + "/key.pem",
	}
}

// NewConfigFromCli returns new config with NewConfig overridden cli options
func NewConfigFromCli(c *cli.Context) *Config {
	config := NewConfig()
	config.Host = globalCliString(c, "host")
	if c.GlobalIsSet("tlsverify") {
		config.Tlsverify = c.GlobalBool("tlsverify")
		config.Tlscacert = globalCliString(c, "tlscacert")
		config.Tlscert = globalCliString(c, "tlscert")
		config.Tlskey = globalCliString(c, "tlskey")
	}
	return config
}

// New returns a new docker client connection with default config
func New() (*docker.Client, error) {
	return NewFromConfig(NewConfig())
}

// NewFromConfig returns a new docker client connection with given config
func NewFromConfig(config *Config) (*docker.Client, error) {
	if config.Tlsverify {
		return docker.NewTLSClient(config.Host, config.Tlscert, config.Tlskey, config.Tlscacert)
	}
	return docker.NewClient(config.Host)
}

// NewFromCli returns a new docker client connection with config built from cli params
func NewFromCli(c *cli.Context) (*docker.Client, error) {
	return NewFromConfig(NewConfigFromCli(c))
}

// GlobalCliParams returns global params that configures docker client connection
func GlobalCliParams() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:   "host, H",
			Value:  DefaultEndpoint,
			Usage:  "Daemon socket(s) to connect to",
			EnvVar: "DOCKER_HOST",
		},
		cli.BoolFlag{
			Name:  "tlsverify, tls",
			Usage: "Use TLS and verify the remote",
		},
		cli.StringFlag{
			Name:  "tlscacert",
			Value: "~/.docker/ca.pem",
			Usage: "Trust certs signed only by this CA",
		},
		cli.StringFlag{
			Name:  "tlscert",
			Value: "~/.docker/cert.pem",
			Usage: "Path to TLS certificate file",
		},
		cli.StringFlag{
			Name:  "tlskey",
			Value: "~/.docker/key.pem",
			Usage: "Path to TLS key file",
		},
	}
}

// InfoCommandSpec returns specifications of the info comment for codegangsta/cli
func InfoCommandSpec() cli.Command {
	return cli.Command{
		Name:   "info",
		Usage:  "show docker info (check connectivity, versions, etc.)",
		Action: infoCommand,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "all, a",
				Usage: "show advanced info",
			},
		},
	}
}

// infoCommand implements 'info' command that prints docker info (check connectivity, versions, etc.)
func infoCommand(c *cli.Context) {
	config := NewConfigFromCli(c)

	fmt.Printf("Docker host: %s\n", config.Host)
	fmt.Printf("Docker use TLS: %s\n", strconv.FormatBool(config.Tlsverify))
	if config.Tlsverify {
		fmt.Printf("  TLS CA cert: %s\n", config.Tlscacert)
		fmt.Printf("  TLS cert: %s\n", config.Tlscert)
		fmt.Printf("  TLS key: %s\n", config.Tlskey)
	}

	dockerClient, err := NewFromCli(c)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: golang randomizes maps every time, so the output is not consistent
	//       find out a way to sort it correctly

	version, err := dockerClient.Version()
	if err != nil {
		log.Fatal(err)
	}

	for _, kv := range *version {
		parts := strings.SplitN(kv, "=", 2)
		fmt.Printf("Docker %s: %s\n", parts[0], parts[1])
	}

	if c.Bool("all") {
		info, err := dockerClient.Info()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("\nDocker advanced info:\n")
		for key, val := range info.Map() {
			fmt.Printf("%s: %s\n", key, val)
		}
	}
}

// globalCliString fixes string arguments enclosed with double quotes
// 'docker-machine config' gives such arguments
func globalCliString(c *cli.Context, name string) string {
	str := c.GlobalString(name)
	if len(str) >= 2 && str[0] == '\u0022' && str[len(str)-1] == '\u0022' {
		str = str[1 : len(str)-1]
	}
	return str
}
