package tests

import (
	// "github.com/stretchr/testify/assert"
	// "io/ioutil"
	// "os"
	// "strconv"
	"testing"
	//"time"
)

func TestAddUrl_Basic(t *testing.T) {

	// ADD file1 http://localhost:123456/something/url/file2 https://some/thing/file3 /dst

	// check contents of /dst/file{1,2,3}
}

func TestAddUrl_BuildCacheHit(t *testing.T) {

	// ADD file1 http://localhost:123456/something/url/file2 https://some/thing/file3

	// serve file2, file3 with no Etags

	// check contents of /dst/file{1,2,3}

	// build container again

	// ensure cache isn't invalidated

}

func TestAddUrl_CacheHit(t *testing.T) {

	// ADD file1 http://localhost:123456/something/url/file2 https://some/thing/file3

	// check contents of /dst/file{1,2,3}

	// serve file2, file3 with Etags

	// build container again

	// ensure cache isn't invalidated

	// ensure download doesn't happen

}

func TestAddUrl_CacheMiss(t *testing.T) {

	// ADD file1 http://localhost:123456/something/url/file2 https://some/thing/file3

	// build once

	// ensure file{1,2,3} get added

	// build second time, serve files with invalidated Etags and new content

	// ensure new files get downloaded

}

func TestAddUrl_NoCache(t *testing.T) {
	// ADD file1 http://localhost:123456/something/url/file2 https://some/thing/file3

	// build once

	// ensure file2, file3 get added

	// build second time with valid etags but -no-cache flag set

	// ensure new files get downloaded
}
