package tests

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMountLocal(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_mount_dir")
	if err != nil {
		t.Fatalf("Can't create temp dir, err : %v", err)
	}
	defer os.Remove(dir)

	err = runRockerBuildWithOptions("FROM alpine:latest\n"+
		"MOUNT "+dir+":/datadir\n"+
		"RUN echo -n foobar > /datadir/foo", "--no-cache")

	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	content, _ := ioutil.ReadFile(dir + "/foo")

	if "foobar" != string(content) {
		t.Fatal("Content doesn't match")
	}
}
