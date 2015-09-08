// This code is borrowed from Docker
// Licensed under the Apache License, Version 2.0; Copyright 2013-2015 Docker, Inc. See LICENSE.APACHE
// NOTICE: no changes has been made to these functions code

package build

import (
	"fmt"
	"os"
	gosignal "os/signal"
	"runtime"
	"time"

	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
)

func (builder *Builder) monitorTtySize(id string) error {
	builder.resizeTty(id)

	if runtime.GOOS == "windows" {
		go func() {
			prevH, prevW := builder.getTtySize()
			for {
				time.Sleep(time.Millisecond * 250)
				h, w := builder.getTtySize()

				if prevW != w || prevH != h {
					builder.resizeTty(id)
				}
				prevH = h
				prevW = w
			}
		}()
	} else {
		sigchan := make(chan os.Signal, 1)
		gosignal.Notify(sigchan, signal.SIGWINCH)
		go func() {
			for range sigchan {
				builder.resizeTty(id)
			}
		}()
	}
	return nil
}

func (builder *Builder) resizeTty(id string) {
	height, width := builder.getTtySize()
	if height == 0 && width == 0 {
		return
	}

	if err := builder.Docker.ResizeContainerTTY(id, height, width); err != nil {
		fmt.Fprintf(builder.OutStream, "Failed to resize container TTY %s, error: %s\n", id, err)
	}
}

func (builder *Builder) getTtySize() (int, int) {
	if !builder.isTerminalOut {
		return 0, 0
	}
	ws, err := term.GetWinsize(builder.fdOut)
	if err != nil {
		fmt.Fprintf(builder.OutStream, "Error getting TTY size: %s\n", err)
		if ws == nil {
			return 0, 0
		}
	}
	return int(ws.Height), int(ws.Width)
}
