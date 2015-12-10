// The MIT License (MIT)
// Copyright (c) 2014 Simon Eskildsen
// NOTE: modified to support tokens longer than 64k

package textformatter

import (
	"bufio"
	"io"
	"runtime"

	"github.com/Sirupsen/logrus"
)

// LogWriter makes a pipe writer to write to the logrus logger
func LogWriter(logger *logrus.Logger) *io.PipeWriter {
	reader, writer := io.Pipe()

	go logWriterScanner(logger, reader)
	runtime.SetFinalizer(writer, writerFinalizer)

	return writer
}

func logWriterScanner(logger *logrus.Logger, reader *io.PipeReader) {
	defer reader.Close()

	// 64k max per line
	buf := bufio.NewReaderSize(reader, 1024*64)

	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Errorf("Error while reading from Writer: %s", err)
			return
		}
		logger.Print(string(line))
	}
}

func writerFinalizer(writer *io.PipeWriter) {
	writer.Close()
}
