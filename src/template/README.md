# rocker/template

Template renderer with additional helpers based on Go's [text/template](http://golang.org/pkg/text/template/) used by [rocker](https://github.com/grammarly/rocker) and [rocker-compose](https://github.com/grammarly/rocker-compose).

# Helpers

### {{ seq *To* }} or {{ seq *From* *To* }} or {{ seq *From* *To* *Step* }}
Sequence generator. Returns an array of integers of a given sequence. Useful when you need to duplicate some configuration, for example scale containers of the same type. Mostly used in combination with `range`:
```
{{ range $i := seq 1 5 2 }}
container-$i
{{ end }}
```

This template will yield:
```
container-1
container-3
container-5
```

### String functions
`rocker/template` exposes some Go's native functions from [strings](http://golang.org/pkg/strings/) package. Here is the list of them:

* `compare` - [strings.Compare](http://golang.org/pkg/strings/#Compare)
* `contains` - [strings.Contains](http://golang.org/pkg/strings/#Contains)
* `containsAny` - [strings.ContainsAny](http://golang.org/pkg/strings/#ContainsAny)
* `count` - [strings.Count](http://golang.org/pkg/strings/#Count)
* `equalFold` - [strings.EqualFold](http://golang.org/pkg/strings/#EqualFold)
* `hasPrefix` - [strings.HasPrefix](http://golang.org/pkg/strings/#HasPrefix)
* `hasSuffix` - [strings.HasSuffix](http://golang.org/pkg/strings/#HasSuffix)
* `index` - [strings.Index](http://golang.org/pkg/strings/#Index)
* `indexAny` - [strings.IndexAny](http://golang.org/pkg/strings/#IndexAny)
* `join` - [strings.Join](http://golang.org/pkg/strings/#Join)
* `lastIndex` - [strings.LastIndex](http://golang.org/pkg/strings/#LastIndex)
* `lastIndexAny` - [strings.LastIndexAny](http://golang.org/pkg/strings/#LastIndexAny)
* `repeat` - [strings.Repeat](http://golang.org/pkg/strings/#Repeat)
* `replace` - [strings.Replace](http://golang.org/pkg/strings/#Replace)
* `split` - [strings.Split](http://golang.org/pkg/strings/#Split)
* `splitAfter` - [strings.SplitAfter](http://golang.org/pkg/strings/#SplitAfter)
* `splitAfterN` - [strings.SplitAfterN](http://golang.org/pkg/strings/#SplitAfterN)
* `splitN` - [strings.SplitN](http://golang.org/pkg/strings/#SplitN)
* `title` - [strings.Title](http://golang.org/pkg/strings/#Title)
* `toLower` - [strings.ToLower](http://golang.org/pkg/strings/#ToLower)
* `toTitle` - [strings.ToTitle](http://golang.org/pkg/strings/#ToTitle)
* `toUpper` - [strings.ToUpper](http://golang.org/pkg/strings/#ToUpper)
* `trim` - [strings.Trim](http://golang.org/pkg/strings/#Trim)
* `trimLeft` - [strings.TrimLeft](http://golang.org/pkg/strings/#TrimLeft)
* `trimPrefix` - [strings.TrimPrefix](http://golang.org/pkg/strings/#TrimPrefix)
* `trimRight` - [strings.TrimRight](http://golang.org/pkg/strings/#TrimRight)
* `trimSpace` - [strings.TrimSpace](http://golang.org/pkg/strings/#TrimSpace)
* `trimSuffix` - [strings.TrimSuffix](http://golang.org/pkg/strings/#TrimSuffix)

Example:
```
{{ replace "www.google.com" "google" "grammarly" -1 }}
```

Will yield:
```
www.grammarly.com
```

### {{ json *anything* }} or {{ *anything* | json }}
Marshals given input to JSON.

Example:
```
ENV={{ .Env | json }}
```

This template will yield:
```
ENV={"USER":"johnsnow","DOCKER_MACHINE_NAME":"dev","PATH":"/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin",...}
```

### {{ yaml *anything* }} or {{ *anything* | yaml }}
Marshals given input to YAML.

Example:
```
{{ .Env | yaml }}
```

This template will yield:
```
USER: johnsnow
DOCKER_MACHINE_NAME: dev
PATH: /usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
```

### {{ *anything* | yaml *N* }} where *N* is indentation level
Useful if you want to nest a yaml struct into another yaml file:

```yaml
foo:
  bar:
    {{ .bar | yaml 2 }}
```

Will indent yaml-encoded `.bar` into two levels:

```yaml
foo:
  bar:
    a: 1
    b: 2
```

### {{ shell *string* }} or {{ *string* | shell }}
Escapes given string so it can be substituted to a shell command.

Example:
```Dockerfile
RUN echo {{ "hello\nworld" | shell }}
```

This template will yield:
```Dockerfile
RUN echo $'hello\nworld'
```

### {{ dump *anything* }}
Pretty-prints any variable. Useful for debugging.

Example:
```
{{ dump .Env }}
```

This template will yield:
```
template.Vars{
    "USER":                       "johnsnow",
    "DOCKER_MACHINE_NAME":        "dev",
    "PATH":                       "/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin",
    ...
}
```

### {{ assert *expression* }}
Raises an error if given expression is false. *Positive* value is an existing non-nil value, non-empty slice, non-empty string, and non-zero number.

For example `assert` is useful to check that passed variables are present.

```
{{ assert .Version }}
```

If the `Version` variable is not given, then template processing will fail with the following error:

```
Error executing template TEMPLATE_NAME, error: template: TEMPLATE_NAME:1:3: executing \"TEMPLATE_NAME\" at <assert .Version>: error calling assert: Assertion failed
```

### {{ image *docker_image_name_with_tag* }} or {{ image *docker_image_name* *tag* }}
Wrapper that is used to substitute images of particular versions derived by artifacts *(TODO: link to artifacts doc)*.

Example:
```Dockerfile
FROM {{ image "ubuntu" }}
# OR
FROM {{ image "ubuntu:latest" }}
# OR
FROM {{ image "ubuntu" "latest" }}
```

Without any additional arguments it will resolve into this:
```Dockerfile
FROM ubuntu:latest
```

But if you have an artifact that is resulted by a previous rocker build, that can be fed back to rocker as variable, the artifact will be substituted:
```yaml
# shorten version of an artifact by rocker
RockerArtifacts:
- Name: ubuntu:latest
  Digest: sha256:ead434cd278824865d6e3b67e5d4579ded02eb2e8367fc165efa21138b225f11
```

```Dockerfile
# rocker build -vars artifacts/*
FROM ubuntu@sha256:ead434cd278824865d6e3b67e5d4579ded02eb2e8367fc165efa21138b225f11
```

This feature is useful when you have a continuous integration pipeline and you want to build images on top of each other with guaranteed immutability. Also, this trick can be used with [rocker-compose](https://github.com/grammarly/rocker-compose) to run images of particular versions devired by the artifacts.

*TODO: also describe semver matching behavior*

# Variables
`rocker/template` automatically populates [os.Environ](https://golang.org/pkg/os/#Environ) to the template along with the variables that are passed from the outside. All environment variables are available under `.Env`.

Example:
```
HOME={{ .Env.HOME }}
```

# Load file content to a variable
This template engine also supports loading files content to a variables. `rocker` and `rocker-compose` support this through a command line parameters:

```bash
rocker build -var key=@key.pem
rocker-compose run -var key=@key.pem
```

If the file path is relative, it will be resolved according to the current working directory.

**Usage options:**

```
key=@relative/file/path.txt
key=@../another/relative/file/path.txt
key=@/absolute/file/path.txt
key=@~/.bash_history
key=\@keep_value_as_is
```

# Development

Please install pre-push git hook that will run tests before every push:

```bash
cd rocker-template
```

To run tests manually:

```bash
make test
```

Or to test something particular:

```bash
go test -run TestProcessConfigTemplate_Seq
```

# Authors

- Yura Bogdanov <yuriy.bogdanov@grammarly.com>

# License

(c) Copyright 2015 Grammarly, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
