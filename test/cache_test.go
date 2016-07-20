package tests

import (
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache_WithEnvVariables(t *testing.T) {
	tag := "rocker-integratin-test:1.2.3"
	defer removeImage(tag)

	err := runRockerBuild(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEqual(t, sha1, sha2, "Env variable should invalidate cache")
}
func TestCache_WorksByDefault(t *testing.T) {
	tag := "rocker-integratin-test:1.2.3"
	defer removeImage(tag)

	err := runRockerBuild(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, sha1, sha2, "same builds should be equal")
}

func TestCache_NoCache(t *testing.T) {
	tag := "rocker-integratin-test:1.2.3"
	defer removeImage(tag)

	err := runRockerBuild(`
FROM alpine
RUN touch /tmp/foo
TAG ` + tag)
	if err != nil {
		t.Fatal(err)
	}

	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuild(`
FROM alpine
RUN touch /tmp/foo
TAG `+tag, "--no-cache")
	if err != nil {
		t.Fatal(err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEqual(t, sha1, sha2, "--no-cache should not be idempotent")
}

func TestCache_FewSameCommands(t *testing.T) {
	tag := "rocker-integratin-test:1.8.35"
	defer removeImage(tag)
	scenario := `FROM alpine
				 RUN true
				 RUN true
				 RUN true
				 RUN true
				 RUN true
				 TAG ` + tag

	err := runRockerBuild(scenario, "--reload-cache")
	if err != nil {
		t.Fatal(err)
	}

	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuild(scenario)
	if err != nil {
		t.Fatal(err)
	}

	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, sha1, sha2, "few same commands should lead to same iamge sha")
}

func TestCache_FewDifferentCommands(t *testing.T) {
	tag := "rocker-integratin-test:1.8.36"
	defer removeImage(tag)
	scenario := `FROM alpine
				 RUN true
				 RUN echo "123" > /foobar
				 RUN ls > /dev/null
				 RUN date
				 TAG ` + tag

	err := runRockerBuild(scenario, "--reload-cache")
	if err != nil {
		t.Fatal(err)
	}
	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuild(scenario)
	if err != nil {
		t.Fatal(err)
	}
	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, sha1, sha2, "few different commands should lead to same iamge sha")
}

func TestCache_MountNotCached(t *testing.T) {
	tag := "rocker-integratin-test:mount_not_cached"
	defer removeImage(tag)
	scenario1 := `FROM alpine
				 TAG ` + tag
	scenario2 := `FROM alpine
				 MOUNT /tmp:/tmp
				 TAG ` + tag

	err := runRockerBuild(scenario1, "--reload-cache")
	if err != nil {
		t.Fatal(err)
	}
	sha1, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	err = runRockerBuild(scenario2)
	if err != nil {
		t.Fatal(err)
	}
	sha2, err := getImageShaByName(tag)
	if err != nil {
		t.Fatal(err)
	}

	//Despite the `MOUNT` isn't "commited" command it still will invalidate the cache
	assert.NotEqual(t, sha1, sha2, "MOUNT should invalidate cache")
}

func TestCache_AndExportImport(t *testing.T) {
	tagExport := "rocker-integratin-test:export"
	tagImport := "rocker-integratin-test:import"
	defer removeImage(tagExport)
	defer removeImage(tagImport)

	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_")
	if err != nil {
		t.Fatal("Can't create tmp dir:", err)
	}
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

	err = runRockerBuild(scenario, "--reload-cache")
	if err != nil {
		t.Fatal(err)
	}
	shaExport1, err := getImageShaByName(tagExport)
	if err != nil {
		t.Fatal(err)
	}
	shaImport1, err := getImageShaByName(tagImport)
	if err != nil {
		t.Fatal(err)
	}
	content, err := ioutil.ReadFile(dir + "/foobar")
	assert.Equal(t, string(content), randomData)

	err = runRockerBuild(scenario)
	if err != nil {
		t.Fatal(err)
	}
	shaExport2, err := getImageShaByName(tagExport)
	if err != nil {
		t.Fatal(err)
	}
	shaImport2, err := getImageShaByName(tagImport)
	if err != nil {
		t.Fatal(err)
	}
	content, err = ioutil.ReadFile(dir + "/foobar")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, string(content), randomData)
	assert.Equal(t, shaExport1, shaExport2, "Export doesn't match")
	assert.Equal(t, shaImport1, shaImport2, "Import doesn't match")
}
