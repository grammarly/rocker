#!/bin/bash
set -e 

#
# Don't forget to generate and commit changelog before making a release
# CHANGELOG_GITHUB_TOKEN=XXX github_changelog_generator --since-tag 1.1.0 --base CHANGELOG.md --no-issues
# 

VERSION=`cat VERSION`
LAST_TAG=`git describe --abbrev=0 --tags 2>/dev/null`

GITHUB_USER=grammarly
GITHUB_REPO=rocker

docker run --rm -ti \
  -e GITHUB_TOKEN=$GITHUB_TOKEN \
  -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt \
  -v `pwd`/dist:/dist \
  dockerhub.grammarly.io/tools/github-release:master release \
      --user $GITHUB_USER \
      --repo $GITHUB_REPO \
      --tag $VERSION \
      --name $VERSION \
      --description "https://github.com/$GITHUB_USER/$GITHUB_REPO/compare/$LAST_TAG...$VERSION"

docker run --rm -ti \
  -e GITHUB_TOKEN=$GITHUB_TOKEN \
  -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt \
  -v `pwd`/dist:/dist \
  dockerhub.grammarly.io/tools/github-release:master upload \
      --user $GITHUB_USER \
      --repo $GITHUB_REPO \
      --tag $VERSION \
      --name rocker-$VERSION-linux_amd64.tar.gz \
      --file ./dist/rocker_linux_amd64.tar.gz

docker run --rm -ti \
  -e GITHUB_TOKEN=$GITHUB_TOKEN \
  -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt \
  -v `pwd`/dist:/dist \
  dockerhub.grammarly.io/tools/github-release:master upload \
      --user $GITHUB_USER \
      --repo $GITHUB_REPO \
      --tag $VERSION \
      --name rocker-$VERSION-darwin_amd64.tar.gz \
      --file ./dist/rocker_darwin_amd64.tar.gz
