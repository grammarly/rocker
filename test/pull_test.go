package tests

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPull(t *testing.T) {
	err_chan := make(chan error)
	go func() {
		err_chan <- runRockerPull("alpine")
	}()

	select {
	case err := <-err_chan:
		assert.Nil(t, err)
	case <-time.After(time.Second * 20):
		t.Fatal("rocker pull timeouted")
	}
}
