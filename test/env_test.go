package tests

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEnvExpansion(t *testing.T) {

	err := runRockerBuildWithOptions(`
FROM alpine
ENV foo=bar
ENV qux=replaced-$foo
RUN touch /$qux
RUN ls -l /replaced-bar`, "--no-cache")

	assert.Nil(t, err, "should expand variable in ENV command")
}

func TestEnvExpansionInOnbuild(t *testing.T) {

	err := runRockerBuildWithOptions(`
FROM alpine
ENV foo=bar
ONBUILD ENV qux=replaced-onbuild-$foo
TAG bla
FROM bla
RUN touch /$qux
RUN ls -l /replaced-onbuild-bar`, "--no-cache")

	assert.Nil(t, err, "should expand variable in ONBUILD command")
}

func TestEnvExpansionPath(t *testing.T) {
	err := runRockerBuildWithOptions(`
FROM alpine
ENV foo=/opt/foo/bin:$PATH
RUN echo $foo
RUN test $foo == /opt/foo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
`, "--no-cache")
	assert.Nil(t, err, "should use PATH from the default PATH setting, if PATH is not set "+
		"with ENV command in any of the parent containers")
}
