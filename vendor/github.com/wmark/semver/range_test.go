// Copyright 2014 The Semver Package Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func hasLowerBound(aRange interface{}, aVersion ...interface{}) string {
	a := aRange.(*Range)
	if s := ShouldResemble(a.lower, aVersion[0]); s != "" {
		return s
	}
	return ShouldBeTrue(a.equalsLower)
}

func isLeftClosedBy(aRange interface{}, aVersion ...interface{}) string {
	a := aRange.(*Range)
	if s := ShouldResemble(a.lower, aVersion[0]); s != "" {
		return s
	}
	return ShouldBeFalse(a.equalsLower)
}

func hasUpperBound(aRange interface{}, aVersion ...interface{}) string {
	a := aRange.(*Range)
	if s := ShouldResemble(a.upper, aVersion[0]); s != "" {
		return s
	}
	return ShouldBeTrue(a.equalsUpper)
}

func isRightClosedBy(aRange interface{}, aVersion ...interface{}) string {
	a := aRange.(*Range)
	if s := ShouldResemble(a.upper, aVersion[0]); s != "" {
		return s
	}
	return ShouldBeFalse(a.equalsUpper)
}

func testIfResembles(actual, expected *Range) {
	So(actual.lower, ShouldResemble, expected.lower)
	So(actual.equalsLower, ShouldEqual, expected.equalsLower)
	So(actual.upper, ShouldResemble, expected.upper)
	So(actual.equalsUpper, ShouldEqual, expected.equalsUpper)
}

func shouldContain(aRange interface{}, aVersion ...interface{}) string {
	a := aRange.(*Range)
	for _, version := range aVersion {
		v := version.(string)
		ver, err := NewVersion(v)
		if err != nil {
			return err.Error()
		}
		if s := ShouldBeTrue(a.Contains(ver)); s != "" {
			return v + " is not in Range"
		}
	}
	return ""
}

func shouldNotContain(aRange interface{}, aVersion ...interface{}) string {
	a := aRange.(*Range)
	for _, version := range aVersion {
		v := version.(string)
		ver, err := NewVersion(v)
		if err != nil {
			return err.Error()
		}
		if s := ShouldBeFalse(a.Contains(ver)); s != "" {
			return v + " is in Range"
		}
	}
	return ""
}

func TestRangeConstruction(t *testing.T) {

	Convey("version 1.2.3 should be part of…", t, func() {
		ver, _ := NewVersion("1.2.3")

		Convey("specific Range 1.2.3", func() {
			verRange, _ := NewRange("1.2.3")
			So(verRange.lower, ShouldResemble, ver)
		})
	})

	v100, _ := NewVersion("1.0.0")
	v120, _ := NewVersion("1.2.0")
	v123, _ := NewVersion("1.2.3")
	v130, _ := NewVersion("1.3.0")
	v200, _ := NewVersion("2.0.0")

	Convey("Range >=1.2.3 <=1.3.0…", t, func() {
		verRange, err := NewRange(">=1.2.3 <=1.3.0")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}

		Convey("has lower bound >=1.2.3", func() {
			So(verRange, hasLowerBound, v123)
		})

		Convey("has upper bound <=1.3.0", func() {
			So(verRange, hasUpperBound, v130)
		})
	})

	Convey("Range >1.2.3 <1.3.0…", t, func() {
		verRange, err := NewRange(">1.2.3 <1.3.0")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}

		Convey("has lower bound >1.2.3", func() {
			So(verRange, isLeftClosedBy, v123)
		})

		Convey("has upper bound <1.3.0", func() {
			So(verRange, isRightClosedBy, v130)
		})
	})

	Convey("Range 1.2.3 - 1.3.0…", t, func() {
		verRange, err := NewRange("1.2.3 - 1.3.0")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}

		Convey("has lower bound >=1.2.3", func() {
			So(verRange, hasLowerBound, v123)
		})

		Convey("has upper bound <=1.3.0", func() {
			So(verRange, hasUpperBound, v130)
		})
	})

	Convey("Range ~1.2.3 equals: >=1.2.3 <1.3.0", t, func() {
		verRange, err := NewRange("~1.2.3")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}

		Convey("has lower bound >=1.2.3", func() {
			So(verRange, hasLowerBound, v123)
		})

		Convey("has upper bound <1.3.0", func() {
			So(verRange, isRightClosedBy, v130)
		})
	})

	Convey("Range ^1.2.3 equals: >=1.2.3 <2.0.0", t, func() {
		verRange, err := NewRange("^1.2.3")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}

		Convey("has lower bound >=1.2.3", func() {
			So(verRange, hasLowerBound, v123)
		})

		Convey("has upper bound <2.0.0", func() {
			So(verRange, isRightClosedBy, v200)
		})
	})

	Convey("Range ~1.2 equals: >=1.2.0 <1.3.0", t, func() {
		verRange, err := NewRange("~1.2")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}

		Convey("has lower bound >=1.2.0", func() {
			So(verRange, hasLowerBound, v120)
		})

		Convey("has upper bound <1.3.0", func() {
			So(verRange, isRightClosedBy, v130)
		})
	})

	Convey("Range ^1.2 equals: >=1.2.0 <2.0.0", t, func() {
		verRange, err := NewRange("^1.2")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}

		Convey("has lower bound >=1.2.0", func() {
			So(verRange, hasLowerBound, v120)
		})

		Convey("has upper bound <2.0.0", func() {
			So(verRange, isRightClosedBy, v200)
		})
	})

	Convey("Ranges ^1 and ~1 equal: >=1.0.0 <2.0.0", t, func() {
		r1, err := NewRange("^1")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}
		r2, err := NewRange("^1")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}

		Convey("has lower bound >=1.0.0", func() {
			So(r1, hasLowerBound, v100)
			So(r2, hasLowerBound, v100)
		})

		Convey("has upper bound <2.0.0", func() {
			So(r1, isRightClosedBy, v200)
			So(r2, isRightClosedBy, v200)
		})
	})

	Convey("Given .x and .* notations", t, func() {
		refRange, _ := NewRange("^1")

		Convey("1.x equals ^1", func() {
			r1, err := NewRange("1.x")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, refRange)
		})

		Convey("1.* equals ^1", func() {
			r1, err := NewRange("1.*")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, refRange)
		})

		Convey("1 equals ^1", func() {
			r1, err := NewRange("1")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, refRange)
		})

		smallRange, _ := NewRange("~1.2")

		Convey("1.2.x equals ~1.2", func() {
			r1, err := NewRange("1.2.x")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, smallRange)
		})

		Convey("1.2.* equals ~1.2", func() {
			r1, err := NewRange("1.2.*")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, smallRange)
		})

		Convey("1.2 equals ~1.2", func() {
			r1, err := NewRange("1.2")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, smallRange)
		})
	})

	Convey("Notations for 'any'", t, func() {
		refRange := new(Range)

		Convey("'x'", func() {
			r1, err := NewRange("x")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, refRange)
		})

		Convey("'*'", func() {
			r1, err := NewRange("*")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, refRange)
		})

		Convey("'' (empty string)", func() {
			r1, err := NewRange("")
			So(err, ShouldBeNil)
			if err != nil {
				return
			}
			testIfResembles(r1, refRange)
		})
	})

	// now come fringe cases
	Convey("Range ^0.1.3 and ~0.1.3 equal: >=0.1.3 <0.2.0", t, func() {
		r1, err := NewRange("^0.1.3")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}
		r2, err := NewRange("~0.1.3")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}
		v013, _ := NewVersion("0.1.3")
		v020, _ := NewVersion("0.2.0")

		Convey("have lower bound >=0.1.3", func() {
			So(r1, hasLowerBound, v013)
			So(r2, hasLowerBound, v013)
		})

		Convey("have upper bound <0.2.0", func() {
			So(r1, isRightClosedBy, v020)
			So(r2, isRightClosedBy, v020)
		})
	})

	Convey("Range ^0.0.2 and ~0.0.2 are 0.2.0", t, func() {
		r1, err := NewRange("^0.0.2")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}
		r2, err := NewRange("~0.0.2")
		So(err, ShouldBeNil)
		if err != nil {
			return
		}
		r002, _ := NewRange("0.0.2")

		Convey("have the same bounds as 0.2.0", func() {
			testIfResembles(r1, r002)
			testIfResembles(r2, r002)
		})
	})
}

func TestSingleBound(t *testing.T) {

	Convey("Given a specific version as Range…", t, func() {
		Convey("1.2.3…", func() {
			verRange, _ := NewRange("1.2.3")

			Convey("reject Version 1.2.4", func() {
				So(verRange, shouldNotContain, "1.2.4")
			})

			Convey("accept Version 1.2.3", func() {
				So(verRange, shouldContain, "1.2.3")
			})

			Convey("accept Version 1.2.3+build2014 (ignore build)", func() {
				So(verRange, shouldContain, "1.2.3+build2014")
			})

			Convey("accept Version 1.2.3-p1 (patch levels are reasonably equal)", func() {
				So(verRange, shouldContain, "1.2.3-p1")
			})

			Convey("reject pre-releases like 1.2.3-alpha", func() {
				So(verRange, shouldNotContain, "1.2.3-alpha")
			})
		})

		Convey("1.2.3-alpha20…", func() {
			verRange, _ := NewRange("1.2.3-alpha20")

			Convey("accept Version 1.2.3-alpha20", func() {
				So(verRange, shouldContain, "1.2.3-alpha20")
			})
			Convey("reject Version 1.2.3-alpha5", func() {
				So(verRange, shouldNotContain, "1.2.3-alpha5")
			})
			Convey("reject Version 1.2.3-beta", func() {
				So(verRange, shouldNotContain, "1.2.3-beta")
			})
			Convey("reject Version 1.2.3", func() {
				So(verRange, shouldNotContain, "1.2.3")
			})
		})
	})

	Convey("Given the lower bound >1.2.3", t, func() {
		verRange, _ := NewRange(">1.2.3")

		Convey("reject Version 1.2.3", func() {
			So(verRange, shouldNotContain, "1.2.3")
		})

		Convey("reject Version 1.2.3-p1 (ignore release/pre-release)", func() {
			So(verRange, shouldNotContain, "1.2.3-p1")
		})

		Convey("reject Version 1.2.3+build2014 (ignore build)", func() {
			So(verRange, shouldNotContain, "1.2.3+build2014")
		})

		Convey("accept Versions…", func() {
			Convey("1.2.4", func() {
				So(verRange, shouldContain, "1.2.4")
			})
			Convey("1.3.0", func() {
				So(verRange, shouldContain, "1.3.0")
			})
			Convey("2.0.0", func() {
				So(verRange, shouldContain, "2.0.0")
			})
		})
	})

	Convey("A lower bound >=1.2.3…", t, func() {
		verRange, _ := NewRange(">=1.2.3")

		Convey("should contain Version 1.2.3", func() {
			So(verRange, shouldContain, "1.2.3")
		})

		Convey("should contain Version 1.2.3-p1", func() {
			So(verRange, shouldContain, "1.2.3-p1")
		})

		Convey("but NOT contain 1.2.3-rc (exclude pre-release)", func() {
			So(verRange, shouldNotContain, "1.2.3-rc")
		})
	})

	Convey("Over-specific lower bounds >=1.2.3-alpha4…", t, func() {
		verRange, _ := NewRange(">=1.2.3-alpha4")

		Convey("should contain Version 1.2.4", func() {
			So(verRange, shouldContain, "1.2.4")
		})

		Convey("should contain Version 1.2.3-alpha4", func() {
			So(verRange, shouldContain, "1.2.3-alpha4")
		})

		Convey("but NOT contain 1.2.3-alpha", func() {
			So(verRange, shouldNotContain, "1.2.3-alpha")
		})
	})

	Convey("Upper bounds such as <1.2.3…", t, func() {
		verRange, _ := NewRange("<1.2.3")

		Convey("reject Version 1.2.3", func() {
			So(verRange, shouldNotContain, "1.2.3")
		})

		Convey("reject Version 1.2.3-alpha3 (ignore release/pre-release)", func() {
			So(verRange, shouldNotContain, "1.2.3-alpha3")
		})

		Convey("reject Version 1.2.3+build2014 (ignore build)", func() {
			So(verRange, shouldNotContain, "1.2.3+build2014")
		})

		Convey("accept Versions…", func() {
			Convey("1.2.0", func() {
				So(verRange, shouldContain, "1.2.0")
			})
			Convey("1.1.0", func() {
				So(verRange, shouldContain, "1.1.0")
			})
			Convey("1.0.0", func() {
				So(verRange, shouldContain, "1.0.0")
			})
		})
	})

	Convey("An over-specific upper bound <1.2.3-beta20…", t, func() {
		verRange, _ := NewRange("<1.2.3-beta20")

		Convey("reject Version 1.2.3-beta20", func() {
			So(verRange, shouldNotContain, "1.2.3-beta20")
		})

		Convey("reject Version 1.2.3-beta20-alpha3 (ignore release specifier)", func() {
			So(verRange, shouldNotContain, "1.2.3-beta20-alpha3")
		})

		Convey("reject Version 1.2.3-beta20+build2014 (ignore build)", func() {
			So(verRange, shouldNotContain, "1.2.3-beta20+build2014")
		})

		Convey("accept Versions…", func() {
			Convey("1.2.3-beta19", func() {
				So(verRange, shouldContain, "1.2.3-beta19")
			})
			Convey("1.2.3-alpha", func() {
				So(verRange, shouldContain, "1.2.3-alpha")
			})
			Convey("1.2.0", func() {
				So(verRange, shouldContain, "1.2.0")
			})
		})
	})

	Convey("An upper bound with equality <=1.2.3…", t, func() {
		verRange, _ := NewRange("<=1.2.3")

		Convey("should contain Version 1.2.3", func() {
			So(verRange, shouldContain, "1.2.3")
		})

		Convey("should contain Version 1.2.3-p1", func() {
			So(verRange, shouldContain, "1.2.3-p1")
		})

		Convey("and, that's new, contain 1.2.3-rc (INCLUDE pre-release)", func() {
			So(verRange, shouldContain, "1.2.3-rc")
		})
	})

}

func TestSatisfies(t *testing.T) {

	Convey("Convenience function 'Satisfies'", t, func() {

		Convey("works with valid input", func() {
			t, _ := Satisfies("1.2.3", "^1.2.2")
			So(t, ShouldBeTrue)
			t, _ = Satisfies("1.2.3-2", "^1.2.3-1")
			So(t, ShouldBeTrue)
		})

		Convey("yields an error on invalid Version", func() {
			t, err := Satisfies("1.2.3.4.5.6", "^1.2.2")
			So(t, ShouldBeFalse)
			So(err, ShouldNotBeNil)
		})

		Convey("yields an error on invalid Range", func() {
			t, err := Satisfies("1.2.3", "^1.2.2/1.2.5")
			So(t, ShouldBeFalse)
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Range.IsSatisfiedBy…", t, func() {

		Convey("rejects pre-releases", func() {
			t, _ := Satisfies("1.2.3-1", "^1.2.2")
			So(t, ShouldBeFalse)
			t, _ = Satisfies("1.2.4-1", "^1.2.2-1")
			So(t, ShouldBeFalse)
			t, _ = Satisfies("1.2.3-1", "<1.2.3")
			So(t, ShouldBeFalse)
		})

		Convey("accepts pre-releases for a pre-release upper bound with the same prefix", func() {
			t, _ := Satisfies("1.2.3-1", "<1.2.3-1")
			So(t, ShouldBeFalse)
			t, _ = Satisfies("1.2.3-1", "<1.2.3-2")
			So(t, ShouldBeTrue)
			t, _ = Satisfies("1.2.3-1", "<=1.2.3-1")
			So(t, ShouldBeTrue)
		})

		Convey("accepts pre-releases for a pre-release lower bound with the same prefix", func() {
			t, _ := Satisfies("1.2.3-1", ">1.2.3-1")
			So(t, ShouldBeFalse)
			t, _ = Satisfies("1.2.3-2", ">1.2.3-1")
			So(t, ShouldBeTrue)
			t, _ = Satisfies("1.2.3-1", ">=1.2.3-1")
			So(t, ShouldBeTrue)
		})
	})

	Convey("Test the examples found in README file.", t, func() {
		v, _ := NewVersion("1.2.3-beta")
		r, _ := NewRange("~1.2")
		So(r.Contains(v), ShouldBeTrue)
		So(r.IsSatisfiedBy(v), ShouldBeFalse)
	})
}
