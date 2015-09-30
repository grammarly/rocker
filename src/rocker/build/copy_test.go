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
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"rocker/test"
	"strings"
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"

	"github.com/docker/docker/pkg/tarsum"
)

func TestCopy_ListFiles_Basic(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"file1.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"file1.txt",
	}
	excludes := []string{}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/file1.txt", "file1.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Wildcard(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"file1.txt": "hello",
		"file2.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"*.txt",
	}
	excludes := []string{}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/file1.txt", "file1.txt"},
		{tmpDir + "/file2.txt", "file2.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Dir_Simple(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"dir/foo.txt": "hello",
		"dir/bar.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"dir",
	}
	excludes := []string{}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/dir/bar.txt", "dir/bar.txt"},
		{tmpDir + "/dir/foo.txt", "dir/foo.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Dir_AndFiles(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"test.txt":    "hello",
		"dir/foo.txt": "hello",
		"dir/bar.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		".",
	}
	excludes := []string{}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/dir/bar.txt", "dir/bar.txt"},
		{tmpDir + "/dir/foo.txt", "dir/foo.txt"},
		{tmpDir + "/test.txt", "test.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Dir_Multi(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"a/test.txt": "hello",
		"b/1.txt":    "hello",
		"b/2.txt":    "hello",
		"c/foo.txt":  "hello",
		"c/x/1.txt":  "hello",
		"c/x/2.txt":  "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"a",
		"b/2.txt",
		"c",
	}
	excludes := []string{}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/a/test.txt", "a/test.txt"},
		{tmpDir + "/b/2.txt", "2.txt"},
		{tmpDir + "/c/foo.txt", "c/foo.txt"},
		{tmpDir + "/c/x/1.txt", "c/x/1.txt"},
		{tmpDir + "/c/x/2.txt", "c/x/2.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Excludes_Basic(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"test1.txt": "hello",
		"test2.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"*.txt",
	}
	excludes := []string{
		"test2.txt",
	}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/test1.txt", "test1.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Excludes_Explicit(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"test1.txt": "hello",
		"test2.txt": "hello",
		"test3.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"test2.txt",
	}
	excludes := []string{
		"*.txt",
	}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/test2.txt", "test2.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Excludes_Exception(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"test1.txt": "hello",
		"test2.txt": "hello",
		"test3.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"*",
	}
	excludes := []string{
		"*.txt",
		"!test2.txt",
	}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/test2.txt", "test2.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Excludes_Dir(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"a/test1.txt": "hello",
		"b/test2.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		".",
	}
	excludes := []string{
		"b",
	}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/a/test1.txt", "a/test1.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_ListFiles_Excludes_FileInAnyDir(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"a/test1.txt":     "hello",
		"b/test2.txt":     "hello",
		"c/d/e/test2.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		".",
	}
	excludes := []string{
		"**/test2.txt",
	}

	matches, err := listFiles(tmpDir, includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("matches: %# v", pretty.Formatter(matches))

	assertions := [][2]string{
		{tmpDir + "/a/test1.txt", "a/test1.txt"},
	}

	assert.Len(t, matches, len(assertions))
	for i, a := range assertions {
		assert.Equal(t, a[0], matches[i].src, "bad match src at index %d", i)
		assert.Equal(t, a[1], matches[i].dest, "bad match dest at index %d", i)
	}
}

func TestCopy_MakeTarStream_Basic(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"a/test.txt": "hello",
		"b/1.txt":    "hello",
		"b/2.txt":    "hello",
		"c/foo.txt":  "hello",
		"c/x/1.txt":  "hello",
		"c/x/2.txt":  "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"a",
		"b/2.txt",
		"c",
	}
	excludes := []string{}
	dest := "/"

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("dest: %# v", pretty.Formatter(dest))

	stream, err := makeTarStream(tmpDir, dest, "COPY", includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	out := writeReadTar(t, tmpDir, stream.tar)

	assertion := strings.Join([]string{
		"a/test.txt",
		"2.txt",
		"c/foo.txt",
		"c/x/1.txt",
		"c/x/2.txt",
	}, "\n") + "\n"

	assert.Equal(t, assertion, out, "bad tar content")
}

func TestCopy_MakeTarStream_FileRename(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"a/test.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"./a/test.txt",
	}
	excludes := []string{}
	dest := "/src/x.txt"

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("dest: %# v", pretty.Formatter(dest))

	stream, err := makeTarStream(tmpDir, dest, "COPY", includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	out := writeReadTar(t, tmpDir, stream.tar)

	assertion := strings.Join([]string{
		"src/x.txt",
	}, "\n") + "\n"

	assert.Equal(t, assertion, out, "bad tar content")
}

func TestCopy_MakeTarStream_OneFileToDir(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"a/test.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		"a/test.txt",
	}
	excludes := []string{}
	dest := "/src/"

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("dest: %# v", pretty.Formatter(dest))

	stream, err := makeTarStream(tmpDir, dest, "COPY", includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	out := writeReadTar(t, tmpDir, stream.tar)

	assertion := strings.Join([]string{
		"src/test.txt",
	}, "\n") + "\n"

	assert.Equal(t, assertion, out, "bad tar content")
}

func TestCopy_MakeTarStream_CurrentDir(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"a/test.txt": "hello",
		"b/1.txt":    "hello",
		"b/2.txt":    "hello",
		"c/foo.txt":  "hello",
		"c/x/1.txt":  "hello",
		"c/x/2.txt":  "hello",
	})
	defer os.RemoveAll(tmpDir)

	includes := []string{
		".",
	}
	excludes := []string{}
	dest := "/go/app/src"

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("dest: %# v", pretty.Formatter(dest))

	stream, err := makeTarStream(tmpDir, dest, "COPY", includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	out := writeReadTar(t, tmpDir, stream.tar)

	assertion := strings.Join([]string{
		"go/app/src/a/test.txt",
		"go/app/src/b/1.txt",
		"go/app/src/b/2.txt",
		"go/app/src/c/foo.txt",
		"go/app/src/c/x/1.txt",
		"go/app/src/c/x/2.txt",
	}, "\n") + "\n"

	assert.Equal(t, assertion, out, "bad tar content")
}

func TestCopy_MakeTarStream_DirRename(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"c/foo.txt": "hello",
		"c/x/1.txt": "hello",
		"c/x/2.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	// ADD ./c /src --> /src
	// ADD ./a/b[/1,2] /src -> /src[/1,2]

	includes := []string{
		"./c",
	}
	excludes := []string{}
	dest := "/src"

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("dest: %# v", pretty.Formatter(dest))

	stream, err := makeTarStream(tmpDir, dest, "COPY", includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	out := writeReadTar(t, tmpDir, stream.tar)

	assertion := strings.Join([]string{
		"src/foo.txt",
		"src/x/1.txt",
		"src/x/2.txt",
	}, "\n") + "\n"

	assert.Equal(t, assertion, out, "bad tar content")
}

func TestCopy_MakeTarStream_DirRenameLeadingSlash(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"c/foo.txt": "hello",
		"c/x/1.txt": "hello",
		"c/x/2.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	// ADD ./c /src --> /src
	// ADD ./a/b[/1,2] /src -> /src[/1,2]

	includes := []string{
		"./c/",
	}
	excludes := []string{}
	dest := "/src"

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("dest: %# v", pretty.Formatter(dest))

	stream, err := makeTarStream(tmpDir, dest, "COPY", includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	out := writeReadTar(t, tmpDir, stream.tar)

	assertion := strings.Join([]string{
		"src/foo.txt",
		"src/x/1.txt",
		"src/x/2.txt",
	}, "\n") + "\n"

	assert.Equal(t, assertion, out, "bad tar content")
}

func TestCopy_MakeTarStream_DirRenameWildcard(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"c/foo.txt": "hello",
		"c/x/1.txt": "hello",
		"c/x/2.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	// ADD ./c /src --> /src
	// ADD ./a/b[/1,2] /src -> /src[/1,2]

	includes := []string{
		"*",
	}
	excludes := []string{}
	dest := "/src"

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("dest: %# v", pretty.Formatter(dest))

	stream, err := makeTarStream(tmpDir, dest, "COPY", includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	out := writeReadTar(t, tmpDir, stream.tar)

	assertion := strings.Join([]string{
		"src/c/foo.txt",
		"src/c/x/1.txt",
		"src/c/x/2.txt",
	}, "\n") + "\n"

	assert.Equal(t, assertion, out, "bad tar content")
}

func TestCopy_MakeTarStream_SingleFileDirRename(t *testing.T) {
	tmpDir := makeTmpDir(t, map[string]string{
		"c/foo.txt": "hello",
	})
	defer os.RemoveAll(tmpDir)

	// ADD ./c /src --> /src
	// ADD ./a/b[/1,2] /src -> /src[/1,2]

	includes := []string{
		"./c",
	}
	excludes := []string{}
	dest := "/src"

	t.Logf("includes: %# v", pretty.Formatter(includes))
	t.Logf("excludes: %# v", pretty.Formatter(excludes))
	t.Logf("dest: %# v", pretty.Formatter(dest))

	stream, err := makeTarStream(tmpDir, dest, "COPY", includes, excludes)
	if err != nil {
		t.Fatal(err)
	}

	out := writeReadTar(t, tmpDir, stream.tar)

	assertion := strings.Join([]string{
		"src/foo.txt",
	}, "\n") + "\n"

	assert.Equal(t, assertion, out, "bad tar content")
}

// TODO:
// WORKDIR /app
// COPY lib lib/
// should copy to /app/lib

// helper functions

func makeTmpDir(t *testing.T, files map[string]string) string {
	tmpDir, err := ioutil.TempDir("", "rocker-copy-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := test.MakeFiles(tmpDir, files); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatal(err)
	}
	t.Logf("temp files: %# v", pretty.Formatter(files))
	return tmpDir
}

func writeReadTar(t *testing.T, tmpDir string, tarStream io.ReadCloser) string {
	data, err := ioutil.ReadAll(tarStream)
	if err != nil {
		t.Fatal(err)
	}
	defer tarStream.Close()

	tarSum, err := tarsum.NewTarSum(bytes.NewReader(data), true, tarsum.Version1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(ioutil.Discard, tarSum); err != nil {
		t.Fatal(err)
	}
	t.Logf("tarsum: %s", tarSum.Sum(nil))

	if err := ioutil.WriteFile(tmpDir+"/archive.tar", data, 0644); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir + "/archive.tar")

	cmd := exec.Command("tar", "-tf", tmpDir+"/archive.tar")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	return string(out)
}
