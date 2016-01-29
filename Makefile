VERSION ?= $(shell cat VERSION)

GITCOMMIT = $(shell git rev-parse HEAD 2>/dev/null)
GITBRANCH = $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
BUILDTIME := $(shell TZ=GMT date "+%Y-%m-%d_%H:%M_GMT")

SRCS = $(shell find . -name '*.go' | grep -v '^./vendor/')
PKGS := $(foreach pkg, $(sort $(dir $(SRCS))), $(pkg))

TESTARGS ?=

default:
	go build

install:
	cp rocker /usr/local/bin/rocker
	chmod +x /usr/local/bin/rocker

dist_dir:
	mkdir -p ./dist/linux_amd64
	mkdir -p ./dist/darwin_amd64


cross: dist_dir
	docker run --rm -ti -v $(shell pwd):/go/src/github.com/grammarly/rocker \
		-e GOOS=linux -e GOARCH=amd64 -e GO15VENDOREXPERIMENT=1 \
		-w /go/src/github.com/grammarly/rocker \
		dockerhub.grammarly.io/golang-1.5.1-cross:v1 go build \
		-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GITCOMMIT) -X main.GitBranch=$(GITBRANCH) -X main.BuildTime=$(BUILDTIME)" \
		-v -o ./dist/linux_amd64/rocker

	docker run --rm -ti -v $(shell pwd):/go/src/github.com/grammarly/rocker \
		-e GOOS=darwin -e GOARCH=amd64 -e GO15VENDOREXPERIMENT=1 \
		-w /go/src/github.com/grammarly/rocker \
		dockerhub.grammarly.io/golang-1.5.1-cross:v1 go build \
		-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GITCOMMIT) -X main.GitBranch=$(GITBRANCH) -X main.BuildTime=$(BUILDTIME)" \
		-v -o ./dist/darwin_amd64/rocker

cross_tars: cross
	COPYFILE_DISABLE=1 tar -zcvf ./dist/rocker_linux_amd64.tar.gz -C dist/linux_amd64 rocker
	COPYFILE_DISABLE=1 tar -zcvf ./dist/rocker_darwin_amd64.tar.gz -C dist/darwin_amd64 rocker

clean:
	rm -Rf dist

testdeps:
	@ go get github.com/GeertJohan/fgt

fmtcheck:
	$(foreach file,$(SRCS),gofmt $(file) | diff -u $(file) - || exit;)

lint:
	@ go get github.com/golang/lint/golint
	$(foreach file,$(SRCS),fgt golint $(file) || exit;)

vet:
	@ go get golang.org/x/tools/cmd/vet
	$(foreach pkg,$(PKGS),fgt go vet $(pkg) || exit;)

gocyclo:
	@ go get github.com/fzipp/gocyclo
	gocyclo -over 25 ./src

test: testdeps fmtcheck vet lint
	go test ./src/... $(TESTARGS)

integ:
	go test ./src/... -tags="integration" -run TestInteg_

.PHONY: clean test fmtcheck lint vet gocyclo default
