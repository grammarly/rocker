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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

func TestURLFetcher_Get_Basic(t *testing.T) {

	tmpDir := makeTmpDir(t, map[string]string{})
	defer os.RemoveAll(tmpDir)

	headers := map[string]string{
		"Etag": "A",
	}

	content := "content"

	server, client := makeTestHttpResponse(200, headers, content)
	defer server.Close()

	urlFetcher := NewURLFetcherFS(tmpDir, false, client)

	ui, err := urlFetcher.Get("http://someurl/test123.txt")

	if err != nil {
		t.Logf("fetch error: %# v", pretty.Formatter(err))
	}
	s, err := ui.dump()
	if err != nil {
		t.Logf("unable to dump json: %s", err)
	}
	t.Logf("UrlInfo: %s", s)

	data, err := ioutil.ReadFile(ui.FileName)

	assert.Equal(t, content, string(data), "downloaded and read data should match")

	ui1, err := urlFetcher.GetInfo("http://someurl/test123.txt")

	assert.Equal(t, ui1.Etag, "A", "stored info etag should match that of downloaded url")
}

func TestURLFetcher_Get_CacheHit() {}

func TestURLFetcher_GetInfo_NoEntry() {}

func TestURLFetcher_getURLInfo_NoEntry() {}

// if there's no etag in response, subsequent request goes for download
func TestURLFetcher_Get_CacheMiss() {}

// 404 and other non-2xx codes produce no cache content, return download error
func TestURLFetcher_Get_HttpError() {}

func makeTestHttpResponse(code int, headers map[string]string, body string) (*httptest.Server, *http.Client) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}))

	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL)
		},
	}

	httpClient := &http.Client{Transport: transport}

	return server, httpClient
}
