package tests

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/grammarly/rocker/src/test"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

func TestAddUrl_Basic(t *testing.T) {

	sampleDir := makeTmpDir(t, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "hello",
		"file3.txt": "content2",
	})
	defer os.RemoveAll(sampleDir)

	files := FM{
		"/file1.txt": func(r *http.Request) respTuple {
			return respTuple{200, HM{"Etag": "AAA"}, "content1"}
		},
		"/file3.txt": func(r *http.Request) respTuple {
			return respTuple{200, HM{"Etag": "CCC"}, "content2"}
		},
	}

	server := makeTestServer(files)
	defer server.Close()

	Rockerfile := `
FROM alpine
ADD %s/file1.txt file2.txt %s/file3.txt /dst/
MOUNT ` + sampleDir + `:/sample
RUN diff -r /dst /sample
`
	Rockerfile = fmt.Sprintf(Rockerfile, server.URL, server.URL)

	cacheDir := makeTmpDir(t, map[string]string{})
	defer os.RemoveAll(cacheDir)

	buildDir := makeTmpDir(t, map[string]string{
		"Rockerfile": Rockerfile,
		"file2.txt":  "hello",
	})
	defer os.RemoveAll(buildDir)

	err := runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir)
	assert.Nil(t, err, "no difference in downloaded and added files")
}

func TestAddUrl_BuildCacheHit(t *testing.T) {

	sampleDir := makeTmpDir(t, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "hello",
		"file3.txt": "content3",
	})
	defer os.RemoveAll(sampleDir)

	file1Hits := 0
	file3Hits := 0

	files := FM{
		"/file1.txt": func(req *http.Request) (resp respTuple) {
			resp = respTuple{200, HM{}, "content1"}
			if req.Method != "HEAD" {
				file1Hits++
			}
			return resp
		},
		"/file3.txt": func(req *http.Request) (resp respTuple) {
			resp = respTuple{200, HM{}, "content3"}
			if req.Method != "HEAD" {
				file3Hits++
			}
			return resp
		},
	}

	server := makeTestServer(files)
	defer server.Close()

	Rockerfile := `
FROM alpine
ADD %s/file1.txt file2.txt %s/file3.txt /dst/
MOUNT ` + sampleDir + `:/sample
RUN diff -r /dst /sample
`
	Rockerfile = fmt.Sprintf(Rockerfile, server.URL, server.URL)

	cacheDir := makeTmpDir(t, map[string]string{})
	defer os.RemoveAll(cacheDir)

	buildDir := makeTmpDir(t, map[string]string{
		"Rockerfile": Rockerfile,
		"file2.txt":  "hello",
	})
	defer os.RemoveAll(buildDir)

	var err error
	err = runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir)
	assert.Nil(t, err, "no difference in downloaded and added files")
	assert.Equal(t, 1, file1Hits, "file1 dowloaded at the time of first build")
	assert.Equal(t, 1, file3Hits, "file1 dowloaded at the time of first build")

	// build container again
	err = runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir)
	assert.Nil(t, err, "no difference in downloaded and added files")
	assert.Equal(t, 2, file1Hits, "file1 dowloaded at the time of second build")
	assert.Equal(t, 2, file3Hits, "file1 dowloaded at the time of second build")

	// XXX ensure cache isn't invalidated
}

func TestAddUrl_CacheHit(t *testing.T) {

	sampleDir := makeTmpDir(t, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "hello",
		"file3.txt": "content3",
	})
	//defer os.RemoveAll(sampleDir)

	file1Hits := 0
	file3Hits := 0

	files := FM{
		"/file1.txt": func(req *http.Request) (resp respTuple) {
			resp = respTuple{200, HM{"Etag": "AAA"}, "content1"}
			if req.Method != "HEAD" {
				file1Hits++
			}
			return resp
		},
		"/file3.txt": func(req *http.Request) (resp respTuple) {
			resp = respTuple{200, HM{"Etag": "BBB"}, "content3"}
			if req.Method != "HEAD" {
				file3Hits++
			}
			return resp
		},
	}

	server := makeTestServer(files)
	defer server.Close()

	Rockerfile := `
FROM alpine
ADD %s/file1.txt file2.txt %s/file3.txt /dst/
MOUNT ` + sampleDir + `:/sample
RUN diff -r /dst /sample
`
	Rockerfile = fmt.Sprintf(Rockerfile, server.URL, server.URL)

	cacheDir := makeTmpDir(t, map[string]string{})
	defer os.RemoveAll(cacheDir)

	buildDir := makeTmpDir(t, map[string]string{
		"Rockerfile": Rockerfile,
		"file2.txt":  "hello",
	})
	defer os.RemoveAll(buildDir)

	var err error
	err = runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir)
	assert.Nil(t, err, "no difference in downloaded and added files")
	assert.Equal(t, 1, file1Hits, "file1 dowloaded at the time of first build")
	assert.Equal(t, 1, file3Hits, "file1 dowloaded at the time of first build")

	// build container again
	err = runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir)
	assert.Nil(t, err, "no difference in downloaded and added files")
	assert.Equal(t, 1, file1Hits, "file1 isn't dowloaded at the time of second build")
	assert.Equal(t, 1, file3Hits, "file3 isn't dowloaded at the time of second build")

	// XXX ensure cache isn't invalidated
}

func TestAddUrl_CacheMiss(t *testing.T) {

	file1Hits := 0
	file3Hits := 0

	files := FM{
		"/file1.txt": func(req *http.Request) (resp respTuple) {
			if file1Hits < 1 {
				resp = respTuple{200, HM{"Etag": "AAA"}, "content11"}
			} else {
				resp = respTuple{200, HM{"Etag": "BBB"}, "content12"}
			}

			if req.Method != "HEAD" {
				file1Hits++
			}
			return resp
		},
		"/file3.txt": func(req *http.Request) (resp respTuple) {
			if file3Hits < 1 {
				resp = respTuple{200, HM{"Etag": "CCC"}, "content31"}
			} else {
				resp = respTuple{200, HM{"Etag": "DDD"}, "content32"}
			}

			if req.Method != "HEAD" {
				file3Hits++
			}
			return resp
		},
	}

	server := makeTestServer(files)
	defer server.Close()

	sampleDir := makeTmpDir(t, map[string]string{
		"file1.txt": "content11",
		"file2.txt": "hello",
		"file3.txt": "content31",
	})
	defer os.RemoveAll(sampleDir)

	Rockerfile := `
FROM alpine
ADD %s/file1.txt file2.txt %s/file3.txt /dst/
MOUNT ` + sampleDir + `:/sample
RUN diff -r /dst /sample
`
	Rockerfile = fmt.Sprintf(Rockerfile, server.URL, server.URL)

	cacheDir := makeTmpDir(t, map[string]string{})
	defer os.RemoveAll(cacheDir)

	buildDir := makeTmpDir(t, map[string]string{
		"Rockerfile": Rockerfile,
		"file2.txt":  "hello",
	})
	defer os.RemoveAll(buildDir)

	var err error
	err = runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir)
	assert.Nil(t, err, "no difference in downloaded and added files")
	assert.Equal(t, 1, file1Hits, "file1 dowloaded at the time of first build")
	assert.Equal(t, 1, file3Hits, "file1 dowloaded at the time of first build")

	for file, content := range map[string]string{"file1.txt": "content12", "file3.txt": "content32"} {
		ioutil.WriteFile(filepath.Join(sampleDir, file), []byte(content), 0644)
	}

	// build container again
	err = runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir)
	assert.Nil(t, err, "no difference in downloaded and added files")
	assert.Equal(t, 2, file1Hits, "file1 dowloaded at the time of second build")
	assert.Equal(t, 2, file3Hits, "file3 dowloaded at the time of second build")

	// XXX ensure cache isn't invalidated

}

func TestAddUrl_NoCache(t *testing.T) {

	sampleDir := makeTmpDir(t, map[string]string{
		"file1.txt": "content1",
		"file2.txt": "hello",
		"file3.txt": "content3",
	})
	//defer os.RemoveAll(sampleDir)

	file1Hits := 0
	file3Hits := 0

	files := FM{
		"/file1.txt": func(req *http.Request) (resp respTuple) {
			resp = respTuple{200, HM{"Etag": "AAA"}, "content1"}
			if req.Method != "HEAD" {
				file1Hits++
			}
			return resp
		},
		"/file3.txt": func(req *http.Request) (resp respTuple) {
			resp = respTuple{200, HM{"Etag": "BBB"}, "content3"}
			if req.Method != "HEAD" {
				file3Hits++
			}
			return resp
		},
	}

	server := makeTestServer(files)
	defer server.Close()

	Rockerfile := `
FROM alpine
ADD %s/file1.txt file2.txt %s/file3.txt /dst/
MOUNT ` + sampleDir + `:/sample
RUN diff -r /dst /sample
`
	Rockerfile = fmt.Sprintf(Rockerfile, server.URL, server.URL)

	cacheDir := makeTmpDir(t, map[string]string{})
	defer os.RemoveAll(cacheDir)

	buildDir := makeTmpDir(t, map[string]string{
		"Rockerfile": Rockerfile,
		"file2.txt":  "hello",
	})
	defer os.RemoveAll(buildDir)

	var err error
	err = runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir)
	assert.Nil(t, err, "no difference in downloaded and added files")
	assert.Equal(t, 1, file1Hits, "file1 dowloaded at the time of first build")
	assert.Equal(t, 1, file3Hits, "file1 dowloaded at the time of first build")

	// build container again
	err = runRockerBuildWdWithOptions(buildDir, "-cache-dir", cacheDir, "-no-cache")
	assert.Nil(t, err, "no difference in downloaded and added files")
	assert.Equal(t, 2, file1Hits, "file1 dowloaded at the time of second build")
	assert.Equal(t, 2, file3Hits, "file3 dowloaded at the time of second build")

	// XXX ensure cache is invalidated

}

type HM map[string]string

type respTuple struct {
	code    int
	headers HM
	body    string
}

type FM map[string]func(r *http.Request) respTuple

func makeTestServer(files FM) *httptest.Server {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		F, ok := files[r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			fmt.Fprint(w, "not found")
			return
		}

		R := F(r)

		for k, v := range R.headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(R.code)
		fmt.Fprint(w, R.body)
	}))

	return server
}

func makeTmpDir(t *testing.T, files map[string]string) string {
	tmpDir, err := ioutil.TempDir("", "rocker-copy-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := test.MakeFiles(tmpDir, files); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	t.Logf("temp directory: %s", tmpDir)
	t.Logf("  with files: %# v", pretty.Formatter(files))
	return tmpDir
}
