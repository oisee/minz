// Run from minzc: go run ../docs/grace-pgo-copy-region-artifacts/verify_evidence.go
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	_, f, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(f), "api")
	rows := []map[string]any{}
	for _, size := range []bool{false, true} {
		file, err := os.Open(filepath.Join(root, fmt.Sprintf("copy-region-size-%t-evidence.json.gz", size)))
		must(err)
		z, err := gzip.NewReader(file)
		must(err)
		var e pipeline.MixedCopyRegionSiteEvidence
		must(json.NewDecoder(z).Decode(&e))
		must(z.Close())
		must(file.Close())
		saved := e.SHA256
		e.SHA256 = ""
		b, err := json.Marshal(e)
		must(err)
		if fmt.Sprintf("%x", sha256.Sum256(b)) != saved {
			panic("typed evidence SHA mismatch")
		}
		for i, c := range e.Candidates {
			hash := fmt.Sprintf("%x", sha256.Sum256(c.Binary))
			if hash != c.BinarySHA256 || hash != c.Run.BinarySHA256 || len(c.Binary) != c.ModuleBytes {
				panic("candidate binary binding mismatch")
			}
			rows = append(rows, map[string]any{"size": size, "candidate": i, "evidence_sha256": saved, "binary_sha256": hash, "t_states": c.TotalTStates, "bytes": c.ModuleBytes, "selected": e.Selected == i})
		}
		selected, err := os.ReadFile(filepath.Join(root, fmt.Sprintf("copy-region-size-%t-selected.bin", size)))
		must(err)
		if fmt.Sprintf("%x", sha256.Sum256(selected)) != e.Candidates[e.Selected].BinarySHA256 {
			panic("selected binary mismatch")
		}
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	must(err)
	fmt.Println(string(b))
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
