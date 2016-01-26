// +build !windows

package debugtrap

import (
	"os"
	"os/signal"
	"syscall"

	psignal "github.com/docker/docker/pkg/signal"
)

// SetupDumpStackTrap set up a handler for SIGUSR1 and dumps
// the goroutine stack trace to INFO log
func SetupDumpStackTrap() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	go func() {
		for range c {
			psignal.DumpStacks()
		}
	}()
}
