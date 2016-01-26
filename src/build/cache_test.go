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
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCache_Basic(t *testing.T) {
	tmpDir := cacheTestTmpDir(t)
	defer os.RemoveAll(tmpDir)

	c := NewCacheFS(tmpDir)

	s := State{
		ParentID: "123",
		ImageID:  "456",
	}
	if err := c.Put(s); err != nil {
		t.Fatal(err)
	}

	s2 := State{
		ImageID: "123",
	}
	res, err := c.Get(s2)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "456", res.ImageID)

	s3 := State{
		ImageID: "789",
	}
	res2, err := c.Get(s3)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, res2)
}

func cacheTestTmpDir(t *testing.T) string {
	tmpDir, err := ioutil.TempDir("", "rocker-cache-test")
	if err != nil {
		t.Fatal(err)
	}
	return tmpDir
}
