# rocker

Rocker breaks the limits of Dockerfile. It adds some crucial features that are missing while keeping Docker’s original design and idea. Read the [blog post](http://tech.grammarly.com/blog/posts/Making-Docker-Rock-at-Grammarly.html) about how and why it was invented.

# *NOTE on v1 branch*
In this branch we are developing the new experimental implementation of Rocker that will be completely client-side driven, with no fallback on `docker build`. This means faster builds and more power. No build context uploads anymore. Also, the builder code is completely rewritten and made much more testable and extensible in the future. Caching might be also rethought. Cross-server builds determinism is our dream. 

### v1 TODO

- [ ] Cache
- [ ] FROM scratch
- [ ] ADD urls and local archives

---

* [Installation](#installation)
* [Rockerfile](#rockerfile)
  * [MOUNT](#mount)
  * [FROM](#from)
  * [EXPORT/IMPORT](#exportimport)
  * [TAG](#tag)
  * [PUSH](#push)
  * [Templating](#templating)
  * [REQUIRE](#require)
  * [INCLUDE](#include)
  * [ATTACH](#attach)
* [Where to go next?](#where-to-go-next)
* [Contributing](#contributing)
* [TODO](#todo)
* [License](#license)

# Installation

### For OSX users

```
brew tap grammarly/tap
brew install grammarly/tap/rocker
```

Ensure that it is built with `go 1.5.x` . If not, make `brew update` before installing `rocker`.

### Manual installation

Go to the [releases](https://github.com/grammarly/rocker/releases) section and download the latest binary for your platform. Then unpack the tar archive and copy the binary somewhere to your path, such as `/usr/local/bin`, and give it executable permissions.

Something like this:
```bash
curl -SL https://github.com/grammarly/rocker/releases/download/0.2.2/rocker-0.2.2_darwin_amd64.tar.gz | tar -xzC /usr/local/bin && chmod +x /usr/local/bin/rocker
```

### Getting help, usage:

```bash
rocker --help
rocker build --help
```

# Rockerfile

It is a backward compatible replacement for Dockerfile. Yes, you can take any Dockerfile, rename it to `Rockerfile` and use `rocker build` instead of `docker build`. What’s the point then? No point. Unless you want to use advanced Rocker commands.

By introducing new commands, Rocker aims to solve the following use cases, which are painful with plain Docker:

1. Mount reusable volumes on build stage, so dependency management tools may use cache between builds.
2. Share ssh keys with build (for pulling private repos, etc.), while not leaving them in the resulting image.
3. Build and run application in different images, be able to easily pass an artifact from one image to another, ideally have this logic in a single Dockerfile.
4. Tag/Push images right from Dockerfiles.
5. Pass variables from shell build command so they can be substituted to a Dockerfile.

And more. These are the most critical issues that were blocking our adoption of Docker at Grammarly.

The most challenging part is caching. While implementing those features seems to be not a big deal, it's not trivial to do that just by utilising Docker’s image cache (the one that `docker build` does). Actually, it is the main reason why those features are still not in Docker. With Rocker we achieve this by introducing a set of trade-offs. Search this page for "trade-off" to find out more details.

### How does it work

Rocker parses the Rockerfile into an AST using the same library Docker uses for parsing Dockerfiles. Then it goes through the instructions and makes a decision, should it execute a command on its own or delegate it to Docker. Internally, Rocker splits a Rockerfile into slices, some of them are executed through Docker’s remote API, some are sent as regular Dockerfiles underneath. This allows to not reimplement the whole thing — only add custom stuff. So if you have a plain Dockerfile, Rocker will not find any custom commands, so it will just pass it straight to Docker.

# MOUNT

```
MOUNT /root/.gradle
```
or
```bash
MOUNT /app/mode_modules /app/bower_components
```
or
```bash
MOUNT .:/src
```
or
```bash
MOUNT $GIT_SSH_KEY:/root/.ssh/id_rsa
```

`MOUNT` is used to share volumes between builds, so they can be reused by tools like dependency management. There are two types of mounts:

1. Share directory from host machine — using the format `source:dest`
2. Using volume container — not using `:`

Volume container names are hashed with Rockerfile’s full path and the directories it shares. So as long as your Rockerfile has the same name and it is in the same place — same volume containers will be used.

Note that Rocker is not tracking changes in mounted directories, so no changes can affect caching. Cache will be busted only if you change list of mounts, add or remove them. In future, we may add some configuration flags, so you can specify if you want to watch the actual mount contents changes, and make them invalidate the cache (for example $GIT_SSH_KEY contents may change).

To force cache invalidation you can always use `--no-cache` flag for `rocker build` command. But you will then need a lot of patience.

**Example usage**

```bash
FROM grammarly/nodejs:latest
ADD . /src                                    #1
WORKDIR /src                                
MOUNT /src/node_modules /src/bower_components #2
MOUNT $GIT_SSH_KEY:/root/.ssh/id_rsa          #3
RUN npm install                               #4
RUN cp -R /src /app                           #5
WORKDIR /app                                 
CMD ["/usr/bin/node", "index.js"]
```

1. We add the whole current directory into a path `/src` inside the container. Note that the `ADD` command is executed by Docker and a [fancy caching mechanism](https://docs.docker.com/articles/dockerfile_best-practices/) is applied here. So every time something changes within the directory, Docker will invalidate the cache, and all following commands will be re-executed. That’s why we want to keep npm stuff somewhere, to not run it from scratch.
2. Over already existing `/src` directory, we mount two volumes, so everything npm and bower will put there will be reused by following builds. Note that **these directories will not remain in the image**, so we have to copy the whole thing to keep it, as you will see below.
3. Mount our git ssh key from host machine, so a private repositories can be fetched by npm and bower. We **should have `GIT_SSH_KEY` env variable exported** (referring to a key file) while running build command.
4. Install the app. Here, something interesting happens. Normally, Rocker passes `RUN` commands straight to Docker. Unless it sees any `MOUNT` commands above. It this case, Rocker will execute `RUN` itself mounting desired volumes in the background.
5. Copy the whole app (with it’s dependencies) to a new folder, so we can keep `node_modules` and `bower_components` directories inside the image. Remember that **mounted directories are transient**. This will not be needed for languages like Java for which the resulting artifact contains everything it needs and the dependency cache can be kept in a separate directory.

**Another example**

```bash
FROM debian:jessie
ADD . /src
WORKDIR /src                                    
RUN make
MOUNT .:/context                                #1
RUN cp program.o /context/program_linux_x86     #2
```

1. Here we mount the current directory from host machine, so we can copy files straight from the image. The directory can now be addressed inside the image by the path `/context`. So, everything we copy there will appear in our current directory.
2. Copy the compiled program to `/context` (the current directory on host machine)

This approach can be used if we want to use Docker for the build context, while keeping our machine and source directory clean.

# FROM

```bash
# build image
FROM google/golang:1.4
MOUNT .:/src
WORKDIR /src
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -v -o rocker.o rocker.go

# run image
FROM busybox
ADD rocker.o /bin/rocker
CMD ["/bin/rocker"]
```

Yes. There are two FROMs in a single Rockerfile. This is useful when we want to have different images for building and running the application (**build image** and **run image**). The point is that the resulting image weights just 4.3Mb, while the build image is 611Mb.

The first image puts the statically compiled binary into our context directory. The second one simply picks it through the `ADD` instruction.

Rocker executes them in a row as a single Dockerfile. The only exception is that `MOUNT`s are not shared between `FROM`s, if you want, you have to declare them again.

# EXPORT/IMPORT

```bash
EXPORT file
```
or
```bash
EXPORT file /to/path
```
or
```bash
EXPORT /path/to/file /to/path
```
and
```bash
IMPORT file
```
or
```bash
IMPORT /exported/path
```
or
```bash
IMPORT /exported/path /to/path
```

Sometimes you don’t want to use a context directory but, instead, to directly pass files from one image to another. Also, copying is not the most efficient way of dealing with large directory trees. That’s when `EXPORT`/`IMPORT` comes handy.

Let’s take our previous example and rewrite it to use `EXPORT`/`IMPORT`.

```bash
FROM google/golang:1.4
ADD . /src
WORKDIR /src
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -v -o rocker.o rocker.go
EXPORT rocker.o                    #1

FROM busybox
IMPORT rocker.o /bin/rocker        #2
CMD ["/bin/rocker"]
```

This Rockerfile gives the same result as the previous one. But nothing is placed in our current directory. How does it happen? Rocker creates a shared volume container and uses [rsync](http://linux.die.net/man/1/rsync) to copy files. With `EXPORT`, it copies from image to a volume container, with `IMPORT` it does the opposite.

The following rsync commands will be executed behind the scenes:

1. `/opt/rsync/bin/rsync-static -a --delete-during rocker.o /.rocker_exports`
2. `/opt/rsync/bin/rsync-static -a /.rocker_exports/rocker.o /bin/rocker`

Where `/.rocker_exports` is a mounted volume container directory.

1. Mount volume container will be reused between builds. It’s name is hashed by the full path of a Rockerfile. [trade-off] So as long as your Rockerfile is in same directory and has the same name, mount volume container will be kept.
2. All `EXPORT`s/`IMPORT`s within a Rockerfile share the same mount volume container directory [trade-off]
3. For both `EXPORT` and `IMPORT`, the destination folder will not be created automatically if it didn't exist before, so you have to manually create subdirectories before doing import.
4. `EXPORT` will be cached until it’s volume container is removed or the command value changes
5. All subsequent `IMPORT`s cache depend on the last `EXPORT` cache id, so as long as the last `EXPORT` is cached, all `IMPORT`s will be cached too.

If you omit a destination argument specifying only source, both `EXPORT` and `IMPORT` will add "/" as a destination automatically.

```bash
EXPORT /src     # will be EXPORT /src /
IMPORT /src     # will be IMPORT /src /
```

As mentioned earlier, root folder for exports and imports is a shared volume, which is located in `/.rocker_exports`, so to clarify it completely, the following will happen:

```bash
EXPORT /src /     # will rsync /src /.rocker_exports
IMPORT /src /     # will rsync /.rocker_exports/src /
```

**Be careful with paths**

If you are going to export directories, not single files, you have to consider using a trailing slash "/". [That matters for rsync](http://devblog.virtage.com/2013/01/to-trailing-slash-or-not-to-trailing-slash-to-rsync-path/). To get an intuitively expected behavior, you have to add a slash to the end of source directory being exported.

```bash
FROM debian:jessie
ADD . /src
RUN cd src && make

EXPORT /src            # will be /.rocker_exports/src 
EXPORT /src  /app      # will be /.rocker_exports/app/src
EXPORT /src/ /app      # will be /.rocker_exports/app   -- which is the most expected behavior
```

**A few recipes**

If you have a directory in the build image and you want the same one in the run image:

```bash
FROM debian:jessie
ADD . /src
RUN cd src && make
EXPORT /src

FROM debian:jessie
IMPORT /src
```

The same, but you want the directory to be named differently in the run image:

```bash
FROM debian:jessie
ADD . /src
RUN cd src && make
EXPORT /src/ /app

FROM debian:jessie
IMPORT /app
```

# TAG

```bash
FROM google/golang:1.4
ADD . /src
WORKDIR /src
RUN go build -v -o /bin/rocker rocker.go
CMD ["/bin/rocker"]
TAG grammarly/rocker:1
```

*TODO: note about the need of explicit :latest*

It simply tags the image at the current build stage. Possibly interesting feature is that you actually can use it anywhere in between commands.

```bash
FROM google/golang:1.4
ADD . /src
WORKDIR /src
TAG rocker-before-compile                             # here
RUN go build -v -o /bin/rocker rocker.go
CMD ["/bin/rocker"]
TAG grammarly/rocker:1
```

# PUSH

Same as `TAG`, but it pushes to a registry if `--push` flag is passed to `rocker build` command. If the flag is not passed, it just `TAG`s. Useful for CI.

```bash
FROM google/golang:1.4
…
CMD ["/bin/rocker"]
PUSH grammarly/rocker:1
```

# Templating

`rocker` uses Go's [text/template](http://golang.org/pkg/text/template/) to pre-process Rockerfiles prior to execution. We extend it with additional helpers from [rocker/template](/src/rocker/template) package that is shared with [rocker-compose](https://github.com/grammarly/rocker-compose) as well.

Example:

```bash
FROM google/golang:1.4
…
CMD ["/bin/rocker"]
PUSH grammarly/rocker:{{ .Version }}
```

Pass the `Version` variable this way:

```bash
rocker build -var Version=0.1.22
```

You can also test rendered Rockerfile by using `-print` option:

```bash
$ rocker build -var Version=0.1.22 -print
FROM google/golang:1.4
…
CMD ["/bin/rocker"]
PUSH grammarly/rocker:0.1.22
```

# REQUIRE

```bash
REQUIRE foo
```
or
```bash
REQUIRE ["foo", "bar"]
```

Useful when you use variables, for example for image name or tag (as shown above). In such case, you should specify the variable because otherwise the build doesn't make sense.

`REQUIRE` does not affect the cache and it doesn't produce any layers.

**Usage**
```bash
FROM google/golang:1.4
…
CMD ["/bin/rocker"]
REQUIRE Version
PUSH grammarly/rocker:{{ .Version }}
```

So if we run the build not specifying the version variable (like `-var "Version=123"`), it will fail
```bash
$ rocker build
...
Error: Var $Version is required but not set
```

# INCLUDE

```bash
INCLUDE path/to/mixin
```
or
```bash
INCLUDE ../../path/to/mixin
```

Adds ability to include other Dockerfiles or Rockerfiles into your file. Useful if you have some collections of mixins on the side, such as a recipe to install nodejs or python, and want to use them.

1. Paths passed to `INCLUDE` are relative to the Rockerfile's directory.
2. It is not allowed to nest includes, e.g. use `INCLUDE` in files which are being included.

**Usage**
```bash
# includes/install_nodejs
RUN apt-get install nodejs
```

```bash
# Rockerfile
FROM debian:jessie
INCLUDE includes/install_nodejs
ADD . /src
WORKDIR /src
CMD ["node", "app.js"]
```

# ATTACH
```bash
ATTACH
```
or
```bash
ATTACH ["/bin/bash"]
```

`ATTACH` allows you to run an intermediate step interactively. For example, if you are debugging a Rockerfile, you can simply place `ATTACH` in the middle of it, and have an interactive shell within a container, with all volumes mounted properly.

It is also useful for making development containers:

```bash
FROM phusion/passenger-ruby22

WORKDIR /src

MOUNT /var/lib/gems
MOUNT $GIT_SSH_KEY:/root/.ssh/id_rsa
MOUNT .:/src

RUN ["bundle", "install"]

ATTACH ["/bin/bash"]
```

With this Rockerfile, you can play with your Ruby application within the source tree by simply running `rocker build --attach`. Note that ruby gems will be isolated for this particular app.

**Notes**

* If no argument is specified, the last CMD will be taken
* `ATTACH`  works only with `rocker build --attach` flag specified. So you can leave the `ATTACH` instructions in the Rockerfile and nobody will be interrupted unless `--attach` is specified.

# Where to go next?

1. See [Rocker’s Rockerfile](/Rockerfile) as an example
2. See [Rsync Rockerfile](/rsync/Rockerfile)
3. See [Example Rockerfile](/example/Rockerfile)
4. Use [Rockerfile.tmLanguage](/Rockerfile.tmLanguage) for SublimeText

# Contributing

### Dependencies

Use [gb](http://getgb.io/) to test and build. We vendor all dependencies, you can find them under `/vendor` directory.

Please, use [gofmt](https://golang.org/cmd/gofmt/) in order to automatically re-format Go code into vendor standardized convention. Ideally, you have to set it on post-save action in your IDE. For SublimeText3, the [GoSublime](https://github.com/DisposaBoy/GoSublime) package does the right thing. Also, [solution for Intellij IDEA](http://marcesher.com/2014/03/30/intellij-idea-run-goimports-on-file-save/).

### Build

(will produce the binary into the `bin/` directory)
```bash
gb build
```

or build for all platforms:
```bash
make
```

If you have a github access token, you can also do a github release:
```bash
make release
```

Also a useful thing to have:
```bash
echo "make test" > .git/hooks/pre-push && chmod +x .git/hooks/pre-push
```

### Test 

```bash
make test
```

or
```bash
gb test rocker/...
```

### Test something particular

```bash
gb test rocker/... -run TestMyFunction
```

# TODO

- [x] Correctly handle streaming TTY from Docker, so we can show fancy progress bars
- [x] rocker build --attach? possibly allow to attach to a running container within build, so can run interactively; may be useful for dev images
- [ ] run own tar stream so there is no need to put a generated dockerfile into a working directory
- [ ] write reamde about rocker cli
- [ ] colorful output for terminals
- [ ] Should the same mounts be reused between different FROMs?
- [ ] rocker inspect; inspecting a Rockerfile - whilch mount/export containers are there
- [ ] SQUASH as discussed [here](https://github.com/docker/docker/issues/332)
- [ ] do not store properties in an image
- [ ] Read Rockerfile from stdin
- [ ] Make more TODOs here

```bash
grep -R TODO **/*.go | grep -v '^vendor/'
```

# License

(c) Copyright 2015 Grammarly, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
