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
	"strings"

	"github.com/grammarly/rocker/src/imagename"

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

// Repositories is a struct that serializes to a "repositories" file
type Repositories map[string]Repository

// Repository is an entity of Repositories struct
type Repository map[string]string

// StorageS3 is a storage driver that implements storing docker images directly on S3
type StorageS3 struct {
	client    *docker.Client
	cacheRoot string
	s3        *s3.S3
	retryer   *Retryer
}

// New makes an instance of StorageS3 storage driver
func New(client *docker.Client, cacheRoot string) *StorageS3 {
	retryer := NewRetryer(400, 6)

	// TODO: configure region?
	cfg := &aws.Config{
		Region:  aws.String("us-east-1"),
		Retryer: retryer,
		Logger:  &Logger{},
	}

	if log.StandardLogger().Level >= log.DebugLevel {
		cfg.LogLevel = aws.LogLevel(aws.LogDebugWithRequestErrors)
	}

	return &StorageS3{
		client:    client,
		cacheRoot: cacheRoot,
		s3:        s3.New(session.New(), cfg),
		retryer:   retryer,
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

	var (
		ext           = ".tar"
		imgPathDigest = fmt.Sprintf("%s/%s%s", img.Name, digest, ext)
		imgPathTag    = fmt.Sprintf("%s/%s%s", img.Name, img.Tag, ext)
	)

	// Make HEAD request to s3 and check if image already uploaded
	_, headErr := s.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(img.Registry),
		Key:    aws.String(imgPathDigest),
	})

	// Object not found, need to store
	if headErr != nil {
		// Other error, raise then
		if e, ok := headErr.(awserr.RequestFailure); !ok || e.StatusCode() != 404 {
			return "", headErr
		}
		// In case we do not have archive
		if tmpf == "" {
			var digest2 string
			if tmpf, digest2, err = s.MakeTar(imageName); err != nil {
				return "", err
			}
			// Verify digest (TODO: remote this check in future?)
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

		log.Infof("| Uploading image to s3.amazonaws.com/%s/%s", img.Registry, imgPathDigest)

		uploadParams := &s3manager.UploadInput{
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

		if err := s.retryer.Outer(func() error {
			_, err := uploader.Upload(uploadParams)
			return err
		}); err != nil {
			return "", fmt.Errorf("Failed to upload object to S3, error: %s", err)
		}
	}

	// Make a content addressable copy of an image file
	copyParams := &s3.CopyObjectInput{
		Bucket:     aws.String(img.Registry),
		CopySource: aws.String(img.Registry + "/" + imgPathDigest),
		Key:        aws.String(imgPathTag),
	}

	log.Infof("| Make alias s3.amazonaws.com/%s/%s", img.Registry, imgPathTag)

	if _, err = s.s3.CopyObject(copyParams); err != nil {
		return "", fmt.Errorf("Failed to PUT object to S3, error: %s", err)
	}

	return digest, nil
}

// Pull imports docker image from tar artifact stored on S3
func (s *StorageS3) Pull(name string) error {
	img := imagename.NewFromString(name)

	if img.Storage != imagename.StorageS3 {
		return fmt.Errorf("Can only pull images with s3 storage specified, got: %s", img)
	}

	if img.Registry == "" {
		return fmt.Errorf("Cannot pull image from S3, missing bucket name, got: %s", img)
	}

	// TODO: here we use tmp file, but we can stream from S3 directly to Docker
	tmpf, err := ioutil.TempFile("", "rocker_image_")
	if err != nil {
		return err
	}
	defer os.Remove(tmpf.Name())

	var (
		// Create a downloader with the s3 client and custom options
		downloader = s3manager.NewDownloaderWithClient(s.s3, func(d *s3manager.Downloader) {
			d.PartSize = 64 * 1024 * 1024 // 64MB per part
		})

		imgPath = img.Name + "/" + img.Tag + ".tar"

		downloadParams = &s3.GetObjectInput{
			Bucket: aws.String(img.Registry),
			Key:    aws.String(imgPath),
		}
	)

	log.Infof("| Import %s/%s.tar to %s", img.NameWithRegistry(), img.Tag, tmpf.Name())

	if err := s.retryer.Outer(func() error {
		_, err := downloader.Download(tmpf, downloadParams)
		return err
	}); err != nil {
		return fmt.Errorf("Failed to download object from S3, error: %s", err)
	}

	fd, err := os.Open(tmpf.Name())
	if err != nil {
		return err
	}
	defer fd.Close()

	// Read through tar reader to patch repositories file since we might
	// mave a different tag property
	var (
		pipeReader, pipeWriter = io.Pipe()
		tr                     = tar.NewReader(fd)
		tw                     = tar.NewWriter(pipeWriter)
		errch                  = make(chan error, 1)

		loadOptions = docker.LoadImageOptions{
			InputStream: pipeReader,
		}
	)

	go func() {
		defer pipeReader.Close()
		err := s.client.LoadImage(loadOptions)
		if err != nil {
			log.Errorf("LoadImage error: %v", err)
		}
		errch <- err
	}()

	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("Failed to read tar content, error: %s", err)
		}

		// Skip "repositories" file, we will write our own
		if hdr.Name == "repositories" {
			// Read repositories file and pass to JSON decoder
			r1 := Repositories{}
			data, err := ioutil.ReadAll(tr)
			if err != nil {
				return fmt.Errorf("Failed to read `repositories` file content, error: %s", err)
			}
			if err := json.Unmarshal(data, &r1); err != nil {
				return fmt.Errorf("Failed to parse `repositories` file json, error: %s", err)
			}

			var imageID string

			// Read first key from repositories
			for _, tags := range r1 {
				for _, id := range tags {
					imageID = id
					break
				}
				break
			}

			// Make a new repositories struct
			r2 := Repositories{
				img.NameWithRegistry(): {
					img.GetTag(): imageID,
				},
			}

			// Write repositories file to the stream
			reposBody, err := json.Marshal(r2)
			if err != nil {
				return fmt.Errorf("Failed to marshal `repositories` file json, error: %s", err)
			}

			hdr := &tar.Header{
				Name: "repositories",
				Mode: 0644,
				Size: int64(len(reposBody)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return fmt.Errorf("Failed to write `repositories` file tar header, error: %s", err)
			}
			if _, err := tw.Write(reposBody); err != nil {
				return fmt.Errorf("Failed to write `repositories` file to tar, error: %s", err)
			}

			continue
		}

		// Passthrough other files as is
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("Failed to passthough tar header, error: %s", err)
		}
		if _, err := io.Copy(tw, tr); err != nil {
			return fmt.Errorf("Failed to passthough tar content, error: %s", err)
		}
	}

	// Finish tar
	if err := tw.Close(); err != nil {
		return fmt.Errorf("Failed to close tar writer, error: %s", err)
	}

	// Close pipeWriter
	if err := pipeWriter.Close(); err != nil {
		return fmt.Errorf("Failed to close tar pipeWriter, error: %s", err)
	}

	if err := <-errch; err != nil {
		errch <- fmt.Errorf("Failed to import image, error: %s", err)
	}

	return nil
}

// MakeTar makes a tar out of docker image and gives a temporary file and a digest
func (s *StorageS3) MakeTar(imageName string) (tmpfile string, digest string, err error) {
	img := imagename.NewFromString(imageName)

	var image *docker.Image
	if image, err = s.client.InspectImage(img.String()); err != nil {
		return "", "", err
	}

	tmpf, err := ioutil.TempFile("", "rocker_image_")
	if err != nil {
		return "", "", err
	}
	defer tmpf.Close()

	var (
		cleanup = func() {
			os.Remove(tmpf.Name())
		}

		humanSize = units.HumanSize(float64(image.VirtualSize))
		errch     = make(chan error, 1)

		pipeReader, pipeWriter = io.Pipe()
		tr                     = tar.NewReader(pipeReader)
		tw                     = tar.NewWriter(tmpf)
		hash                   = sha256.New()
		tarHashStream          = io.MultiWriter(tw, hash)

		exportParams = docker.ExportImageOptions{
			Name:         img.String(),
			OutputStream: pipeWriter,
		}
	)

	defer pipeWriter.Close()

	log.Infof("| Buffering image to a file %s (%s)", tmpf.Name(), humanSize)

	go func() {
		errch <- s.client.ExportImage(exportParams)
	}()

	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup()
			return "", "", err
		}

		// Skip "repositories" file, we will write our own
		if hdr.Name == "repositories" {
			io.Copy(ioutil.Discard, tr)
			continue
		}

		// Write any other file
		tw.WriteHeader(hdr)
		if _, err := io.Copy(tarHashStream, tr); err != nil {
			cleanup()
			return "", "", err
		}
	}

	// Write "repositories" file
	digest = fmt.Sprintf("sha256-%x", hash.Sum(nil))

	reposBody, err := json.Marshal(Repositories{
		img.NameWithRegistry(): {
			img.Tag: image.ID,
			digest:  image.ID,
		},
	})
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

	// Finish tar

	if err := tw.Close(); err != nil {
		cleanup()
		return "", "", err
	}

	if err := <-errch; err != nil {
		cleanup()
		return "", "", fmt.Errorf("Failed to export docker image %s, error: %s", img, err)
	}

	// Cache digest by image ID
	if err := s.CachePut(image.ID, digest); err != nil {
		return "", "", fmt.Errorf("Failed to save digest cache, error: %s", err)
	}

	return tmpf.Name(), digest, nil
}

// ListTags returns the list of parsed tags existing for given image name on S3
func (s *StorageS3) ListTags(imageName string) (images []*imagename.ImageName, err error) {
	image := imagename.NewFromString(imageName)

	params := &s3.ListObjectsInput{
		Bucket:  aws.String(image.Registry),
		MaxKeys: aws.Int64(1000),
		Prefix:  aws.String(image.Name),
	}

	resp, err := s.s3.ListObjects(params)
	if err != nil {
		return nil, err
	}

	for _, s3Obj := range resp.Contents {
		split := strings.Split(*s3Obj.Key, "/")
		if len(split) < 2 {
			continue
		}

		imgName := strings.Join(split[:len(split)-1], "/")
		imgName = fmt.Sprintf("s3.amazonaws.com/%s/%s", image.Registry, imgName)

		tag := strings.TrimSuffix(split[len(split)-1], ".tar")
		candidate := imagename.New(imgName, tag)

		if candidate.Name != image.Name {
			continue
		}

		if image.Contains(candidate) || image.Tag == candidate.Tag {
			images = append(images, candidate)
		}
	}

	return
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
