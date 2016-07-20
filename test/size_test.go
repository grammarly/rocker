package tests

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

type rockerBuildFn func(string, io.Writer, ...string) error

func TestSize(t *testing.T) {

	rockerFile := `
FROM busybox:latest
RUN dd if=/dev/zero of=/root/binary1M-1 bs=1 count=1000000
ONBUILD RUN dd if=/dev/zero of=/root/binary1M-2 bs=1 count=1000000
TAG tag1
FROM tag1
RUN dd if=/dev/zero of=/root/binary1M-3 bs=1 count=1000000
RUN echo done`

	rd, wr := io.Pipe()
	jsonRd := json.NewDecoder(rd)

	result := make(chan []int)

	go func() {
		deltas := []int{}
		for {
			m := map[string]interface{}{}
			if err := jsonRd.Decode(&m); err != nil {
				if err == io.EOF {
					break
				}
				debugf("decode error: %s", err)
				result <- []int{}
			}
			debugf("decoded: %#v\n", pretty.Formatter(m))

			size0, ok1 := m["size"]
			delta0, ok2 := m["delta"]

			if ok1 && ok2 {
				size1 := int(size0.(float64))
				delta1 := int(delta0.(float64))
				debugf("size(%v) delta(%v)", size1, delta1)

				deltas = append(deltas, delta1)
			}
		}
		debugf("returning: %v\n", deltas)

		result <- deltas
	}()

	err := runRockerBuildWithOptions(rockerBuildOptions{
		rockerfileContent: rockerFile,
		globalParams:      []string{"--json"},
		stdout:            wr,
	})
	if err != nil {
		t.Fatal(err)
	}
	wr.Close()

	deltas := <-result

	assert.Equal(t, []int{
		0,       // FROM
		1000000, // RUN dd with binary1M-1
		0,       // ONBUILD RUN dd
		0,       // FROM tag1
		1000000, // onbuild-triggered dd
		1000000, // RUN dd with binary1M-3
		0,       // RUN echo
		2000000, // final delta from tag1
	}, deltas, "deltas should be correct")

}
