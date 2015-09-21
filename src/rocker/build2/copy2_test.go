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

package build2

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/docker/docker/pkg/tarsum"
	"github.com/kr/pretty"
)

func TestMakeTarStream_Basic(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	includes := []string{
		"testdata/*.txt",
	}
	excludes := []string{
		"testdata/*.tar",
		"testdata/*.txt2",
	}

	stream, err := makeTarStream(wd, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}
	data, err := ioutil.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}

	tarSum, err := tarsum.NewTarSum(bytes.NewReader(data), true, tarsum.Version1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(ioutil.Discard, tarSum); err != nil {
		t.Fatal(err)
	}
	println("tarsum:" + tarSum.Sum(nil))

	if err := ioutil.WriteFile("testdata/file.tar", data, 0644); err != nil {
		t.Fatal(err)
	}

	println("Written to testdata/file.tar")

}

func TestExpandIncludes_Basic(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	includes := []string{
		"testdata",
	}
	excludes := []string{
		"testdata/*.tar",
		"testdata/*.txt2",
	}

	matches, err := expandIncludes(wd, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	pretty.Println(matches)
}
