package tests

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestExportSimple(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export")
	assert.Nil(t, err, "Can't create tmp dir")
	os.RemoveAll(dir)

	err = runRockerBuildWithOptions(`
		FROM alpine:latest
		RUN echo -n "test_export" > /exported_file
		EXPORT /exported_file

		FROM alpine:latest
		MOUNT `+dir+`:/datadir
		IMPORT /exported_file /datadir/imported_file`, "--no-cache")
	assert.Nil(t, err, "Failed to run rocker build")

	content, err := ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err, "Can't read file")

	assert.Equal(t, "test_export", string(content))
}
