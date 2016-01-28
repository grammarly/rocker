// +build !linux,!darwin,!freebsd

package debugtrap

// SetupDumpStackTrap set up a handler for SIGUSR1 and dumps
// the goroutine stack trace to INFO log
func SetupDumpStackTrap() {
	return
}
