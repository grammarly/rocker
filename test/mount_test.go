package tests

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMount_Local(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_mount_dir")
	if err != nil {
		t.Fatalf("Can't create temp dir, err : %v", err)
	}
	defer os.RemoveAll(dir)

	err = runRockerBuild("FROM alpine:latest\n"+
		"MOUNT "+dir+":/datadir\n"+
		"RUN echo -n foobar > /datadir/foo", "--no-cache")
	if err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(dir + "/foo")
	if err != nil {
		t.Fatal("Can't read temp file:", err)
	}

	assert.Equal(t, "foobar", string(content), "Content doesn't match, expected: 'foobar'")
}
