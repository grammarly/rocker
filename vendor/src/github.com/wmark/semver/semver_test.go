// Copyright 2014 The Semver Package Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestDotDelimitedNumber(t *testing.T) {
	goodVer := "1.3.8"
	badVer := "a.b.c"

	Convey("Given an acceptable Version string", t, func() {
		v, err := newDotDelimitedNumber(goodVer)

		Convey("convert it correctly", func() {
			So(err, ShouldBeNil)
			So(len(v), ShouldEqual, 3)
			So(v, ShouldResemble, dotDelimitedNumber([]int{1, 3, 8}))
		})
	})

	Convey("Given a mal-formed Version string", t, func() {
		v, err := newDotDelimitedNumber(badVer)

		Convey("yield an error", func() {
			So(err, ShouldNotBeNil)
		})

		Convey("return an incomplete sequence", func() {
			So(len(v), ShouldBeLessThan, 3)
		})
	})
}

func TestVersion(t *testing.T) {
	Convey("Version 1.3.8 should be part of Version…", t, func() {
		v := []int{1, 3, 8, 0}

		Convey("1.3.8", func() {
			refVer, err := NewVersion("1.3.8")
			So(err, ShouldBeNil)
			So(refVer.version[:4], ShouldResemble, v)
		})

		Convey("1.3.8+build20140722", func() {
			refVer, err := NewVersion("1.3.8+build20140722")
			So(err, ShouldBeNil)
			So(refVer.version[:4], ShouldResemble, v)
		})

		Convey("1.3.8+build2014", func() {
			refVer, err := NewVersion("1.3.8+build2014")
			So(err, ShouldBeNil)
			So(refVer.version[:4], ShouldResemble, v)
		})

		Convey("1.3.8-alpha", func() {
			refVer, err := NewVersion("1.3.8-alpha")
			So(err, ShouldBeNil)
			So(refVer.version[:4], ShouldResemble, v)
		})

		Convey("1.3.8-beta", func() {
			refVer, err := NewVersion("1.3.8-beta")
			So(err, ShouldBeNil)
			So(refVer.version[:4], ShouldResemble, v)
		})

		Convey("1.3.8-pre", func() {
			refVer, err := NewVersion("1.3.8-pre")
			So(err, ShouldBeNil)
			So(refVer.version[:4], ShouldResemble, v)
		})

		Convey("1.3.8-r3", func() {
			refVer, err := NewVersion("1.3.8-p3")
			So(err, ShouldBeNil)
			So(refVer.version[:4], ShouldResemble, v)
		})

		Convey("1.3.8-3", func() {
			refVer, err := NewVersion("1.3.8-3")
			So(err, ShouldBeNil)
			So(refVer.version[:4], ShouldResemble, v)
		})

	})

	Convey("Working order between Versions", t, func() {

		Convey("equality", func() {
			v1, _ := NewVersion("1.3.8")
			v2, _ := NewVersion("1.3.8")
			So(v1, ShouldResemble, v2)
		})

		Convey("between different release types", func() {
			Convey("1.0.0 < 2.0.0", func() {
				v1, _ := NewVersion("1.0.0")
				v2, _ := NewVersion("2.0.0")
				So(v1.Less(v2), ShouldBeTrue)
				So(v2.Less(v1), ShouldBeFalse)
				So(v1, ShouldNotResemble, v2)
			})

			Convey("2.2.1 < 2.4.0-beta", func() {
				v1, _ := NewVersion("2.2.1")
				v2, _ := NewVersion("2.4.0-beta")
				So(v1.Less(v2), ShouldBeTrue)
				So(v2.Less(v1), ShouldBeFalse)
				So(v1, ShouldNotResemble, v2)
			})

			Convey("1.0.0 < 1.0.0-p", func() {
				v1, _ := NewVersion("1.0.0")
				v2, _ := NewVersion("1.0.0-p")
				So(v1.Less(v2), ShouldBeTrue)
				So(v2.Less(v1), ShouldBeFalse)
				So(v1, ShouldNotResemble, v2)
			})

			Convey("1.0.0-rc < 1.0.0", func() {
				v1, _ := NewVersion("1.0.0-rc")
				v2, _ := NewVersion("1.0.0")
				So(v1.Less(v2), ShouldBeTrue)
				So(v1, ShouldNotResemble, v2)
			})

			Convey("1.0.0-pre < 1.0.0-rc", func() {
				v1, _ := NewVersion("1.0.0-pre")
				v2, _ := NewVersion("1.0.0-rc")
				So(v1.Less(v2), ShouldBeTrue)
				So(v1, ShouldNotResemble, v2)
			})

			Convey("1.0.0-beta < 1.0.0-pre", func() {
				v1, _ := NewVersion("1.0.0-beta")
				v2, _ := NewVersion("1.0.0-pre")
				So(v1.Less(v2), ShouldBeTrue)
				So(v1, ShouldNotResemble, v2)
			})

			Convey("1.0.0-alpha < 1.0.0-beta", func() {
				v1, _ := NewVersion("1.0.0-alpha")
				v2, _ := NewVersion("1.0.0-beta")
				So(v1.Less(v2), ShouldBeTrue)
				So(v1, ShouldNotResemble, v2)
			})
		})

		Convey("between same release types", func() {
			Convey("1.0.0-p0 < 1.0.0-p1", func() {
				v1, _ := NewVersion("1.0.0-p0")
				v2, _ := NewVersion("1.0.0-p1")
				So(v1.Less(v2), ShouldBeTrue)
				So(v1, ShouldNotResemble, v2)
			})
		})

		Convey("with release type specifier", func() {
			Convey("1.0.0-rc4-alpha1 < 1.0.0-rc4", func() {
				v1, _ := NewVersion("1.0.0-rc4-alpha1")
				v2, _ := NewVersion("1.0.0-rc4")
				So(v1.Less(v2), ShouldBeTrue)
				So(v1, ShouldNotResemble, v2)
			})
		})

		Convey("with builds", func() {
			Convey("1.0.0+build14 < 1.0.0+build15", func() {
				v1, _ := NewVersion("1.0.0+build14")
				v2, _ := NewVersion("1.0.0+build15")
				So(v1.Less(v2), ShouldBeTrue)
				So(v1, ShouldNotResemble, v2)
			})

			Convey("1.0.0_pre20140722+build14 < 1.0.0_pre20140722+build15", func() {
				v1, _ := NewVersion("1.0.0_pre20140722+build14")
				v2, _ := NewVersion("1.0.0_pre20140722+build15")
				So(v1, ShouldNotResemble, v2)
				So(v1.Less(v2), ShouldBeTrue)
			})
		})

	})

	// see http://devmanual.gentoo.org/ebuild-writing/file-format/
	Convey("Gentoo's example of order works.", t, func() {
		v1, _ := NewVersion("1.0.0_alpha_pre")
		v2, _ := NewVersion("1.0.0_alpha_rc1")
		v3, _ := NewVersion("1.0.0_beta_pre")
		v4, _ := NewVersion("1.0.0_beta_p1")
		So(v1.version, ShouldResemble, [...]int{1, 0, 0, 0, alpha, 0, 0, 0, 0, pre, 0, 0, 0, 0})
		So(v2.version, ShouldResemble, [...]int{1, 0, 0, 0, alpha, 0, 0, 0, 0, rc, 1, 0, 0, 0})
		So(v3.version, ShouldResemble, [...]int{1, 0, 0, 0, beta, 0, 0, 0, 0, pre, 0, 0, 0, 0})
		So(v4.version, ShouldResemble, [...]int{1, 0, 0, 0, beta, 0, 0, 0, 0, patch, 1, 0, 0, 0})

		So(v1, ShouldNotResemble, v2)
		So(v2, ShouldNotResemble, v3)
		So(v3, ShouldNotResemble, v4)
		So(v1.Less(v2), ShouldBeTrue)
		So(v2.Less(v3), ShouldBeTrue)
		So(v3.Less(v4), ShouldBeTrue)
	})

	Convey("Reject too long Versions.", t, func() {
		Convey("with surplus digits", func() {
			_, err := NewVersion("1.0.0.0.4")
			So(err, ShouldNotBeNil)
		})

		Convey("with too long parts", func() {
			_, err := NewVersion("100000000000007000000000000000070000000000000.0.0")
			So(err, ShouldNotBeNil)
			_, err = NewVersion("1.0.0_alpha444444444444444444444444444444444444444")
			So(err, ShouldNotBeNil)
			_, err = NewVersion("1.0.0_alpha-rc444444444444444444444444444444444444")
			So(err, ShouldNotBeNil)
			_, err = NewVersion("1.0.0_alpha-rc1+build44444444444444444444444444444")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestVersionOrder(t *testing.T) {

	Convey("Version 1.2.3-alpha4 should be…", t, func() {
		v1, _ := NewVersion("1.2.3-alpha4")

		Convey("reasonably less than Version 1.2.3", func() {
			v2, _ := NewVersion("1.2.3")
			So(v1.limitedLess(v2), ShouldBeTrue)
		})

		Convey("reasonably less than Version 1.2.3-alpha4.0.0.1", func() {
			v2, _ := NewVersion("1.2.3-alpha4.0.0.1")
			So(v1.limitedLess(v2), ShouldBeTrue)
		})

		Convey("not reasonably less than 1.2.3-alpha4-p5", func() {
			v2, _ := NewVersion("1.2.3-alpha4-p5")
			So(v1.limitedLess(v2), ShouldBeFalse)
		})
	})

}
