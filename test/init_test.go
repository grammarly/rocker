package tests

import (
	"flag"
	"os"
	"testing"
)

var (
	verbosity_level = flag.Int("verbosity", 0, "Level of verbosity. 0 - nothing, 1 - cmds, 2 - output of cmd's")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
