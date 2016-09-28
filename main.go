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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grammarly/rocker/src/build"
	"github.com/grammarly/rocker/src/debugtrap"
	"github.com/grammarly/rocker/src/dockerclient"
	"github.com/grammarly/rocker/src/storage/s3"
	"github.com/grammarly/rocker/src/template"
	"github.com/grammarly/rocker/src/textformatter"
	"github.com/grammarly/rocker/src/util"

	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/units"
	"github.com/fatih/color"
	"github.com/fsouza/go-dockerclient"

	log "github.com/Sirupsen/logrus"
)

var (
	// Version that is passed on compile time through -ldflags
	Version = "built locally"

	// GitCommit that is passed on compile time through -ldflags
	GitCommit = "none"

	// GitBranch that is passed on compile time through -ldflags
	GitBranch = "none"

	// BuildTime that is passed on compile time through -ldflags
	BuildTime = "none"

	// HumanVersion is a human readable app version
	HumanVersion = fmt.Sprintf("%s - %.7s (%s) %s", Version, GitCommit, GitBranch, BuildTime)
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	debugtrap.SetupDumpStackTrap()
}

func main() {
	app := cli.NewApp()

	app.Name = "rocker"
	app.Version = HumanVersion

	app.Usage = "Docker based build tool\n\n   Run 'rocker COMMAND --help' for more information on a command."

	app.Author = ""
	app.Email = ""
	app.Authors = []cli.Author{
		{"Yura Bogdanov", "yuriy.bogdanov@grammarly.com"},
		{"Stas Levental", "stas.levental@grammarly.com"},
		{"Roman Khlystik", "roman.khlystik@grammarly.com"},
	}

	app.Flags = append([]cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, vv, D",
			Usage: "Be verbose",
		},
		cli.BoolFlag{
			Name:  "json",
			Usage: "Print output in json",
		},
		cli.BoolTFlag{
			Name:  "colors",
			Usage: "Make output colored",
		},
		cli.BoolFlag{
			Name:   "cmd, C",
			EnvVar: "ROCKER_PRINT_COMMAND",
			Usage:  "Print command-line that was used to exec",
		},
	}, dockerclient.GlobalCliParams()...)

	buildFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "file, f",
			Value: "Rockerfile",
			Usage: "rocker build file to execute",
		},
		cli.StringFlag{
			Name:  "auth, a",
			Value: "",
			Usage: "Username and password in user:password format",
		},
		cli.StringSliceFlag{
			Name:  "build-arg",
			Value: &cli.StringSlice{},
			Usage: "Set build-time variables, can pass multiple of those, format is key=value (default [])",
		},
		cli.StringSliceFlag{
			Name:  "var",
			Value: &cli.StringSlice{},
			Usage: "set variables to pass to build tasks, value is like \"key=value\"",
		},
		cli.StringSliceFlag{
			Name:  "vars",
			Value: &cli.StringSlice{},
			Usage: "Load variables form a file, either JSON or YAML. Can pass multiple of this.",
		},
		cli.BoolFlag{
			Name:  "no-cache",
			Usage: "supresses cache for docker builds",
		},
		cli.BoolFlag{
			Name:  "reload-cache",
			Usage: "removes any cache that hit and save the new one",
		},
		cli.StringFlag{
			Name:  "cache-dir",
			Value: "~/.rocker_cache",
			Usage: "Set the directory where the cache will be stored",
		},
		cli.BoolFlag{
			Name:  "no-reuse",
			Usage: "suppresses reuse for all the volumes in the build",
		},
		cli.BoolFlag{
			Name:  "push",
			Usage: "pushes all the images marked with push to docker hub",
		},
		cli.BoolFlag{
			Name:  "pull",
			Usage: "always attempt to pull a newer version of the FROM images",
		},
		cli.BoolFlag{
			Name:  "attach",
			Usage: "attach to a container in place of ATTACH command",
		},
		cli.BoolFlag{
			Name:  "meta",
			Usage: "add metadata to the tagged images, such as user, Rockerfile source, variables and git branch/sha",
		},
		cli.BoolFlag{
			Name:  "print",
			Usage: "just print the Rockerfile after template processing and stop",
		},
		cli.BoolFlag{
			Name:  "demand-artifacts",
			Usage: "fail if artifacts not found for {{ image }} helpers",
		},
		cli.StringFlag{
			Name:  "id",
			Usage: "override the default id generation strategy for current build",
		},
		cli.StringFlag{
			Name:  "artifacts-path",
			Usage: "put artifacts (files with pushed images description) to the directory",
		},
		cli.BoolFlag{
			Name:  "no-garbage",
			Usage: "remove the images from the tail if not tagged",
		},
		cli.IntFlag{
			Name:  "push-retry",
			Usage: "number of retries for failed image pushes",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:   "build",
			Usage:  "launches a build for the specified Rockerfile",
			Action: buildCommand,
			Flags:  buildFlags,
		},
		{
			Name:   "pull",
			Usage:  "launches a pull of image (supports s3 storage driver)",
			Action: pullCommand,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "file, f",
					Value: "Rockerfile",
					Usage: "rocker build file to execute",
				},
				cli.StringFlag{
					Name:  "auth, a",
					Value: "",
					Usage: "Username and password in user:password format",
				},
				cli.StringFlag{
					Name:  "cache-dir",
					Value: "~/.rocker_cache",
					Usage: "Set the directory where the cache will be stored",
				},
			},
		},
		dockerclient.InfoCommandSpec(),
	}

	app.Before = func(c *cli.Context) error {
		initLogs(c)

		if c.GlobalBool("cmd") {
			log.Infof("rocker %s | Cmd: %s\n", HumanVersion, strings.Join(os.Args, " "))
		}

		return nil
	}

	app.CommandNotFound = func(ctx *cli.Context, command string) {
		fmt.Printf("Command not found: %v\n", command)
		os.Exit(1)
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf(err.Error())
		os.Exit(1)
	}
}

func buildCommand(c *cli.Context) {

	var (
		rockerfile *build.Rockerfile
		err        error
	)

	// We don't want info level for 'print' mode
	// So log only errors unless 'debug' is on
	if c.Bool("print") && log.StandardLogger().Level != log.DebugLevel {
		log.StandardLogger().Level = log.ErrorLevel
	}

	vars, err := template.VarsFromFileMulti(c.StringSlice("vars"))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	cliVars, err := template.VarsFromStrings(c.StringSlice("var"))
	if err != nil {
		log.Fatal(err)
	}

	vars = vars.Merge(cliVars)

	if c.Bool("demand-artifacts") {
		vars["DemandArtifacts"] = true
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	configFilename := c.String("file")
	contextDir := wd

	if configFilename == "-" {

		rockerfile, err = build.NewRockerfile(filepath.Base(wd), os.Stdin, vars, template.Funs{})
		if err != nil {
			log.Fatal(err)
		}

	} else {

		if !filepath.IsAbs(configFilename) {
			configFilename = filepath.Join(wd, configFilename)
		}

		rockerfile, err = build.NewRockerfileFromFile(configFilename, vars, template.Funs{})
		if err != nil {
			log.Fatal(err)
		}

		// Initialize context dir
		contextDir = filepath.Dir(configFilename)
	}

	args := c.Args()
	if len(args) > 0 {
		contextDir = args[0]
		if !filepath.IsAbs(contextDir) {
			contextDir = filepath.Join(wd, args[0])
		}
	} else if contextDir != wd {
		log.Warningf("Implicit context directory used: %s. You can override context directory using the last argument.", contextDir)
	}

	dir, err := os.Stat(contextDir)
	if err != nil {
		log.Errorf("Problem with opening directory %s, error: %s", contextDir, err)
		os.Exit(2)
	}
	if !dir.IsDir() {
		log.Errorf("Context directory %s is not a directory.", contextDir)
		os.Exit(2)

	}
	log.Debugf("Context directory: %s", contextDir)

	if c.Bool("print") {
		fmt.Print(rockerfile.Content)
		os.Exit(0)
	}

	dockerignore := []string{}

	dockerignoreFilename := filepath.Join(contextDir, ".dockerignore")
	if _, err := os.Stat(dockerignoreFilename); err == nil {
		if dockerignore, err = build.ReadDockerignoreFile(dockerignoreFilename); err != nil {
			log.Fatal(err)
		}
	}

	var config *dockerclient.Config
	config = dockerclient.NewConfigFromCli(c)

	dockerClient, err := dockerclient.NewFromConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	cacheDir, err := util.MakeAbsolute(c.String("cache-dir"))
	if err != nil {
		log.Fatal(err)
	}

	var cache build.Cache
	if !c.Bool("no-cache") {
		cache = build.NewCacheFS(cacheDir)
	}

	var (
		stdoutContainerFormatter log.Formatter = &log.JSONFormatter{}
		stderrContainerFormatter log.Formatter = &log.JSONFormatter{}
	)
	if !c.GlobalBool("json") {
		stdoutContainerFormatter = build.NewMonochromeContainerFormatter()
		stderrContainerFormatter = build.NewColoredContainerFormatter()
	}

	options := build.DockerClientOptions{
		Client:                   dockerClient,
		Auth:                     initAuth(c),
		Log:                      log.StandardLogger(),
		S3storage:                s3.New(dockerClient, cacheDir),
		StdoutContainerFormatter: stdoutContainerFormatter,
		StderrContainerFormatter: stderrContainerFormatter,
		PushRetryCount:           c.Int("push-retry"),
		Host:                     config.Host,
		LogExactSizes:            c.GlobalBool("json"),
	}
	client := build.NewDockerClient(options)

	builder := build.New(client, rockerfile, cache, build.Config{
		InStream:      os.Stdin,
		OutStream:     os.Stdout,
		ContextDir:    contextDir,
		Dockerignore:  dockerignore,
		ArtifactsPath: c.String("artifacts-path"),
		Pull:          c.Bool("pull"),
		NoGarbage:     c.Bool("no-garbage"),
		Attach:        c.Bool("attach"),
		Verbose:       c.GlobalBool("verbose"),
		ID:            c.String("id"),
		NoCache:       c.Bool("no-cache"),
		ReloadCache:   c.Bool("reload-cache"),
		Push:          c.Bool("push"),
		CacheDir:      cacheDir,
		LogJSON:       c.GlobalBool("json"),
		BuildArgs:     BuilargKVStringsToMap(c.StringSlice("build-arg")),
	})

	plan, err := build.NewPlan(rockerfile.Commands(), true)
	if err != nil {
		log.Fatal(err)
	}

	// Check the docker connection before we actually run
	if err := dockerclient.Ping(dockerClient, 5000); err != nil {
		log.Fatal(err)
	}

	if err := builder.Run(plan); err != nil {
		log.Fatal(err)
	}

	fields := log.Fields{}
	if c.GlobalBool("json") {
		fields["size"] = builder.VirtualSize
		fields["delta"] = builder.ProducedSize
	}

	size := fmt.Sprintf("final size %s (+%s from the base image)",
		units.HumanSize(float64(builder.VirtualSize)),
		units.HumanSize(float64(builder.ProducedSize)),
	)

	log.WithFields(fields).Infof("Successfully built %.12s | %s", builder.GetImageID(), size)
}

func pullCommand(c *cli.Context) {
	args := c.Args()
	if len(args) < 1 {
		log.Fatal("rocker pull <image>")
	}

	dockerClient, err := dockerclient.NewFromCli(c)
	if err != nil {
		log.Fatal(err)
	}

	cacheDir, err := util.MakeAbsolute(c.String("cache-dir"))
	if err != nil {
		log.Fatal(err)
	}

	options := build.DockerClientOptions{
		Client:                   dockerClient,
		Auth:                     initAuth(c),
		Log:                      log.StandardLogger(),
		S3storage:                s3.New(dockerClient, cacheDir),
		StdoutContainerFormatter: log.StandardLogger().Formatter,
		StderrContainerFormatter: log.StandardLogger().Formatter,
	}
	client := build.NewDockerClient(options)

	if err := client.PullImage(args[0]); err != nil {
		log.Fatal(err)
	}
}

func initAuth(c *cli.Context) (auth *docker.AuthConfigurations) {
	var err error
	if c.IsSet("auth") {
		// Obtain auth configuration from cli params
		authParam := c.String("auth")
		if strings.Contains(authParam, ":") {
			userPass := strings.Split(authParam, ":")
			auth = &docker.AuthConfigurations{
				Configs: map[string]docker.AuthConfiguration{
					"*": docker.AuthConfiguration{
						Username: userPass[0],
						Password: userPass[1],
					},
				},
			}
		}
		return
	}
	// Obtain auth configuration from .docker/config.json
	if auth, err = docker.NewAuthConfigurationsFromDockerCfg(); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	return
}

func initLogs(ctx *cli.Context) {
	logger := log.StandardLogger()

	if ctx.GlobalBool("verbose") {
		logger.Level = log.DebugLevel
	}

	var (
		isTerm    = log.IsTerminal()
		json      = ctx.GlobalBool("json")
		useColors = isTerm && !json
	)

	if ctx.GlobalIsSet("colors") {
		useColors = ctx.GlobalBool("colors")
	}

	color.NoColor = !useColors

	if json {
		logger.Formatter = &log.JSONFormatter{}
	} else {
		formatter := &textformatter.TextFormatter{}
		formatter.DisableColors = !useColors

		logger.Formatter = formatter
	}
}

func stringOr(args ...string) string {
	for _, str := range args {
		if str != "" {
			return str
		}
	}
	return ""
}

// BuilargKVStringsToMap converts ["key=value"] to {"key":"value"}
// Also, if the value is omitted (e.g. --build-arg NPM_TOKEN), then the ENV variable value will be taken
// You can still force the value to be empty by specifying --build-arg NPM_TOKEN=""
func BuilargKVStringsToMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		kv := strings.SplitN(value, "=", 2)
		if len(kv) == 1 {
			result[kv[0]] = os.Getenv(kv[0])
		} else {
			result[kv[0]] = kv[1]
		}
	}

	return result
}
