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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
)

// URLFetcher is an interface to fetch urls from internets
type URLFetcher interface {
	Get(url string) (*URLInfo, error)
	GetInfo(url string) (*URLInfo, error)
}

// URLFetcherFS is an URLFetcher backed by FS cache
type URLFetcherFS struct {
	cacheDir string
	client   *http.Client
	noCache  bool
}

// URLInfo is a metadata representing stored or to-be-stored url
type URLInfo struct {
	ID       string
	URL      string
	FileName string `json:"-"`
	BaseName string
	HasEtag  bool
	Etag     string
	Size     int64
	Fetcher  *URLFetcherFS `json:"-"`
}

// NewURLFetcherFS returns an instance of URLFetcherFS, initialized to
// live in <base>/url_fetcher_cache
func NewURLFetcherFS(base string, noCache bool, httpClient *http.Client) (cache *URLFetcherFS) {
	cacheDir := filepath.Join(base, "url_fetcher_cache")

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &URLFetcherFS{
		cacheDir: cacheDir,
		client:   httpClient,
		noCache:  noCache,
	}
}

// GetInfo retrieves stored URLInfo data
func (uf *URLFetcherFS) GetInfo(url0 string) (info *URLInfo, err error) {
	info, ok, err := uf.getURLInfo(url0)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("no url found in cache: `%s`", url0)
	}

	return info, nil
}

// Get downloads url, stores file and metadata in cache
func (uf *URLFetcherFS) Get(url0 string) (info *URLInfo, err error) {
	info, ok, err := uf.getURLInfo(url0)
	if err != nil {
		return nil, err
	}

	if !uf.noCache && ok {

		log.Debugf("Validating %s [%s]", info.URL, info.FileName)

		if info.isEtagValid() {
			log.Debugf("%s valid!", info.URL)

			return info, nil
		}
	}

	if err = info.download(); err != nil {
		return nil, err
	}

	return info, nil
}

func (uf *URLFetcherFS) getURLInfo(url0 string) (info *URLInfo, ok bool, err error) {
	info, err = uf.makeURLInfo(url0)
	if err != nil {
		return nil, false, err
	}

	ok, err = info.load()
	if err != nil {
		return nil, false, err
	}

	return info, ok, nil
}

func (uf *URLFetcherFS) makeURLInfo(u string) (info *URLInfo, err error) {
	if !isURL(u) {
		return nil, fmt.Errorf("expecting http:// or https:// url, got `%s` instead", u)
	}

	u1, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	baseName := filepath.Base(u1.Path)

	if baseName == "" {
		return nil, fmt.Errorf("unable to determine filename from url: %s", u)
	}

	h := sha256.Sum256([]byte(u))
	id := fmt.Sprintf("%x", h)

	info = &URLInfo{
		ID:       id,
		URL:      u,
		BaseName: baseName,
		Fetcher:  uf,
	}
	info.FileName = info.getBlobFileName()
	return info, nil
}

func (uf *URLFetcherFS) makeID(u string) (id string) {
	h := sha256.Sum256([]byte(u))
	id = fmt.Sprintf("%x", h)
	return id
}

func isURL(u string) bool {
	return (7 <= len(u) && u[:7] == "http://") ||
		(8 <= len(u) && u[:8] == "https://")
}

func (info *URLInfo) getBlobFileName() (fileName string) {
	return filepath.Join(info.Fetcher.cacheDir, info.ID[:2], info.ID)
}

func (info *URLInfo) getInfoFileName() (fileName string) {
	return info.getBlobFileName() + ".json"
}

func (info *URLInfo) isEtagValid() bool {
	if !info.HasEtag {
		return false
	}

	httpClient := info.Fetcher.client

	response, err := httpClient.Head(info.URL)
	if err != nil {
		return false
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || 300 <= response.StatusCode {
		return false
	}

	if etag := response.Header.Get("Etag"); etag == info.Etag {
		return true
	}

	return false
}

func (info *URLInfo) download() (err error) {
	log.Infof("Downloading `%s` into `%s`", info.URL, info.FileName)

	httpClient := info.Fetcher.client

	response, err := httpClient.Get(info.URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || 300 <= response.StatusCode {
		return fmt.Errorf("Got non-2xx status for `%s`: %s", info.URL, response.Status)
	}

	if err = os.MkdirAll(filepath.Dir(info.FileName), 0755); err != nil {
		return err
	}

	f, err := os.Create(info.FileName)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := io.Copy(f, response.Body)
	if err != nil {
		return err
	}

	info.Size = n

	if etag := response.Header.Get("Etag"); etag != "" {
		info.HasEtag = true
		info.Etag = etag
	} else {
		info.HasEtag = false
		info.Etag = ""
	}

	if err = info.store(); err != nil {
		return err
	}

	return nil
}

func (info *URLInfo) load() (ok bool, err error) {
	fileName := info.getInfoFileName()

	data, err := ioutil.ReadFile(fileName)

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("Failed to read urlinfo file %s content, error: %s", fileName, err)
	}

	if err = json.Unmarshal(data, info); err != nil {
		return false, fmt.Errorf("Failed to parse urlinfo file %s json, error: %s", fileName, err)
	}

	return true, nil
}

func (info *URLInfo) store() (err error) {
	fileName := info.getInfoFileName()

	if err := os.MkdirAll(filepath.Dir(fileName), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fileName, data, 0644)
}

func (info *URLInfo) dump() (data string, err error) {
	data0, err := json.Marshal(info)
	return string(data0), err
}
