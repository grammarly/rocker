package tests

import "testing"

func TestEnv_Expansion(t *testing.T) {

	err := runRockerBuild(`
FROM alpine
ENV foo=bar
ENV qux=replaced-$foo
RUN touch /$qux
RUN ls -l /replaced-bar`, "--no-cache")

	if err != nil {
		t.Fatal("should expand variable in ENV command:", err)
	}
}

func TestEnv_ExpansionInOnbuild(t *testing.T) {

	err := runRockerBuild(`
FROM alpine
ENV foo=bar
ONBUILD ENV qux=replaced-onbuild-$foo
TAG bla
FROM bla
RUN touch /$qux
RUN ls -l /replaced-onbuild-bar`, "--no-cache")

	if err != nil {
		t.Fatal("should expand variable in ONBUILD command:", err)
	}
}

func TestEnv_ExpansionPath(t *testing.T) {
	err := runRockerBuild(`
FROM alpine
ENV foo=/opt/foo/bin:$PATH
RUN echo $foo
RUN test $foo == /opt/foo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
`, "--no-cache")

	if err != nil {
		t.Fatal("should use PATH from the default PATH setting, if PATH is not set "+
			"with ENV command in any of the parent containers:", err)
	}
}
