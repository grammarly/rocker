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

package s3

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"rocker/imagename"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/docker/docker/pkg/units"
	"github.com/fsouza/go-dockerclient"
)

const (
	cacheDir = "_digests"
)

// StorageS3 is a storage driver that implements storing docker images directly on S3
type StorageS3 struct {
	client    *docker.Client
	cacheRoot string
	s3        *s3.S3
}

// New makes an instance of StorageS3 storage driver
func New(client *docker.Client, cacheRoot string) *StorageS3 {
	// TODO: configure region?
	return &StorageS3{
		client:    client,
		cacheRoot: cacheRoot,
		s3:        s3.New(session.New(), &aws.Config{Region: aws.String("us-east-1")}),
	}
}

// Push pushes image tarball directly to S3
func (s *StorageS3) Push(imageName string) (digest string, err error) {
	img := imagename.NewFromString(imageName)

	if img.Storage != imagename.StorageS3 {
		return "", fmt.Errorf("Can only push images with s3 storage specified, got: %s", img)
	}

	if img.Registry == "" {
		return "", fmt.Errorf("Cannot push image to S3, missing bucket name, got: %s", img)
	}

	var image *docker.Image
	if image, err = s.client.InspectImage(img.String()); err != nil {
		return "", err
	}

	if digest, err = s.CacheGet(image.ID); err != nil {
		return "", err
	}

	var tmpf string

	defer func() {
		if tmpf != "" {
			os.Remove(tmpf)
		}
	}()

	// Not cached, make tar
	if digest == "" {
		if tmpf, digest, err = s.MakeTar(imageName); err != nil {
			return "", err
		}
	}

	ext := ".tar"
	imgPathDigest := fmt.Sprintf("%s/%s%s", img.Name, digest, ext)
	imgPathTag := fmt.Sprintf("%s/%s%s", img.Name, img.Tag, ext)

	// Make HEAD request to s3 and check if image already uploaded
	// TODO: handle not found correctly
	_, headErr := s.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(img.Registry),
		Key:    aws.String(imgPathDigest),
	})

	// Object not found, need to store
	if headErr != nil {
		// Other error, raise then
		if e, ok := headErr.(awserr.RequestFailure); !ok || e.StatusCode() != 404 {
			return "", err
		}
		// In case we do not have archive
		if tmpf == "" {
			var digest2 string
			if tmpf, digest2, err = s.MakeTar(imageName); err != nil {
				return "", err
			}
			// Verify digest (TODO: remote this check in future)
			if digest != digest2 {
				return "", fmt.Errorf("The new digest does no equal old one (shouldn't happen) %s != %s", digest, digest2)
			}
		}

		uploader := s3manager.NewUploaderWithClient(s.s3, func(u *s3manager.Uploader) {
			u.PartSize = 64 * 1024 * 1024 // 64MB per part
		})

		fd, err := os.Open(tmpf)
		if err != nil {
			return "", err
		}
		defer fd.Close()

		log.Infof("| Uploading image to s3://%s/%s", img.Registry, imgPathDigest)

		upParams := &s3manager.UploadInput{
			Bucket:      aws.String(img.Registry),
			Key:         aws.String(imgPathDigest),
			ContentType: aws.String("application/x-tar"),
			Body:        fd,
			Metadata: map[string]*string{
				"Tag":     aws.String(img.Tag),
				"ImageID": aws.String(image.ID),
				"Digest":  aws.String(digest),
			},
		}

		if _, err := uploader.Upload(upParams); err != nil {
			return "", fmt.Errorf("Failed to upload object to S3, error: %s", err)
		}
	}

	// Make a content addressable copy of an image file
	copyParams := &s3.CopyObjectInput{
		Bucket:     aws.String(img.Registry),
		CopySource: aws.String(img.Registry + "/" + imgPathDigest),
		Key:        aws.String(imgPathTag),
	}

	log.Infof("| Make alias s3://%s/%s", img.Registry, imgPathTag)

	if _, err = s.s3.CopyObject(copyParams); err != nil {
		return "", fmt.Errorf("Failed to PUT object to S3, error: %s", err)
	}

	return digest, nil
}

// Pull imports docker image from tar artifact stored on S3
func (s *StorageS3) Pull(name string) (*docker.Image, error) {
	img := imagename.NewFromString(name)

	if img.Storage != imagename.StorageS3 {
		return nil, fmt.Errorf("Can only pull images with s3 storage specified, got: %s", img)
	}

	if img.Registry == "" {
		return nil, fmt.Errorf("Cannot pull image from S3, missing bucket name, got: %s", img)
	}

	// TODO: here we use tmp file, but we can stream from S3 directly to Docker
	tmpf, err := ioutil.TempFile("", "rocker_image_")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpf.Name())

	// Create a downloader with the s3 client and custom options
	downloader := s3manager.NewDownloaderWithClient(s.s3, func(d *s3manager.Downloader) {
		d.PartSize = 64 * 1024 * 1024 // 64MB per part
	})

	imgPath := img.Name + "/" + img.Tag + ".tar"

	downloadParams := &s3.GetObjectInput{
		Bucket: aws.String(img.Registry),
		Key:    aws.String(imgPath),
	}

	log.Infof("| Import s3://%s/%s to %s", img.Registry, imgPath, tmpf.Name())

	if _, err := downloader.Download(tmpf, downloadParams); err != nil {
		return nil, fmt.Errorf("Failed to download object from S3, error: %s", err)
	}

	fd, err := os.Open(tmpf.Name())
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	loadOptions := docker.LoadImageOptions{
		InputStream: fd,
	}

	if err := s.client.LoadImage(loadOptions); err != nil {
		return nil, fmt.Errorf("Failed to import image, error: %s", err)
	}

	image, err := s.client.InspectImage(img.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to inspect image %s after pull, error: %s", img, err)
	}

	return image, nil
}

// MakeTar makes a tar out of docker image and gives a temporary file and a digest
func (s *StorageS3) MakeTar(imageName string) (tmpfile string, digest string, err error) {
	img := imagename.NewFromString(imageName)

	var image *docker.Image
	if image, err = s.client.InspectImage(img.String()); err != nil {
		return "", "", err
	}

	humanSize := units.HumanSize(float64(image.VirtualSize))

	// TODO: here we use tmp file, but we can stream to S3 directly from Docker
	// https://github.com/aws/aws-sdk-go/issues/272
	tmpf, err := ioutil.TempFile("", "rocker_image_")
	if err != nil {
		return "", "", err
	}
	defer tmpf.Close()

	cleanup := func() {
		os.Remove(tmpf.Name())
	}

	pipeReader, pipeWriter := io.Pipe()
	defer pipeWriter.Close()

	opts := docker.ExportImageOptions{
		Name:         img.String(),
		OutputStream: pipeWriter,
	}

	tr := tar.NewReader(pipeReader)
	tw := tar.NewWriter(tmpf)
	hash := sha256.New()
	tarHashStream := io.MultiWriter(tw, hash)

	log.Infof("| Buffering image to a file %s (%s)", tmpf.Name(), humanSize)

	errch := make(chan error, 1)

	go func() {
		errch <- s.client.ExportImage(opts)
	}()

	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			cleanup()
			return "", "", err
		}
		// fmt.Printf("Contents of %s\n", hdr.Name)
		// We will write our own repositories
		if hdr.Name == "repositories" {
			io.Copy(ioutil.Discard, tr)
			continue
		}
		tw.WriteHeader(hdr)
		if _, err := io.Copy(tarHashStream, tr); err != nil {
			cleanup()
			return "", "", err
		}
	}

	// Write repositories
	digest = fmt.Sprintf("sha256-%x", hash.Sum(nil))

	repos := map[string]map[string]string{
		img.NameWithRegistry(): map[string]string{
			img.Tag: image.ID,
			digest:  image.ID,
		},
	}
	reposBody, err := json.Marshal(repos)
	if err != nil {
		cleanup()
		return "", "", err
	}

	hdr := &tar.Header{
		Name: "repositories",
		Mode: 0644,
		Size: int64(len(reposBody)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		cleanup()
		return "", "", err
	}
	if _, err := tw.Write(reposBody); err != nil {
		cleanup()
		return "", "", err
	}

	if err := tw.Close(); err != nil {
		cleanup()
		return "", "", err
	}

	if err := <-errch; err != nil {
		cleanup()
		return "", "", fmt.Errorf("Failed to export docker image %s, error: %s", img, err)
	}

	if err := s.CachePut(image.ID, digest); err != nil {
		return "", "", fmt.Errorf("Failed to save digest cache, error: %s", err)
	}

	return tmpf.Name(), digest, nil
}

// CacheGet returns cached digest of the image
func (s *StorageS3) CacheGet(imageID string) (digest string, err error) {
	fileName := filepath.Join(s.cacheRoot, cacheDir, imageID)

	data, err := ioutil.ReadFile(fileName)
	if err != nil && os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// CachePut stores digest cache of the image
func (s *StorageS3) CachePut(imageID, digest string) error {
	fileName := filepath.Join(s.cacheRoot, cacheDir, imageID)

	if err := os.MkdirAll(filepath.Dir(fileName), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(fileName, []byte(digest), 0644)
}
