// This code is borrowed from Docker
// Licensed under the Apache License, Version 2.0; Copyright 2013-2015 Docker, Inc. See LICENSE.APACHE
// NOTICE: no changes has been made to these functions code

package build2

import (
	"io"
	"os"
	gosignal "os/signal"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
)

func (c *DockerClient) monitorTtySize(id string, out io.Writer) error {
	c.resizeTty(id, out)

	if runtime.GOOS == "windows" {
		go func() {
			prevH, prevW := c.getTtySize(out)
			for {
				time.Sleep(time.Millisecond * 250)
				h, w := c.getTtySize(out)

				if prevW != w || prevH != h {
					c.resizeTty(id, out)
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
				c.resizeTty(id, out)
			}
		}()
	}
	return nil
}

func (c *DockerClient) resizeTty(id string, out io.Writer) {
	height, width := c.getTtySize(out)
	if height == 0 && width == 0 {
		return
	}

	if err := c.client.ResizeContainerTTY(id, height, width); err != nil {
		log.Errorf("Failed to resize container TTY %.12s, error: %s\n", id, err)
	}
}

func (c *DockerClient) getTtySize(out io.Writer) (int, int) {
	var (
		fdOut, isTerminalOut = term.GetFdInfo(out)
	)

	if !isTerminalOut {
		return 0, 0
	}

	ws, err := term.GetWinsize(fdOut)
	if err != nil {
		log.Errorf("Error getting TTY size: %s\n", err)
		if ws == nil {
			return 0, 0
		}
	}

	return int(ws.Height), int(ws.Width)
}
