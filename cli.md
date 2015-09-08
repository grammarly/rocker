rocker [build] [-f rocker.yml] -e 'k=v;k=v' [--no-cache] [--no-reuse] [--publish]

Launches docker build for the specified file (rocker.yml in the current directory by default).

--no-cache  supresses cache for docker builds
--no-reuse  supresees reuse for all the tasks in the build.
--publish   publishes all the images marked with published: true to docker hub.

rocker --help, rocker -h, rocker help

Prints help.


rocker --verbose, rocker -v

Verbose mode.


rocker version

Prints current version.


rocker clean

Deletes all docker images with rocker-auto-generated names.



env vars:

- DOCKER_HOST
- ROBOT_GITHUB_SSH















