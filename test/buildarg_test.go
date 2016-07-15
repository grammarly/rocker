package tests

import "testing"

func TestBuildArg_Simple(t *testing.T) {
	rockerfileContent := `
FROM alpine
RUN export | grep proxy`

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerfileContent,
		buildParams:       []string{"--no-cache", "--build-arg", "http_proxy=http://host:3128"},
		testLines:         []string{"export http_proxy='http://host:3128'"},
	})
	if err != nil {
		t.Fatal(err)
	}
}
