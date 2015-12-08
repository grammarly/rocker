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

package imagename

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/kr/pretty"

	"github.com/stretchr/testify/assert"
	"github.com/wmark/semver"
)

func TestImageParsingWithoutNamespace(t *testing.T) {
	img := NewFromString("repo/name:1")
	assert.Equal(t, "", img.Registry)
	assert.Equal(t, "1", img.Tag)
	assert.Equal(t, "repo/name", img.Name)
	assert.True(t, img.Contains(NewFromString("repo/name:1")))
}

func TestWildcardNamespace(t *testing.T) {
	img := NewFromString("repo/name:*")
	assert.Empty(t, img.Registry)
	assert.Equal(t, "*", img.Tag)
	assert.Equal(t, "repo/name", img.Name)
	assert.True(t, img.Contains(NewFromString("repo/name:1.0.0")))
}

func TestEnvironmentImageName(t *testing.T) {
	img := NewFromString("repo/name:1.0.0")
	assert.False(t, img.Contains(NewFromString("repo/name:1.0.123")))
}

func TestImageRealLifeNamingExample(t *testing.T) {
	img := NewFromString("docker.io/tools/dockerize:v0.0.1")
	assert.Equal(t, "docker.io", img.Registry)
	assert.Equal(t, "tools/dockerize", img.Name)
	assert.Equal(t, "v0.0.1", img.Tag)
	assert.True(t, img.Contains(NewFromString("docker.io/tools/dockerize:v0.0.1")))
}

func TestRangeContainsPlainVersion(t *testing.T) {
	img := NewFromString("docker.io/tools/dockerize:0.0.1")
	expected, _ := semver.NewRange("0.0.1")
	assert.Equal(t, "docker.io", img.Registry)
	assert.Equal(t, "tools/dockerize", img.Name)
	assert.Equal(t, "0.0.1", img.Tag)
	assert.Equal(t, expected, img.Version)

	v, _ := semver.NewVersion("0.0.1")
	assert.True(t, img.Version.Contains(v))
}

func TestUpperRangeBounds(t *testing.T) {
	img := NewFromString("docker.io/tools/dockerize:~1.2.3")
	assert.Equal(t, "docker.io", img.Registry)
	assert.Equal(t, "tools/dockerize", img.Name)
	assert.False(t, img.IsStrict())
	v, _ := semver.NewVersion("1.2.8")
	assert.True(t, img.Version.Contains(v))
}

func TestWildcardRangeBounds(t *testing.T) {
	img := NewFromString("docker.io/tools/dockerize:1.2.*")
	assert.Equal(t, "docker.io", img.Registry)
	assert.Equal(t, "tools/dockerize", img.Name)
	assert.False(t, img.IsStrict())
	v, _ := semver.NewVersion("1.2.8")
	assert.True(t, img.Version.Contains(v))
	v, _ = semver.NewVersion("1.2.0")
	assert.True(t, img.Version.Contains(v))
}

func TestWildcardContains(t *testing.T) {
	img1 := NewFromString("docker.io/tools/dockerize:1.2.*")
	img2 := NewFromString("docker.io/tools/dockerize:1.2.1")
	assert.False(t, img1.IsStrict())
	assert.True(t, img1.HasVersionRange())
	assert.True(t, img2.IsStrict())
	v, _ := semver.NewVersion("1.2.1")
	assert.Equal(t, v, img2.TagAsVersion())

	assert.True(t, img1.Contains(img2))
	assert.False(t, img2.Contains(img1))
}

func TestRangeContains(t *testing.T) {
	img1 := NewFromString("docker.io/tools/dockerize:~1.2.1")
	img2 := NewFromString("docker.io/tools/dockerize:1.2.999")
	assert.True(t, img1.Contains(img2))
	assert.False(t, img2.Contains(img1))
}

func TestNilContains(t *testing.T) {
	img1 := NewFromString("docker.io/tools/dockerize:~1.2.1")
	assert.False(t, img1.Contains(nil))
}

func TestRangeNotContains(t *testing.T) {
	img1 := NewFromString("docker.io/tools/dockerize:~1.2.1")
	img2 := NewFromString("docker.io/tools/dockerize:1.3.1")
	assert.False(t, img1.Contains(img2))
	assert.False(t, img2.Contains(img1))

	img2 = NewFromString("docker.io/xxx/dockerize:1.2.1")
	assert.False(t, img1.Contains(img2))

	img2 = NewFromString("dockerhub.grammarly.com/tools/dockerize:1.2.1")
	assert.False(t, img1.Contains(img2))

	img2 = NewFromString("docker.io/tools/dockerize:1.2.1")
	assert.True(t, img1.Contains(img2))
}

func TestVersionContains(t *testing.T) {
	img1 := NewFromString("docker.io/tools/dockerize:1.2.1")
	img2 := NewFromString("docker.io/tools/dockerize:1.2.1")
	assert.True(t, img1.Contains(img2))
	assert.True(t, img2.Contains(img1))
}

func TestTagContains(t *testing.T) {
	img1 := NewFromString("docker.io/tools/dockerize:test1")
	img2 := NewFromString("docker.io/tools/dockerize:test1")
	assert.True(t, img1.Contains(img2))
	assert.True(t, img2.Contains(img1))
}

func TestTagNotContains(t *testing.T) {
	img1 := NewFromString("docker.io/tools/dockerize:test1")
	img2 := NewFromString("docker.io/tools/dockerize:test2")
	assert.False(t, img1.Contains(img2))
	assert.False(t, img2.Contains(img1))
}

func TestImageRealLifeNamingExampleWithCapi(t *testing.T) {
	img := NewFromString("docker.io/common-api")
	assert.Equal(t, "docker.io", img.Registry)
	assert.Equal(t, "common-api", img.Name)
	assert.Equal(t, false, img.HasTag())
	assert.Equal(t, "latest", img.GetTag())
	assert.Equal(t, "docker.io/common-api:latest", img.String())
}

func TestImageParsingWithNamespace(t *testing.T) {
	img := NewFromString("hub/ns/name:1")
	assert.Equal(t, "", img.Registry)
	assert.Equal(t, "hub/ns/name", img.Name)
	assert.Equal(t, "1", img.Tag)
}

func TestImageParsingWithoutTag(t *testing.T) {
	img := NewFromString("repo/name")
	assert.Equal(t, "", img.Registry)
	assert.Equal(t, "repo/name", img.Name)
	assert.Equal(t, "latest", img.GetTag())
	assert.Equal(t, false, img.HasTag())
	assert.Equal(t, "repo/name:latest", img.String())
}

func TestImageWithDotsWithoutTag(t *testing.T) {
	img := NewFromString("a.b.c.d")
	assert.Equal(t, "", img.Registry)
	assert.Equal(t, "a.b.c.d", img.Name)
	assert.Equal(t, "latest", img.GetTag())
	assert.Equal(t, false, img.HasTag())
	assert.Equal(t, "a.b.c.d:latest", img.String())
}

func TestImageWithDotsWithTag(t *testing.T) {
	img := NewFromString("a.b.c.d:snapshot")
	assert.Equal(t, "", img.Registry)
	assert.Equal(t, "a.b.c.d", img.Name)
	assert.Equal(t, "snapshot", img.GetTag())
	assert.Equal(t, true, img.HasTag())
	assert.Equal(t, "a.b.c.d:snapshot", img.String())
}

func TestImageWithRegistryAndDotsAndTag(t *testing.T) {
	img := NewFromString("hub.com/a.b.c.d:snapshot")
	assert.Equal(t, "hub.com", img.Registry)
	assert.Equal(t, "a.b.c.d", img.Name)
	assert.Equal(t, "snapshot", img.GetTag())
	assert.Equal(t, true, img.HasTag())
	assert.Equal(t, "hub.com/a.b.c.d:snapshot", img.String())
}

func TestImageWithRegistryAndSlashAndDotsAndTag(t *testing.T) {
	img := NewFromString("hub.com/a.b/c.d:snapshot")
	assert.Equal(t, "hub.com", img.Registry)
	assert.Equal(t, "a.b/c.d", img.Name)
	assert.Equal(t, "snapshot", img.GetTag())
	assert.Equal(t, true, img.HasTag())
	assert.Equal(t, "hub.com/a.b/c.d:snapshot", img.String())
}

func TestImageLatest(t *testing.T) {
	img := NewFromString("rocker-build:latest")
	assert.Equal(t, "", img.Registry, "bag registry value")
	assert.Equal(t, "rocker-build", img.Name, "bad image name")
	assert.Equal(t, "latest", img.GetTag(), "bad image tag")
}

func TestImageIpRegistry(t *testing.T) {
	img := NewFromString("127.0.0.1:5000/golang:1.4")
	assert.Equal(t, "127.0.0.1:5000", img.Registry, "bag registry value")
	assert.Equal(t, "golang", img.Name, "bad image name")
	assert.Equal(t, "1.4", img.GetTag(), "bad image tag")
}

func TestImageTagSha(t *testing.T) {
	img := NewFromString("golang@sha256:ead434cd278824865d6e3b67e5d4579ded02eb2e8367fc165efa21138b225f11")
	assert.Equal(t, "", img.Registry, "bag registry value")
	assert.Equal(t, "golang", img.Name, "bad image name")
	assert.Equal(t, "sha256:ead434cd278824865d6e3b67e5d4579ded02eb2e8367fc165efa21138b225f11", img.GetTag(), "bad image tag")
	assert.Equal(t, "golang@sha256:ead434cd278824865d6e3b67e5d4579ded02eb2e8367fc165efa21138b225f11", img.String())
}

func TestImageAll(t *testing.T) {
	img := NewFromString("golang:1.*")
	assert.False(t, img.All())
}

func TestImageTest(t *testing.T) {
	t.Skip()
	names := []string{
		"golang:latest",
		"golang:stable",
		"golang:1.5.1",
		"golang:1.5.*",
		"golang:*",
		"golang",
	}
	for _, n := range names {
		img := NewFromString(n)
		m := [][2]interface{}{
			{"IsStrict()", img.IsStrict()},
			{"HasVersion()", img.HasVersion()},
			{"HasVersionRange()", img.HasVersionRange()},
			{"All()", img.All()},
			{"GetTag()", img.GetTag()},
		}
		fmt.Printf("%s\t%# v\n", n, pretty.Formatter(m))
	}
}

func TestImageResolveVersion_Strict(t *testing.T) {
	img := NewFromString("golang:1.5.2")
	list := []*ImageName{
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:1.5.3"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.5.2", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_Wildcard(t *testing.T) {
	img := NewFromString("golang:1.5.*")
	list := []*ImageName{
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:1.5.3"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.5.3", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_WildcardMulti(t *testing.T) {
	img := NewFromString("golang:1.4.*")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.4.2"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.4.2", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_WildcardMatchX(t *testing.T) {
	img := NewFromString("golang:1.4.x")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.4.x"),
		NewFromString("golang:1.4.2"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.4.x", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_WildcardMatchX2(t *testing.T) {
	img := NewFromString("golang:1.x")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.4.x"),
		NewFromString("golang:1.4.2"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.5.2", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_WildcardMatchX3(t *testing.T) {
	img := NewFromString("golang:1.x")
	list := []*ImageName{
		NewFromString("golang:1.x"),
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.4.x"),
		NewFromString("golang:1.4.2"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.x", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_All(t *testing.T) {
	img := NewFromString("golang:*")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.5.1", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_Latest(t *testing.T) {
	img := NewFromString("golang:latest")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:latest", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_OtherTag(t *testing.T) {
	img := NewFromString("golang:stable")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:stable"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:stable", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_NoTag(t *testing.T) {
	img := NewFromString("golang")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:stable"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:latest", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_NoTagOnlyLatest(t *testing.T) {
	img := NewFromString("golang")
	list := []*ImageName{
		NewFromString("golang:stable"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:latest", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_PatchExact(t *testing.T) {
	img := NewFromString("golang:1.4.1")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.4.1-p2"),
		NewFromString("golang:1.4.1-p1"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.4.1", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_PatchMatch(t *testing.T) {
	img := NewFromString("golang:1.4.1")
	list := []*ImageName{
		NewFromString("golang:1.4.1-p1"),
		NewFromString("golang:1.4.1-p2"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.4.1-p2", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_PatchStrict(t *testing.T) {
	img := NewFromString("golang:1.4.1-p1")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:1.4.1-p2"),
		NewFromString("golang:1.4.1-p1"),
		NewFromString("golang:1.5.1"),
		NewFromString("golang:1.5.2"),
		NewFromString("golang:latest"),
	}
	assert.Equal(t, "golang:1.4.1-p1", img.ResolveVersion(list).String())
}

func TestImageResolveVersion_NotFound(t *testing.T) {
	img := NewFromString("golang:1.5.1")
	list := []*ImageName{
		NewFromString("golang:1.4.1"),
		NewFromString("golang:stable"),
		NewFromString("golang:latest"),
	}
	assert.Nil(t, img.ResolveVersion(list))
}

func TestImageIsSameKind(t *testing.T) {
	assert.True(t, NewFromString("rocker-build").IsSameKind(*NewFromString("rocker-build")))
	assert.True(t, NewFromString("rocker-build:latest").IsSameKind(*NewFromString("rocker-build:latest")))
	assert.True(t, NewFromString("rocker-build").IsSameKind(*NewFromString("rocker-build:1.2.1")))
	assert.True(t, NewFromString("rocker-build:1.2.1").IsSameKind(*NewFromString("rocker-build:1.2.1")))
	assert.True(t, NewFromString("grammarly/rocker-build").IsSameKind(*NewFromString("grammarly/rocker-build")))
	assert.True(t, NewFromString("grammarly/rocker-build:3.1").IsSameKind(*NewFromString("grammarly/rocker-build:3.1")))
	assert.True(t, NewFromString("grammarly/rocker-build").IsSameKind(*NewFromString("grammarly/rocker-build:3.1")))
	assert.True(t, NewFromString("grammarly/rocker-build:latest").IsSameKind(*NewFromString("grammarly/rocker-build:latest")))
	assert.True(t, NewFromString("quay.io/rocker-build").IsSameKind(*NewFromString("quay.io/rocker-build")))
	assert.True(t, NewFromString("quay.io/rocker-build:latest").IsSameKind(*NewFromString("quay.io/rocker-build:latest")))
	assert.True(t, NewFromString("quay.io/rocker-build:3.2").IsSameKind(*NewFromString("quay.io/rocker-build:3.2")))
	assert.True(t, NewFromString("quay.io/rocker-build").IsSameKind(*NewFromString("quay.io/rocker-build:3.2")))
	assert.True(t, NewFromString("quay.io/grammarly/rocker-build").IsSameKind(*NewFromString("quay.io/grammarly/rocker-build")))
	assert.True(t, NewFromString("quay.io/grammarly/rocker-build:latest").IsSameKind(*NewFromString("quay.io/grammarly/rocker-build:latest")))
	assert.True(t, NewFromString("quay.io/grammarly/rocker-build:1.2.1").IsSameKind(*NewFromString("quay.io/grammarly/rocker-build:1.2.1")))
	assert.True(t, NewFromString("quay.io/grammarly/rocker-build").IsSameKind(*NewFromString("quay.io/grammarly/rocker-build:1.2.1")))

	assert.False(t, NewFromString("rocker-build").IsSameKind(*NewFromString("rocker-build2")))
	assert.False(t, NewFromString("rocker-build").IsSameKind(*NewFromString("rocker-compose")))
	assert.False(t, NewFromString("rocker-build").IsSameKind(*NewFromString("grammarly/rocker-build")))
	assert.False(t, NewFromString("rocker-build").IsSameKind(*NewFromString("quay.io/rocker-build")))
	assert.False(t, NewFromString("rocker-build").IsSameKind(*NewFromString("quay.io/grammarly/rocker-build")))
}

func TestTagsGetOld(t *testing.T) {
	tags := Tags{
		Items: []*Tag{
			&Tag{"1", *NewFromString("hub/ns/name:1"), time.Unix(1, 0).Unix()},
			&Tag{"3", *NewFromString("hub/ns/name:3"), time.Unix(3, 0).Unix()},
			&Tag{"2", *NewFromString("hub/ns/name:2"), time.Unix(2, 0).Unix()},
			&Tag{"5", *NewFromString("hub/ns/name:5"), time.Unix(5, 0).Unix()},
			&Tag{"4", *NewFromString("hub/ns/name:4"), time.Unix(4, 0).Unix()},
		},
	}
	old := tags.GetOld(2)

	assert.Equal(t, 3, len(old), "bad number of old tags")
	assert.Equal(t, "hub/ns/name:3", old[0].String(), "bad old image 1")
	assert.Equal(t, "hub/ns/name:2", old[1].String(), "bad old image 2")
	assert.Equal(t, "hub/ns/name:1", old[2].String(), "bad old image 3")
}

func TestImagename_ToYaml(t *testing.T) {
	value := struct {
		Name *ImageName
	}{
		NewFromString("hub/ns/name:1"),
	}

	data, err := yaml.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "name: hub/ns/name:1\n", string(data))
}

func TestImagename_S3_Basic(t *testing.T) {
	img := NewFromString("s3://bucket-name/image-name:1.2.3")
	assert.Equal(t, "bucket-name", img.Registry)
	assert.Equal(t, "image-name", img.Name)
	assert.Equal(t, "1.2.3", img.GetTag())
	assert.Equal(t, "bucket-name/image-name", img.NameWithRegistry())
	assert.Equal(t, "s3://bucket-name/image-name:1.2.3", img.String())
}

func TestImagename_S3_Etag(t *testing.T) {
	img := NewFromString("s3://bucket-name/image-name@sha256:ead434cd278824865d6e3b67e5d4579ded02eb2e8367fc165efa21138b225f11")
	assert.Equal(t, "bucket-name", img.Registry)
	assert.Equal(t, "image-name", img.Name)
	assert.Equal(t, true, img.TagIsSha())
	assert.Equal(t, "bucket-name/image-name", img.NameWithRegistry())
	assert.Equal(t, "sha256:ead434cd278824865d6e3b67e5d4579ded02eb2e8367fc165efa21138b225f11", img.GetTag())
	assert.Equal(t, "s3://bucket-name/image-name@sha256:ead434cd278824865d6e3b67e5d4579ded02eb2e8367fc165efa21138b225f11", img.String())
}
