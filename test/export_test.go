package tests

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestExportSimple(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

	err = runRockerBuildWithOptions(`
		FROM alpine:latest
		RUN echo -n "test_export" > /exported_file
		EXPORT /exported_file

		FROM alpine:latest
		MOUNT `+dir+`:/datadir
		IMPORT /exported_file /datadir/imported_file`, "--no-cache")
	assert.Nil(t, err, "Failed to run rocker build")

	content, err := ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err, "Can't read file")

	assert.Equal(t, "test_export", string(content))
}

func TestExportSeparateFilesDifferentExport(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

	rockerContentFirst := `FROM alpine:latest
						   EXPORT /etc/hostname
						   RUN echo -n "first_diff" > /exported_file
						   EXPORT /exported_file

						   FROM alpine
						   MOUNT ` + dir + `:/datadir
						   IMPORT /exported_file /datadir/imported_file`

	rockerContentSecond := `FROM alpine:latest
						    EXPORT /etc/issue
						    RUN echo -n "second_diff" > /exported_file
						    EXPORT /exported_file 

						    FROM alpine
						    MOUNT ` + dir + `:/datadir
						    IMPORT /exported_file /datadir/imported_file`

	err = runRockerBuildWithOptions(rockerContentFirst, "--reload-cache")
	assert.Nil(t, err)

	err = runRockerBuildWithOptions(rockerContentSecond, "--reload-cache")
	assert.Nil(t, err)

	err = runRockerBuildWithOptions(rockerContentFirst)
	assert.Nil(t, err)

	content, err := ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err)
	assert.Equal(t, "first_diff", string(content))
}

func TestExportSmolinIssue(t *testing.T) {
	tag := "rocker-integratin-test-export-smolin"
	defer removeImage(tag + ":qa")
	defer removeImage(tag + ":prod")

	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_smolin")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

	rockerfile, err := createTempFile("")
	assert.Nil(t, err, "Can't create temp file")
	defer os.RemoveAll(rockerfile)
	randomData := strconv.Itoa(int(time.Now().UnixNano() % int64(100000001)))

	rockerContentFirst := []byte(` {{ $env := .env}}
							 FROM alpine
							 RUN echo -n "{{ $env }}" > /exported_file
						 	 EXPORT /exported_file

							 FROM alpine
							 IMPORT /exported_file /imported_file
							 TAG ` + tag + ":{{ $env }}")

	rockerContentSecond := []byte(` {{ $env := .env}}
							 FROM alpine
							 RUN echo -n "{{ $env }}" > /exported_file
						 	 EXPORT /exported_file

							 FROM alpine
							 RUN echo "invalidate with ` + randomData + `"
							 IMPORT /exported_file /imported_file
							 TAG ` + tag + ":{{ $env }}")

	err = ioutil.WriteFile(rockerfile, rockerContentFirst, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile, "--reload-cache", "--var", "env=qa")
	assert.Nil(t, err)

	err = ioutil.WriteFile(rockerfile, rockerContentFirst, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile, "--reload-cache", "--var", "env=prod")
	assert.Nil(t, err)

	err = ioutil.WriteFile(rockerfile, rockerContentSecond, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile, "--var", "env=qa")
	assert.Nil(t, err)

	content := `FROM ` + tag + `:qa
					   MOUNT ` + dir + `:/data
					   RUN cp /imported_file /data/qa.file

					   FROM ` + tag + `:prod
					   MOUNT ` + dir + `:/data
					   RUN cp /imported_file /data/prod.file`
	err = runRockerBuildWithOptions(content, "--no-cache")
	assert.Nil(t, err)

	qaContent, err := ioutil.ReadFile(dir + "/qa.file")
	assert.Nil(t, err)
	assert.Equal(t, string(qaContent), "qa")

	prodContent, err := ioutil.ReadFile(dir + "/prod.file")
	assert.Nil(t, err)
	assert.Equal(t, string(prodContent), "prod")

}
func TestExportSeparateFilesSameExport(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_sep")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

	rockerContentFirst := `FROM alpine:latest
						   EXPORT /etc/issue
						   RUN echo -n "first_separate" > /exported_file
						   EXPORT /exported_file

						   FROM alpine
						   MOUNT ` + dir + `:/datadir
						   IMPORT /exported_file /datadir/imported_file
						   `

	rockerContentSecond := `FROM alpine:latest
						    EXPORT /etc/issue
						    RUN echo -n "second_separate" > /exported_file
						    EXPORT /exported_file

						    FROM alpine
						    MOUNT ` + dir + `:/datadir
						    IMPORT /exported_file /datadir/imported_file`

	rockerContentThird := `FROM alpine:latest
						   EXPORT /etc/issue
						   RUN echo -n "first_separate" > /exported_file
						   EXPORT /exported_file

						   FROM alpine
						   MOUNT ` + dir + `:/datadir
						   RUN echo "Invalidate cache"
						   IMPORT /exported_file /datadir/imported_file
						   `

	err = runRockerBuildWithOptions(rockerContentFirst, "--reload-cache")
	assert.Nil(t, err)

	err = runRockerBuildWithOptions(rockerContentSecond)
	assert.Nil(t, err)

	err = runRockerBuildWithOptions(rockerContentThird)
	assert.Nil(t, err)

	content, err := ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err)
	assert.Equal(t, "first_separate", string(content))
}

func TestExportSameFileDifferentCmd(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

	rockerfile, err := createTempFile("")
	assert.Nil(t, err, "Can't create temp file")
	defer os.RemoveAll(rockerfile)

	rockerContentFirst := []byte(`FROM alpine
						 	 RUN echo -n "first_foobar1" > /exported_file
						 	 EXPORT /exported_file
						 	 FROM alpine
						 	 MOUNT ` + dir + `:/datadir
						 	 IMPORT /exported_file /datadir/imported_file`)

	rockerContentSecond := []byte(`FROM alpine
							  RUN echo -n "second_foobar1" > /exported_file
							  EXPORT /exported_file
							  FROM alpine
							  MOUNT ` + dir + `:/datadir
							  IMPORT /exported_file /datadir/imported_file`)

	rockerContentThird := []byte(`FROM alpine
						 	 RUN echo -n "first_foobar1" > /exported_file
						 	 EXPORT /exported_file
						 	 FROM alpine
						 	 MOUNT ` + dir + `:/datadir
							 RUN echo "Invalidate cache"
						 	 IMPORT /exported_file /datadir/imported_file`)

	err = ioutil.WriteFile(rockerfile, rockerContentFirst, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile, "--reload-cache")
	assert.Nil(t, err)
	content, err := ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err)
	assert.Equal(t, "first_foobar1", string(content))

	err = ioutil.WriteFile(rockerfile, rockerContentSecond, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile)
	assert.Nil(t, err)
	content, err = ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err)
	assert.Equal(t, "second_foobar1", string(content))

	err = ioutil.WriteFile(rockerfile, rockerContentThird, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile)
	assert.Nil(t, err)
	content, err = ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err, "Can't read file")
	assert.Equal(t, "first_foobar1", string(content))
}

func TestExportSameFileFewFroms(t *testing.T) {
	dir, err := ioutil.TempDir("/tmp", "rocker_integration_test_export_")
	assert.Nil(t, err, "Can't create tmp dir")
	defer os.RemoveAll(dir)

	rockerfile, err := createTempFile("")
	assert.Nil(t, err, "Can't create temp file")
	defer os.RemoveAll(rockerfile)

	rockerContentFirst := []byte(`FROM alpine
								  EXPORT /etc/issue

								  FROM alpine
								  RUN echo -n "first_few" > /exported_file
								  EXPORT /exported_file

						 	      FROM alpine
						 	      MOUNT ` + dir + `:/datadir
						 	      IMPORT /exported_file /datadir/imported_file`)

	rockerContentSecond := []byte(`FROM alpine
								  EXPORT /etc/issue

								  FROM alpine
								  RUN echo -n "second_few" > /exported_file
								  EXPORT /exported_file`)

	err = ioutil.WriteFile(rockerfile, rockerContentFirst, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile, "--reload-cache")
	assert.Nil(t, err)

	err = ioutil.WriteFile(rockerfile, rockerContentSecond, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile, "--reload-cache")
	assert.Nil(t, err)

	err = ioutil.WriteFile(rockerfile, rockerContentFirst, 0644)
	assert.Nil(t, err)
	err = runRockerBuildWithFile(rockerfile)
	assert.Nil(t, err)
	content, err := ioutil.ReadFile(dir + "/imported_file")
	assert.Nil(t, err, "Can't read file")
	assert.Equal(t, "first_few", string(content))
}

func TestDoubleExport(t *testing.T) {
	rockerContent := `FROM alpine
					  EXPORT /etc/issue issue
					  EXPORT /etc/hostname hostname

					  FROM alpine
					  IMPORT issue
					  IMPORT hostname`

	err := runRockerBuildWithOptions(rockerContent, "--reload-cache")
	assert.Nil(t, err)

	err = runRockerBuildWithOptions(rockerContent)
	assert.Nil(t, err)
}
