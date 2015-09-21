// NOTICE: it was originally grabbed from the docker source and
//         adopted for use by rocker; see LICENSE in the current
//         directory from the license and the copyright.
//
//         Copyright 2013-2015 Docker, Inc.

package build2

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/tarsum"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/fsouza/go-dockerclient/vendor/github.com/docker/docker/pkg/archive"
	"github.com/fsouza/go-dockerclient/vendor/github.com/docker/docker/pkg/system"
)

type copyInfo struct {
	origPath   string
	destPath   string
	hash       string
	decompress bool
	tmpDir     string
}

func copyCommand(b *Build, args []string, allowRemote bool, allowDecompression bool, cmdName string) (s State, err error) {

	s = b.state

	if len(args) < 2 {
		return s, fmt.Errorf("Invalid %s format - at least two arguments required", cmdName)
	}

	// Work in daemon-specific filepath semantics
	dest := filepath.FromSlash(args[len(args)-1]) // last one is always the dest

	copyInfos := []*copyInfo{}

	// b.Config.Image = b.image

	defer func() {
		for _, ci := range copyInfos {
			if ci.tmpDir != "" {
				os.RemoveAll(ci.tmpDir)
			}
		}
	}()

	// Loop through each src file and calculate the info we need to
	// do the copy (e.g. hash value if cached).  Don't actually do
	// the copy until we've looked at all src files
	for _, orig := range args[0 : len(args)-1] {
		if err := calcCopyInfo(
			b,
			cmdName,
			&copyInfos,
			orig,
			dest,
			allowRemote,
			allowDecompression,
			true,
		); err != nil {
			return s, err
		}
	}

	if len(copyInfos) == 0 {
		return s, fmt.Errorf("No source files were specified")
	}
	if len(copyInfos) > 1 && !strings.HasSuffix(dest, string(os.PathSeparator)) {
		return s, fmt.Errorf("When using %s with more than one source file, the destination must be a directory and end with a /", cmdName)
	}

	// For backwards compat, if there's just one CI then use it as the
	// cache look-up string, otherwise hash 'em all into one
	var srcHash string
	// var origPaths string

	if len(copyInfos) == 1 {
		srcHash = copyInfos[0].hash
		// origPaths = copyInfos[0].origPath
	} else {
		var hashs []string
		var origs []string
		for _, ci := range copyInfos {
			hashs = append(hashs, ci.hash)
			origs = append(origs, ci.origPath)
		}
		hasher := sha256.New()
		hasher.Write([]byte(strings.Join(hashs, ",")))
		srcHash = "multi:" + hex.EncodeToString(hasher.Sum(nil))
		// origPaths = strings.Join(origs, " ")
	}

	s.commitMsg = append(s.commitMsg, fmt.Sprintf("%s %s in %s", cmdName, srcHash, dest))

	// TODO: probe cache

	// TODO: do the actual copy

	// for _, ci := range copyInfos {
	// 	if err := b.addContext(container, ci.origPath, ci.destPath, ci.decompress); err != nil {
	// 		return err
	// 	}
	// }

	return s, nil
}

func calcCopyInfo(b *Build, cmdName string, cInfos *[]*copyInfo, origPath string, destPath string, allowRemote bool, allowDecompression bool, allowWildcards bool) error {

	// Work in daemon-specific OS filepath semantics. However, we save
	// the the origPath passed in here, as it might also be a URL which
	// we need to check for in this function.
	passedInOrigPath := origPath
	origPath = filepath.FromSlash(origPath)
	destPath = filepath.FromSlash(destPath)

	if origPath != "" && origPath[0] == os.PathSeparator && len(origPath) > 1 {
		origPath = origPath[1:]
	}
	origPath = strings.TrimPrefix(origPath, "."+string(os.PathSeparator))

	// Twiddle the destPath when its a relative path - meaning, make it
	// relative to the WORKINGDIR
	if !filepath.IsAbs(destPath) {
		hasSlash := strings.HasSuffix(destPath, string(os.PathSeparator))
		destPath = filepath.Join(string(os.PathSeparator), filepath.FromSlash(b.state.config.WorkingDir), destPath)

		// Make sure we preserve any trailing slash
		if hasSlash {
			destPath += string(os.PathSeparator)
		}
	}

	// In the remote/URL case, download it and gen its hashcode
	if urlutil.IsURL(passedInOrigPath) {

		// As it's a URL, we go back to processing on what was passed in
		// to this function
		origPath = passedInOrigPath

		if !allowRemote {
			return fmt.Errorf("Source can't be a URL for %s", cmdName)
		}

		ci := copyInfo{}
		ci.origPath = origPath
		ci.hash = origPath // default to this but can change
		ci.destPath = destPath
		ci.decompress = false
		*cInfos = append(*cInfos, &ci)

		// Initiate the download
		resp, err := httputils.Download(ci.origPath)
		if err != nil {
			return err
		}

		// Create a tmp dir
		tmpDirName, err := ioutil.TempDir(b.cfg.ContextDir, "docker-remote")
		if err != nil {
			return err
		}
		ci.tmpDir = tmpDirName

		// Create a tmp file within our tmp dir
		tmpFileName := filepath.Join(tmpDirName, "tmp")
		tmpFile, err := os.OpenFile(tmpFileName, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}

		// Download and dump result to tmp file
		// TODO: adopt Docker's progressreader?
		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			tmpFile.Close()
			return err
		}
		tmpFile.Close()

		// Set the mtime to the Last-Modified header value if present
		// Otherwise just remove atime and mtime
		times := make([]syscall.Timespec, 2)

		lastMod := resp.Header.Get("Last-Modified")
		if lastMod != "" {
			mTime, err := http.ParseTime(lastMod)
			// If we can't parse it then just let it default to 'zero'
			// otherwise use the parsed time value
			if err == nil {
				times[1] = syscall.NsecToTimespec(mTime.UnixNano())
			}
		}

		if err := system.UtimesNano(tmpFileName, times); err != nil {
			return err
		}

		ci.origPath = filepath.Join(filepath.Base(tmpDirName), filepath.Base(tmpFileName))

		// If the destination is a directory, figure out the filename.
		if strings.HasSuffix(ci.destPath, string(os.PathSeparator)) {
			u, err := url.Parse(origPath)
			if err != nil {
				return err
			}
			path := u.Path
			if strings.HasSuffix(path, string(os.PathSeparator)) {
				path = path[:len(path)-1]
			}
			parts := strings.Split(path, string(os.PathSeparator))
			filename := parts[len(parts)-1]
			if filename == "" {
				return fmt.Errorf("cannot determine filename from url: %s", u)
			}
			ci.destPath = ci.destPath + filename
		}

		// Calc the checksum, even if we're using the cache
		r, err := archive.Tar(tmpFileName, archive.Uncompressed)
		if err != nil {
			return err
		}
		tarSum, err := tarsum.NewTarSum(r, true, tarsum.Version1)
		if err != nil {
			return err
		}
		if _, err := io.Copy(ioutil.Discard, tarSum); err != nil {
			return err
		}
		ci.hash = tarSum.Sum(nil)
		r.Close()

		return nil
	}

	// TODO: Deal with wildcards
	// if allowWildcards && containsWildcards(origPath) {
	// 	for _, fileInfo := range b.context.GetSums() {
	// 		if fileInfo.Name() == "" {
	// 			continue
	// 		}
	// 		match, _ := filepath.Match(origPath, fileInfo.Name())
	// 		if !match {
	// 			continue
	// 		}

	// 		// Note we set allowWildcards to false in case the name has
	// 		// a * in it
	// 		calcCopyInfo(b, cmdName, cInfos, fileInfo.Name(), destPath, allowRemote, allowDecompression, false)
	// 	}
	// 	return nil
	// }

	// Must be a dir or a file

	if err := checkPathForAddition(b, origPath); err != nil {
		return err
	}
	fi, _ := os.Stat(filepath.Join(b.cfg.ContextDir, origPath))

	ci := copyInfo{}
	ci.origPath = origPath
	ci.hash = origPath
	ci.destPath = destPath
	ci.decompress = allowDecompression
	*cInfos = append(*cInfos, &ci)

	// Deal with the single file case
	if !fi.IsDir() {
		r, err := archive.Tar(ci.origPath, archive.Uncompressed)
		if err != nil {
			return err
		}
		tarSum, err := tarsum.NewTarSum(r, true, tarsum.Version1)
		if err != nil {
			return err
		}
		if _, err := io.Copy(ioutil.Discard, tarSum); err != nil {
			return err
		}
		ci.hash = "file:" + tarSum.Sum(nil)
		r.Close()

		// This will match first file in sums of the archive
		// fis := b.context.GetSums().GetFile(ci.origPath)
		// if fis != nil {
		// 	ci.hash = "file:" + fis.Sum()
		// }
		return nil
	}

	// TODO: tarsum for dirs
	//       NewTarWithOptions might do the trick

	// Must be a dir
	// var subfiles []string
	// absOrigPath := filepath.Join(b.cfg.ContextDir, ci.origPath)

	// // Add a trailing / to make sure we only pick up nested files under
	// // the dir and not sibling files of the dir that just happen to
	// // start with the same chars
	// if !strings.HasSuffix(absOrigPath, string(os.PathSeparator)) {
	// 	absOrigPath += string(os.PathSeparator)
	// }

	// // Need path w/o slash too to find matching dir w/o trailing slash
	// absOrigPathNoSlash := absOrigPath[:len(absOrigPath)-1]

	// for _, fileInfo := range b.context.GetSums() {
	// 	absFile := filepath.Join(b.contextPath, fileInfo.Name())
	// 	// Any file in the context that starts with the given path will be
	// 	// picked up and its hashcode used.  However, we'll exclude the
	// 	// root dir itself.  We do this for a coupel of reasons:
	// 	// 1 - ADD/COPY will not copy the dir itself, just its children
	// 	//     so there's no reason to include it in the hash calc
	// 	// 2 - the metadata on the dir will change when any child file
	// 	//     changes.  This will lead to a miss in the cache check if that
	// 	//     child file is in the .dockerignore list.
	// 	if strings.HasPrefix(absFile, absOrigPath) && absFile != absOrigPathNoSlash {
	// 		subfiles = append(subfiles, fileInfo.Sum())
	// 	}
	// }
	// sort.Strings(subfiles)
	// hasher := sha256.New()
	// hasher.Write([]byte(strings.Join(subfiles, ",")))
	// ci.hash = "dir:" + hex.EncodeToString(hasher.Sum(nil))

	return nil
}

func containsWildcards(name string) bool {
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

func checkPathForAddition(b *Build, orig string) error {
	origPath := filepath.Join(b.cfg.ContextDir, orig)
	origPath, err := filepath.EvalSymlinks(origPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	}
	if _, err := os.Stat(origPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s: no such file or directory", orig)
		}
		return err
	}
	return nil
}
