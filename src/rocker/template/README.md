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

### {{ replace *s* *old* *new* }}
Does a string replacement. Works simply like Go's [strings.Replace](http://golang.org/pkg/strings/#Replace).

Example:
```
{{ replace "www.google.com" "google" "grammarly" }}
```

This template will yield:
```
www.grammarly.com
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

# Variables
`rocker/template` automatically populates [os.Environ](https://golang.org/pkg/os/#Environ) to the template along with the variables that are passed from the outside. All environment variables are available under `.Env`.

Example:
```
HOME={{ .Env.HOME }}
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
