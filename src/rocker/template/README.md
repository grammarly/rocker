# rocker/template

Template renderer with additional helpers based on Go's `text/template` used by [rocker](https://github.com/grammarly/rocker) and [rocker-compose](https://github.com/grammarly/rocker-compose).

# Helpers

###### {{ seq *To* }} or {{ seq *From* *To* }} or {{ seq *From* *To* *Step* }}
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

# Development

Please install pre-push git hook that will run tests before every push:

```bash
cd rocker-template
./install_githook.sh
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
