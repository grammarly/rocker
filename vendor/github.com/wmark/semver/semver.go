// Copyright 2014 The Semver Package Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package semver

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

type dotDelimitedNumber []int

func newDotDelimitedNumber(str string) (dotDelimitedNumber, error) {
	strSequence := strings.Split(str, ".")
	if len(strSequence) > 4 {
		return nil, errors.New("too much columns on that number")
	}
	numSequence := make(dotDelimitedNumber, 0, len(strSequence))
	for _, s := range strSequence {
		i, err := strconv.Atoi(s)
		if err != nil {
			return numSequence, err
		}
		numSequence = append(numSequence, i)
	}
	return numSequence, nil
}

// alpha = -4, beta = -3, pre = -2, rc = -1, common = 0, patch = 1
const (
	alpha = iota - 4
	beta
	pre
	rc
	common
	patch
)

const (
	idxReleaseType   = 4
	idxRelease       = 5
	idxSpecifierType = 9
	idxSpecifier     = 10
)

var releaseDesc = map[int]string{
	alpha: "alpha",
	beta:  "beta",
	pre:   "pre",
	rc:    "rc",
	patch: "p",
}

var releaseValue = map[string]int{
	"alpha": alpha,
	"beta":  beta,
	"pre":   pre,
	"":      pre,
	"rc":    rc,
	"p":     patch,
}

var verRegexp = regexp.MustCompile(`^(\d+(?:\.\d+){0,3})(?:([-_]alpha|[-_]beta|[-_]pre|[-_]rc|[-_]p|-)(\d+(?:\.\d+){0,3})?)?(?:([-_]alpha|[-_]beta|[-_]pre|[-_]rc|[-_]p|-)(\d+(?:\.\d+){0,3})?)?(?:(\+build)(\d*))?$`)

type Version struct {
	// 0–3: version, 4: releaseType, 5–8: releaseVer, 9: releaseSpecifier, 10–14: specifier
	version [14]int
	build   int
}

func NewVersion(str string) (*Version, error) {
	allMatches := verRegexp.FindAllStringSubmatch(str, -1)
	if allMatches == nil {
		return nil, errors.New("Given string does not resemble a Version.")
	}
	ver := new(Version)
	m := allMatches[0]

	// version
	n, err := newDotDelimitedNumber(m[1])
	if err != nil {
		return nil, err
	}
	copy(ver.version[:], n)

	// release
	if m[2] != "" {
		ver.version[idxReleaseType] = releaseValue[strings.Trim(m[2], "-_")]
	}
	if m[3] != "" {
		n, err := newDotDelimitedNumber(m[3])
		if err != nil {
			return nil, err
		}
		copy(ver.version[idxRelease:], n)
	}

	// release specifier
	if m[4] != "" {
		ver.version[idxSpecifierType] = releaseValue[strings.Trim(m[4], "-_")]
	}
	if m[5] != "" {
		n, err := newDotDelimitedNumber(m[5])
		if err != nil {
			return nil, err
		}
		copy(ver.version[idxSpecifier:], n)
	}

	// build
	if m[7] != "" {
		i, err := strconv.Atoi(m[7])
		if err != nil {
			return nil, err
		}
		ver.build = i
	}

	return ver, nil
}

// Returns sign(a - b).
func signDelta(a, b [14]int, cutoffIdx int) int8 {
	for i := range a {
		if i >= cutoffIdx {
			return 0
		}
		if a[i] < b[i] {
			return -1
		} else if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// Convenience function for sorting.
func (t *Version) Less(o *Version) bool {
	sd := signDelta(t.version, o.version, 15)
	return sd < 0 || (sd == 0 && t.build < o.build)
}

// Limited to version, (pre-)release type and (pre-)release version.
// Commutative.
func (t *Version) limitedLess(o *Version) bool {
	return signDelta(t.version, o.version, idxSpecifierType) < 0
}

// Equality limited to version, (pre-)release type and (pre-)release version.
// For example, 1.0.0-pre and 1.0.0-rc are not the same, but
// 1.0.0-beta-pre3 and 1.0.0-beta-pre5 are equal.
// Permits 'patch levels', regarding 1.0.0 equal to 1.0.0-p1.
// Non-commutative: 1.0.0-p1 does not equal 1.0.0!
func (t *Version) LimitedEqual(o *Version) bool {
	if t.version[idxReleaseType] == common && o.version[idxReleaseType] > common {
		return t.sharesPrefixWith(o)
	}
	return signDelta(t.version, o.version, idxSpecifierType) == 0
}

// Use this to exclude pre-releases.
func (v *Version) IsAPreRelease() bool {
	return v.version[idxReleaseType] < common
}

// A 'prefix' is the major, minor, patch and revision number.
// For example: 1.2.3.4…
func (t *Version) sharesPrefixWith(o *Version) bool {
	return signDelta(t.version, o.version, idxReleaseType) == 0
}
