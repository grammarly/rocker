package tests

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMountLocal(t *testing.T) {
	//t.Skip("skipping till bug will be fixed")
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_mount_dir")
	if err != nil {
		t.Fatalf("Can't create temp dir, err : %v", err)
	}
	defer os.RemoveAll(dir)

	err = runRockerBuildWithOptions("FROM alpine:latest\n"+
		"MOUNT "+dir+":/datadir\n"+
		"RUN echo -n foobar > /datadir/foo", "--no-cache")

	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	content, err := ioutil.ReadFile(dir + "/foo")
	if err != nil {
		t.Fatalf("Can't read temp file. Error: %v", err)
	}

	if "foobar" != string(content) {
		t.Fatalf("Content doesn't match, expected: 'foobar', got: '%s'", string(content))
	}
}
