package tests

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

func TestExportSimple(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

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

func TestExportMultipleFrom(t *testing.T) {
}

func TestExportSmolinIssue(t *testing.T) {
	t.Skip()
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

	rockerfile, err := createTempFile("")
	assert.Nil(t, err, "Can't create temp file")
	defer os.RemoveAll(rockerfile)

	rockerContentFirst := []byte(`FROM alpine
						 	 RUN echo -n "first" > /exported_file
						 	 EXPORT /exported_file
						 	 FROM alpine
						 	 MOUNT ` + dir + `:/datadir
						 	 IMPORT /exported_file /datadir/imported_file`)

	rockerContentSecond := []byte(`FROM alpine
							  RUN echo -n "second" > /exported_file
							  EXPORT /exported_file
							  FROM alpine
							  MOUNT ` + dir + `:/datadir
							  IMPORT /exported_file /datadir/imported_file`)

	err = ioutil.WriteFile(rockerfile, rockerContentFirst, 0644)
	assert.Nil(t, err)

	err = runRockerBuildWithFile(rockerfile, "--reload-cache")
	assert.Nil(t, err)

	content, err := ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err)
	assert.Equal(t, "first", string(content))

	err = ioutil.WriteFile(rockerfile, rockerContentSecond, 0644)
	assert.Nil(t, err)

	err = runRockerBuildWithFile(rockerfile)
	assert.Nil(t, err)

	content, err = ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err)
	assert.Equal(t, "second", string(content))

	err = ioutil.WriteFile(rockerfile, rockerContentFirst, 0644)
	assert.Nil(t, err)

	err = runRockerBuildWithFile(rockerfile)
	assert.Nil(t, err)

	content, err = ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err, "Can't read file")
	assert.Equal(t, "first", string(content))
}
