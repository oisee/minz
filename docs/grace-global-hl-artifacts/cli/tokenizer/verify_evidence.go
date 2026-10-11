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
	root := filepath.Dir(f)
	rows := []map[string]any{}
	for _, mode := range []string{"speed", "size"} {
		b, e := read(filepath.Join(root, mode, "joint-evidence.json"))
		must(e)
		var evidence pipeline.MixedCopyRegionSiteEvidence
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

func read(path string) ([]byte, error) {
	b, e := os.ReadFile(path)
	if e == nil {
		return b, nil
	}
	b, e = os.ReadFile(path + ".gz")
	if e != nil {
		return nil, e
	}
	r, e := gzip.NewReader(bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	defer r.Close()
	return io.ReadAll(r)
}
