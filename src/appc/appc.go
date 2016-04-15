package appc

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/appc/docker2aci/lib"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/docker/go-units"
	"github.com/fsouza/go-dockerclient"
	"github.com/grammarly/rocker/src/imagename"

	log "github.com/Sirupsen/logrus"
)

// Appc is appc
type Appc struct {
	client  *docker.Client
	s3      *s3.S3
	retryer *Retryer
}

// Manifest is manifest
type Manifest struct {
	Config   string
	RepoTags []string
	Layers   []string
}

// New makes new Appc
func New(client *docker.Client) *Appc {
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

	return &Appc{
		client:  client,
		s3:      s3.New(session.New(), cfg),
		retryer: retryer,
	}
}

// Push converts docker image to ACI and pushes to S3
func (a *Appc) Push(imageName, digest string) (digest2 string, err error) {
	img := imagename.NewFromString(imageName)

	var image *docker.Image
	if image, err = a.client.InspectImage(img.String()); err != nil {
		return "", err
	}

	var (
		ext           = ".aci"
		bucket        = "funday-bucket"
		prefix        = "repo/"
		imgPathDigest = path.Join(prefix, img.Name, digest+ext)
		imgPathTag    = path.Join(prefix, img.Name, img.Tag+ext)
	)

	// Make HEAD request to s3 and check if image already uploaded
	_, headErr := a.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(imgPathDigest),
	})

	// Object not found, need to store
	if headErr != nil {
		// Other error, raise then
		if e, ok := headErr.(awserr.RequestFailure); !ok || e.StatusCode() != 404 {
			return "", headErr
		}
		// In case we do not have archive
		tmpf, err := ioutil.TempFile("", "rocker_appc_convert_")
		if err != nil {
			return "", err
		}
		defer os.Remove(tmpf.Name())

		tmpdir, err := ioutil.TempDir("", "rocker_appc_convert_")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(tmpdir)

		var (
			humanSize = units.HumanSize(float64(image.VirtualSize))

			exportParams = docker.ExportImageOptions{
				Name:         img.String(),
				OutputStream: tmpf,
			}
		)

		log.Infof("| Buffering image %s to a file %s (%s)", img, tmpf.Name(), humanSize)

		if err := a.client.ExportImage(exportParams); err != nil {
			return "", err
		}

		log.Infof("| Converting image %s to ACI located in %s", img, tmpdir)

		// TODO: cache ACI images by digests?

		res, err := docker2aci.ConvertSavedFile(tmpf.Name(), docker2aci.FileConfig{
			CommonConfig: docker2aci.CommonConfig{
				Squash:    true,
				OutputDir: tmpdir,
			},
		})
		if err != nil {
			return "", err
		}

		if len(res) != 1 {
			return "", fmt.Errorf("Expecting to get a single file from docker2aci converter, got %q", res)
		}

		uploader := s3manager.NewUploaderWithClient(a.s3, func(u *s3manager.Uploader) {
			u.PartSize = 64 * 1024 * 1024 // 64MB per part
		})

		fd, err := os.Open(res[0])
		if err != nil {
			return "", err
		}
		defer fd.Close()

		log.Infof("| Uploading image to s3.amazonaws.com/%s/%s", bucket, imgPathDigest)

		uploadParams := &s3manager.UploadInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(imgPathDigest),
			ContentType: aws.String("application/x-tar"),
			Body:        fd,
			Metadata: map[string]*string{
				"Tag":     aws.String(img.Tag),
				"ImageID": aws.String(image.ID),
				"Digest":  aws.String(digest),
			},
		}

		if err := a.retryer.Outer(func() error {
			_, err := uploader.Upload(uploadParams)
			return err
		}); err != nil {
			return "", fmt.Errorf("Failed to upload object to S3, error: %s", err)
		}
	}

	// Make a content addressable copy of an image file
	copyParams := &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(bucket + "/" + imgPathDigest),
		Key:        aws.String(imgPathTag),
	}

	log.Infof("| Make alias s3.amazonaws.com/%s/%s", bucket, imgPathTag)

	if _, err = a.s3.CopyObject(copyParams); err != nil {
		return "", fmt.Errorf("Failed to PUT object to S3, error: %s", err)
	}

	return digest, nil
}
