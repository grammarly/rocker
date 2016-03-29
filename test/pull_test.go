package tests

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPull(t *testing.T) {
	errChan := make(chan error)
	go func() {
		errChan <- runRockerPull("alpine")
	}()

	select {
	case err := <-errChan:
		assert.Nil(t, err)
	case <-time.After(time.Second * 20):
		t.Fatal("rocker pull timeouted")
	}
}
