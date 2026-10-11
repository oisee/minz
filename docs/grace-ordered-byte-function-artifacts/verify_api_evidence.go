package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/pipeline"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	_, f, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(f), "api")
	rows := []map[string]any{}
	for _, size := range []bool{false, true} {
		mode := fmt.Sprint(size)
		b, e := os.ReadFile(filepath.Join(root, "size-"+mode+"-evidence.json.gz"))
		must(e)
		z, err := gzip.NewReader(bytes.NewReader(b))
		must(err)
		b, err = io.ReadAll(z)
		must(err)
		must(z.Close())
		var evidence pipeline.OrderedByteFunctionEvidence
		must(json.Unmarshal(b, &evidence))
		saved := evidence.SHA256
		evidence.SHA256 = ""
		typed, e := json.Marshal(evidence)
		must(e)
		if fmt.Sprintf("%x", sha256.Sum256(typed)) != saved {
			panic("typed evidence SHA")
		}
		for i, c := range evidence.Candidates {
			hash := fmt.Sprintf("%x", sha256.Sum256(c.Binary))
			if hash != c.BinarySHA256 || hash != c.Run.BinarySHA256 {
				panic("candidate binary SHA")
			}
			rows = append(rows, map[string]any{"mode": mode, "candidate": i, "evidence_sha256": saved, "binary_sha256": hash, "t_states": c.TotalTStates, "bytes": c.ModuleBytes})
		}
	}
	b, e := json.MarshalIndent(rows, "", "  ")
	must(e)
	fmt.Println(string(b))
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
