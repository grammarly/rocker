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

// URLCache is a persistent cache for downloadable urls
type URLCache interface {
	Get(string) (*URLInfo, error)
	GetInfo(string) (*URLInfo, error)
}

// URLCacheFS is an FS-backed URLCache implementation
type URLCacheFS struct {
	cacheDir string
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
	Cache    *URLCacheFS `json:"-"`
}

func makeURLCacheFS(base string) (cache *URLCacheFS, err error) {
	cacheDir := filepath.Join(base, "download_cache")

	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return nil, err
	}

	return &URLCacheFS{cacheDir}, nil
}

// GetInfo retrieves stored URLInfo data
func (uc *URLCacheFS) GetInfo(url0 string) (info *URLInfo, err error) {
	info, ok, err := uc.getURLInfo(url0)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("no url found in cache: `%s`", url0)
	}

	return info, nil
}

// Get downloads url, storing it into cache
func (uc *URLCacheFS) Get(url0 string) (info *URLInfo, err error) {
	info, ok, err := uc.getURLInfo(url0)
	if err != nil {
		return nil, err
	}

	if ok && info.isEtagValid() {
		return info, nil
	}

	err = info.download()
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (uc *URLCacheFS) getURLInfo(url0 string) (info *URLInfo, ok bool, err error) {
	info, err = uc.makeURLInfo(url0)
	if err != nil {
		return nil, false, err
	}

	ok, err = info.load()
	if err != nil {
		return nil, false, err
	}

	return info, ok, nil
}

func (uc *URLCacheFS) makeURLInfo(u string) (info *URLInfo, err error) {
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
		Cache:    uc,
	}
	info.FileName = info.getBlobFileName()
	return info, nil
}

func (uc *URLCacheFS) makeID(u string) (id string) {
	h := sha256.Sum256([]byte(u))
	id = fmt.Sprintf("%x", h)
	return id
}

func isURL(u string) bool {
	return (7 <= len(u) && u[:7] == "http://") ||
		(8 <= len(u) && u[:8] == "https://")
}

func (info *URLInfo) getBlobFileName() (fileName string) {
	return filepath.Join(info.Cache.cacheDir, info.ID[:2], info.ID)
}

func (info *URLInfo) getInfoFileName() (fileName string) {
	return info.getBlobFileName() + ".json"
}

func (info *URLInfo) isEtagValid() bool {
	if !info.HasEtag {
		return false
	}

	return false
}

func (info *URLInfo) download() (err error) {
	log.Infof("Downloading `%s` into `%s`", info.URL, info.FileName)

	response, err := http.Get(info.URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || 300 <= response.StatusCode {
		return fmt.Errorf("Got non-2xx status for `%s`: %s", info.URL, response.Status)
	}

	err = os.MkdirAll(filepath.Dir(info.FileName), 0755)
	if err != nil {
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

	if etag, ok := response.Header["ETag"]; ok {
		info.HasEtag = true
		info.Etag = etag[0]
	}

	info.store()

	return nil
}

func (info *URLInfo) load() (ok bool, err error) {
	fileName := info.getInfoFileName()

	data, err := ioutil.ReadFile(fileName)

	if err != nil {
		if pe, ok0 := err.(*os.PathError); ok0 && pe.Err == os.ErrNotExist {
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
