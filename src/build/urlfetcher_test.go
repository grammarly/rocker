/*-
 * Copyright 2015 Grammarly, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package build

import (
	"fmt"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func TestURLFetcher_Get_Basic(t *testing.T) {

	tf := makeTempFetcher(t, false)
	defer tf.cleanup()

	tf.files["/file1.txt"] = func(r *http.Request) respTuple {
		return respTuple{200, HM{"Etag": "AAA"}, "content1"}
	}

	ui0, err := tf.fetcher.Get("http://someurl/file1.txt")
	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}
	s, err := ui0.dump()
	if err != nil {
		t.Logf("unable to dump json: %s", err)
	}
	t.Logf("UrlInfo: %s", s)

	data, err := ioutil.ReadFile(ui0.FileName)
	assert.Nil(t, err, "unable to read file contents")
	assert.Equal(t, "content1", string(data), "downloaded and read data should match")

	ui1, err := tf.fetcher.GetInfo("http://someurl/file1.txt")

	assert.Nil(t, err, "unable to read info")
	assert.Equal(t, ui1.Etag, "AAA", "stored info etag should match that of downloaded url")
}

func TestURLFetcher_Get_CacheHit(t *testing.T) {
	tf := makeTempFetcher(t, false)
	defer tf.cleanup()

	var hits = 0

	tf.files["/file1.txt"] = func(r *http.Request) respTuple {
		if r.Method == "HEAD" {
			return respTuple{200, HM{"Etag": "AAA"}, ""}
		}
		hits++
		return respTuple{200, HM{"Etag": "AAA"}, "content1"}
	}

	ui0, err := tf.fetcher.Get("http://someurl/file1.txt")
	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}
	s, err := ui0.dump()
	if err != nil {
		t.Logf("unable to dump json: %s", err)
	}
	t.Logf("UrlInfo: %s", s)

	data, err := ioutil.ReadFile(ui0.FileName)
	assert.Nil(t, err, "unable to read file contents")
	assert.Equal(t, "content1", string(data), "downloaded and read data should match")
	assert.Equal(t, hits, 1, "1st Get call should actually download file")

	_, err = tf.fetcher.Get("http://someurl/file1.txt")
	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}

	ui2, err := tf.fetcher.GetInfo("http://someurl/file1.txt")
	assert.Nil(t, err, "unable to read info")
	assert.Equal(t, ui2.Etag, "AAA", "stored info etag should match that of downloaded url")
	assert.Equal(t, hits, 1, "2nd Get should not actually download file")

}

// func TestURLFetcher_GetInfo_NoEntry() {}
// func TestURLFetcher_getURLInfo_NoEntry() {}

// if there's non-matching etag in response, subsequent request goes for download
func TestURLFetcher_Get_CacheMissEtagChanged(t *testing.T) {
	tf := makeTempFetcher(t, false)
	defer tf.cleanup()

	var hits = 0

	tf.files["/file1.txt"] = func(r *http.Request) respTuple {
		hits++
		if hits <= 1 {
			return respTuple{200, HM{"Etag": "AAA"}, "content1"}
		}
		if r.Method == "HEAD" {
			return respTuple{200, HM{"Etag": "BBB"}, ""}
		}
		return respTuple{200, HM{"Etag": "BBB"}, "content2"}
	}

	ui0, err := tf.fetcher.Get("http://someurl/file1.txt")
	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}
	s, err := ui0.dump()
	if err != nil {
		t.Logf("unable to dump json: %s", err)
	}
	t.Logf("UrlInfo: %s", s)
	assert.Equal(t, 1, hits, "1st Get call should actually download file")

	data, err := ioutil.ReadFile(ui0.FileName)
	assert.Nil(t, err, "unable to read file contents")
	assert.Equal(t, "content1", string(data), "downloaded and read data should match")

	_, err = tf.fetcher.Get("http://someurl/file1.txt")
	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}

	ui2, err := tf.fetcher.GetInfo("http://someurl/file1.txt")
	assert.Nil(t, err, "unable to read info")

	s, err = ui2.dump()
	if err != nil {
		t.Logf("unable to dump json: %s", err)
	}
	t.Logf("UrlInfo: %s", s)

	assert.Equal(t, "BBB", ui2.Etag, "stored info etag should match that of downloaded url")
	assert.Equal(t, 3, hits, "2nd Get should actually download file")

	data, err = ioutil.ReadFile(ui2.FileName)
	assert.Nil(t, err, "unable to read file contents")
	assert.Equal(t, "content2", string(data), "downloaded and read data should match")
}

// if there's no etag in response, subsequent request goes for download
func TestURLFetcher_Get_CacheMissNoEtag(t *testing.T) {
	tf := makeTempFetcher(t, false)
	defer tf.cleanup()

	var hits = 0

	tf.files["/file1.txt"] = func(r *http.Request) respTuple {
		hits++
		if hits < 2 {
			if r.Method == "HEAD" {
				return respTuple{200, HM{"Etag": "AAA"}, ""}
			}
			return respTuple{200, HM{"Etag": "AAA"}, "content1"}
		}
		if r.Method == "HEAD" {
			return respTuple{200, HM{}, ""}
		}
		return respTuple{200, HM{}, "content2"}
	}

	// XXX first download
	ui0, err := tf.fetcher.Get("http://someurl/file1.txt")
	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}
	s, err := ui0.dump()
	if err != nil {
		t.Logf("unable to dump json: %s", err)
	}
	t.Logf("UrlInfo: %s", s)
	assert.Equal(t, 1, hits, "1st Get call should actually download file")

	data, err := ioutil.ReadFile(ui0.FileName)
	assert.Nil(t, err, "unable to read file contents")
	assert.Equal(t, "content1", string(data), "downloaded and read data should match")

	// XXX second download
	_, err = tf.fetcher.Get("http://someurl/file1.txt")
	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}

	ui2, err := tf.fetcher.GetInfo("http://someurl/file1.txt")
	assert.Nil(t, err, "unable to read info")

	s, err = ui2.dump()
	if err != nil {
		t.Logf("unable to dump json: %s", err)
	}
	t.Logf("UrlInfo: %s", s)

	assert.Equal(t, false, ui2.HasEtag, "info should have no Etag")
	assert.Equal(t, 3, hits, "2nd Get should actually download file")

	data, err = ioutil.ReadFile(ui2.FileName)
	assert.Nil(t, err, "unable to read file contents")
	assert.Equal(t, "content2", string(data), "downloaded and read data should match")
}

// 404 and other non-2xx codes return download error
func TestURLFetcher_Get_HttpError(t *testing.T) {

	tf := makeTempFetcher(t, false)
	defer tf.cleanup()

	var hits = 0

	tf.files["/file1.txt"] = func(r *http.Request) respTuple {
		hits++
		if hits <= 1 {
			if r.Method == "HEAD" {
				return respTuple{200, HM{"Etag": "AAA"}, ""}
			}
			return respTuple{200, HM{"Etag": "AAA"}, "content1"}
		}

		return respTuple{404, HM{}, ""}
	}

	ui0, err := tf.fetcher.Get("http://someurl/file1.txt")

	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}
	s, err := ui0.dump()
	if err != nil {
		t.Logf("unable to dump json: %s", err)
	}
	t.Logf("UrlInfo: %s", s)

	data, err := ioutil.ReadFile(ui0.FileName)
	assert.Nil(t, err, "unable to read file contents")
	assert.Equal(t, "content1", string(data), "downloaded and read data should match")
	assert.Equal(t, hits, 1, "1st Get call should actually download file")

	_, err = tf.fetcher.Get("http://someurl/file1.txt")
	assert.NotNil(t, err, "should receive 404 error")
}

type HM map[string]string

type respTuple struct {
	code    int
	headers HM
	body    string
}

type FM map[string]func(r *http.Request) respTuple

type testFetcher struct {
	tmpDir  string
	fetcher *URLFetcherFS
	server  *httptest.Server
	client  *http.Client
	files   FM
}

func makeTempFetcher(t *testing.T, noCache bool) *testFetcher {

	tmpDir := makeTmpDir(t, map[string]string{})

	files := FM{}

	server, client := makeTestHTTPPair(files)

	urlFetcher := NewURLFetcherFS(tmpDir, false, client)

	return &testFetcher{
		tmpDir:  tmpDir,
		fetcher: urlFetcher,
		server:  server,
		client:  client,
		files:   files,
	}
}

func (f *testFetcher) cleanup() {
	os.RemoveAll(f.tmpDir)
	f.server.Close()
}

func makeTestHTTPPair(files FM) (*httptest.Server, *http.Client) {

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

	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			url0, err := url.Parse(server.URL)
			url0.Path = req.URL.Path
			return url0, err
		},
	}

	httpClient := &http.Client{Transport: transport}

	return server, httpClient
}
