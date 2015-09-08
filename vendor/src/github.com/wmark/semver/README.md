Semantic Versioning for Golang
==============================

[![Build Status](https://drone.io/github.com/wmark/semver/status.png)](https://drone.io/github.com/wmark/semver/latest)
[![Coverage Status](https://coveralls.io/repos/wmark/semver/badge.png?branch=master)](https://coveralls.io/r/wmark/semver?branch=master)
[![GoDoc](https://godoc.org/github.com/wmark/semver?status.png)](https://godoc.org/github.com/wmark/semver)

A library for parsing and processing of *Versions* and *Ranges* in:

* [Semantic Versioning](http://semver.org/) (semver) v2.0.0 notation
  * used by npmjs.org, pypi.org…
* Gentoo's ebuild format
* NPM

Licensed under a [BSD-style license](LICENSE).

Usage
-----
```bash
$ go get -v github.com/wmark/semver
```

```go
import github.com/wmark/semver

v1, err := semver.NewVersion("1.2.3-beta")
v2, err := semver.NewVersion("2.0.0-alpha20140805.456-rc3+build1800")
v1.Less(v2)

r1, err := NewRange("~1.2")
r1.Contains(v1)      // true
r1.IsSatisfiedBy(v1) // false
```

Also check the [GoDocs](http://godoc.org/github.com/wmark/semver)
and [Gentoo Linux Ebuild File Format](http://devmanual.gentoo.org/ebuild-writing/file-format/),
[Gentoo's notation of dependencies](http://devmanual.gentoo.org/general-concepts/dependencies/).

Please Note
-----------

It is, ordered from lowest to highest:

    alpha < beta < pre < rc < (no release type/»common«) < p

Therefore it is:

    Version("1.0.0-pre1") ≙ Version("1.0.0-1") < Version("1.0.0") < Version("1.0.0-p1")

… because the SemVer specification says:

    9. A pre-release version MAY be denoted by appending a hyphen and a series of
    dot separated identifiers immediately following the patch version. […]

Most *NodeJS* authors write **~1.2.3** where **>=1.2.3** would fit better.
*~1.2.3* is ```Range(">=1.2.3 <1.3.0")``` and excludes versions such as *1.4.0*,
which almost always work.

Contribute
----------

Please open issues with minimal examples of what is going wrong. For example:

    Mark, this does not work but I feel that it should:
    ```golang
    v, err := semver.Version("17.1o0")
    # yields an error
    # I expected 17.100
    ```

Please write a test case with your expectations, if you ask for a new feature.

    I'd like **semver.Range** to support interface **expvar.Var**, like this:
    ```golang
    Convey("Range implements interface's expvar.Var…", t, func() {
      r, _ := NewRange("1.2.3")
      
      Convey("String()", func() {
        So(r.String(), ShouldEqual, "1.2.3")
      })
    })
    ```

Pull requests are welcome.
Please add your name and email address to file *AUTHORS* and/or *CONTRIBUTORS*.
Thanks!
