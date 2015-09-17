
SRCS = $(shell find . -name '*.go' | grep -v '^./vendor/')
PKGS := $(foreach pkg, $(sort $(dir $(SRCS))), $(pkg))

TESTARGS ?=

deps:
	@ go get github.com/kr/pretty

testdeps: deps
	@ go get github.com/GeertJohan/fgt
	@ go get github.com/stretchr/testify/assert

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
	go test

.PHONY: test fmtcheck lint vet gocyclo
