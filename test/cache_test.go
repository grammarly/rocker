package tests

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestCacheWithEnvVariables(t *testing.T) {
	tag := "rocker-integratin-test:1.2.3"
	defer removeImage(tag)

	err := runRockerBuild(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)

	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuild(`
FROM alpine
RUN ENV_VAR=foo touch /tmp/foo
TAG ` + tag)
	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	if sha1 == sha2 {
		t.Fatal("Env variable should invalidate cache")
	}
}
func TestCacheWorksByDefault(t *testing.T) {
	tag := "rocker-integratin-test:1.2.3"
	defer removeImage(tag)

	err := runRockerBuild(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)

	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuild(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)
	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	if sha1 != sha2 {
		t.Fail()
	}
}

func TestNoCache(t *testing.T) {
	tag := "rocker-integratin-test:1.2.3"
	defer removeImage(tag)

	err := runRockerBuild(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)
	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuildWithOptions(`
FROM alpine
RUN touch /tmp/foo
TAG `+tag, "--no-cache")
	if err != nil {
		t.Fatalf("Test fail: %v\n", err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	if sha1 == sha2 {
		t.Fatalf("Sha of images are equal but shouldn't. sha1: %s, sha2: %s", sha1, sha2)
	}
}

func TestCacheFewSameCommands(t *testing.T) {
	tag := "rocker-integratin-test:1.8.35"
	defer removeImage(tag)
	scenario := `FROM alpine
				 RUN true
				 RUN true
				 RUN true
				 RUN true
				 RUN true
				 TAG ` + tag

	err := runRockerBuildWithOptions(scenario, "--reload-cache")
	assert.Nil(t, err)
	sha1, err := getImageShaByName(tag)
	assert.Nil(t, err)

	err = runRockerBuildWithOptions(scenario)
	assert.Nil(t, err)
	sha2, err := getImageShaByName(tag)
	assert.Nil(t, err)

	assert.Equal(t, sha1, sha2)
}

func TestCacheFewDifferentCommands(t *testing.T) {
	tag := "rocker-integratin-test:1.8.36"
	defer removeImage(tag)
	scenario := `FROM alpine
				 RUN true
				 RUN echo "123" > /foobar
				 RUN ls > /dev/null
				 RUN date
				 TAG ` + tag

	err := runRockerBuildWithOptions(scenario, "--reload-cache")
	assert.Nil(t, err)
	sha1, err := getImageShaByName(tag)
	assert.Nil(t, err)

	err = runRockerBuildWithOptions(scenario)
	assert.Nil(t, err)
	sha2, err := getImageShaByName(tag)
	assert.Nil(t, err)

	assert.Equal(t, sha1, sha2)
}

func TestCacheAndExportImport(t *testing.T) {
	tagExport := "rocker-integratin-test:export"
	tagImport := "rocker-integratin-test:import"
	defer removeImage(tagExport)
	defer removeImage(tagImport)

	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

	randomData := strconv.Itoa(int(time.Now().UnixNano() % int64(100000001)))
	scenario := `FROM alpine
				 RUN true
				 RUN echo -n ` + randomData + ` > /foobar
				 EXPORT /foobar
				 TAG ` + tagExport + `
				 FROM alpine
				 MOUNT ` + dir + `:/datadir
				 IMPORT  /foobar /datadir/foobar
				 TAG ` + tagImport

	err = runRockerBuildWithOptions(scenario, "--reload-cache")
	assert.Nil(t, err)
	shaExport1, err := getImageShaByName(tagExport)
	assert.Nil(t, err)
	shaImport1, err := getImageShaByName(tagImport)
	assert.Nil(t, err)
	content, err := ioutil.ReadFile(dir + "/foobar")
	assert.Equal(t, string(content), randomData)

	err = runRockerBuildWithOptions(scenario)
	assert.Nil(t, err)
	shaExport2, err := getImageShaByName(tagExport)
	assert.Nil(t, err)
	shaImport2, err := getImageShaByName(tagImport)
	assert.Nil(t, err)
	content, err = ioutil.ReadFile(dir + "/foobar")
	assert.Equal(t, string(content), randomData)

	assert.Equal(t, shaExport1, shaExport2)
	assert.Equal(t, shaImport1, shaImport2)
}
