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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	log "github.com/Sirupsen/logrus"
)

// Cache interface describes a cache backend
type Cache interface {
	Get(s State) (s2 *State, err error)
	Put(s State) error
	Del(s State) error
}

// CacheFS implements file based cache backend
type CacheFS struct {
	root string
}

// NewCacheFS creates a file based cache backend
func NewCacheFS(root string) *CacheFS {
	return &CacheFS{
		root: root,
	}
}

// Get fetches cache
func (c *CacheFS) Get(s State) (res *State, err error) {
	pattern := filepath.Join(c.root, s.ImageID, "*.json")

	latestTime := time.Unix(0, 0)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("Failed to stat cache file %s, error: %s", path, err)
		}

		s2 := State{}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("Failed to read cache file %s content, error: %s", path, err)
		}
		if err := json.Unmarshal(data, &s2); err != nil {
			return nil, fmt.Errorf("Failed to parse cache file %s json, error: %s", path, err)
		}

		log.Debugf("CACHE COMPARE %s %s %q %q", s.ImageID, s2.ImageID, s.Commits, s2.Commits)

		if s.Equals(s2) && info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			res = &s2
		}
	}

	return
}

// Put stores cache
func (c *CacheFS) Put(s State) error {
	log.Debugf("CACHE PUT %s %s %q", s.ParentID, s.ImageID, s.Commits)

	fileName := filepath.Join(c.root, s.ParentID, s.ImageID) + ".json"
	if err := os.MkdirAll(filepath.Dir(fileName), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fileName, data, 0644)
}

// Del deletes cache
func (c *CacheFS) Del(s State) error {
	log.Debugf("CACHE DELETE %s %s %q", s.ParentID, s.ImageID, s.Commits)

	fileName := filepath.Join(c.root, s.ParentID, s.ImageID) + ".json"
	return os.RemoveAll(fileName)
}
