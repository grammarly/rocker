VERSION := 0.2.0

OSES := linux darwin
ARCHS := amd64
BINARIES := rocker

LAST_TAG = $(shell git describe --abbrev=0 --tags 2>/dev/null)
GITCOMMIT = $(shell git rev-parse HEAD 2>/dev/null)
GITBRANCH = $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
BUILDTIME = $(shell date "+%Y-%m-%d %H:%M GMT")

GITHUB_USER := grammarly
GITHUB_REPO := rocker
GITHUB_RELEASE := docker run --rm -ti \
											-e GITHUB_TOKEN=$(GITHUB_TOKEN) \
											-v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt \
											-v $(shell pwd)/dist:/dist \
											dockerhub.grammarly.io/tools/github-release:master

ALL_ARCHS := $(foreach os, $(OSES), $(foreach arch, $(ARCHS), $(os)/$(arch) ))
ALL_BINARIES := $(foreach arch, $(ALL_ARCHS), $(foreach bin, $(BINARIES), dist/$(VERSION)/$(arch)/$(bin) ))
OUT_BINARIES := $(foreach arch, $(ALL_ARCHS), $(foreach bin, $(BINARIES), dist/$(bin)_$(subst /,_,$(arch)) ))
ALL_TARS := $(ALL_BINARIES:%=%.tar.gz)

os = $(shell echo "$(1)" | awk -F/ '{print $$3}' )
arch = $(shell echo "$(1)" | awk -F/ '{print $$4}' )
bin = $(shell echo "$(1)" | awk -F/ '{print $$5}' )

UPLOAD_CMD = $(GITHUB_RELEASE) upload \
			--user $(GITHUB_USER) \
			--repo $(GITHUB_REPO) \
			--tag $(VERSION) \
			--name $(call bin,$(FILE))-$(VERSION)_$(call os,$(FILE))_$(call arch,$(FILE)).tar.gz \
			--file $(FILE).tar.gz

SRCS = $(shell git ls-files '*.go' | grep -v '^vendor/')
PKGS := $(foreach pkg, $(sort $(dir $(SRCS))), ./$(pkg))

export GOPATH = $(shell pwd):$(shell pwd)/vendor

all: $(ALL_BINARIES)
	$(foreach BIN, $(BINARIES), $(shell cp dist/$(VERSION)/$(shell go env GOOS)/amd64/$(BIN) dist/$(BIN)))

$(OUT_BINARIES): $(ALL_BINARIES)
	cp $< $@

release: $(ALL_TARS)
	git pull
	git push && git push --tags
	$(GITHUB_RELEASE) release \
			--user $(GITHUB_USER) \
			--repo $(GITHUB_REPO) \
			--tag $(VERSION) \
			--name $(VERSION) \
			--description "https://github.com/$(GITHUB_USER)/$(GITHUB_REPO)/compare/$(LAST_TAG)...$(VERSION)"
	$(foreach FILE,$(ALL_BINARIES),$(UPLOAD_CMD);)

tar: $(ALL_TARS)

%.tar.gz: %
	COPYFILE_DISABLE=1 tar -zcvf $@ -C dist/$(VERSION)/$(call os,$<)/$(call arch,$<) $(call bin,$<)

$(ALL_BINARIES): build_image
	docker run --rm -ti -v $(shell pwd)/dist:/src/dist \
		-e GOOS=$(call os,$@) -e GOARCH=$(call arch,$@) -e GOPATH=/src:/src/vendor \
		rocker-build:latest go build \
		-ldflags "-X main.Version '$(VERSION)' -X main.GitCommit '$(GITCOMMIT)' -X main.GitBranch '$(GITBRANCH)' -X main.BuildTime '$(BUILDTIME)'" \
		-v -o $@ src/cmd/$(call bin,$@)/main.go

build_image:
	rocker build -f Rockerfile.build-cross

docker_image:
	rocker build -var Version=$(VERSION)

install:
	cp dist/$(VERSION)/$(shell go env GOOS)/amd64/rocker /usr/local/bin/rocker
	chmod +x /usr/local/bin/rocker

clean:
	rm -Rf dist

fmtcheck:
	$(foreach file,$(SRCS),goimports $(file) | diff -u $(file) - || exit;)

lint:
	@ go get -v github.com/golang/lint/golint
	@ go get github.com/GeertJohan/fgt
	$(foreach file,$(SRCS),fgt golint $(file) || exit;)

vet:
	@-go get -v golang.org/x/tools/cmd/vet
	@ go get github.com/GeertJohan/fgt
	$(foreach pkg,$(PKGS),fgt go vet $(pkg) || exit;)

gocyclo:
	@ go get -v github.com/fzipp/gocyclo
	gocyclo -over 25 ./src

test: fmtcheck vet lint
	@ go get -v github.com/constabulary/gb/...
	gb test rocker/...

.PHONY: clean build_image test fmtcheck lint vet gocyclo
