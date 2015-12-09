// +build integration

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

// Run the test like this:
// GOPATH=`pwd`:`pwd`/vendor go test -v rocker/storage/s3 -tags="integration"

package s3

import (
	"os"
	"rocker/dockerclient"
	"testing"

	"github.com/kr/pretty"
)

func TestStorageS3_Basic(t *testing.T) {
	client, err := dockerclient.New()
	if err != nil {
		t.Fatal(err)
	}

	s := New(client)

	tmpf, digest, err := s.MakeTar("alpine-s3:0.2")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpf)

	pretty.Println(digest, tmpf)
}
