// Run from minzc: go run ../docs/grace-pgo-tokenizer-artifacts/verify_evidence.go
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
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Dir(file)
	rows := []map[string]any{}
	for _, group := range []string{"measured", "cli"} {
		for _, size := range []bool{false, true} {
			name := filepath.Join(root, "measured", fmt.Sprintf("copy-size-%t-evidence.json.gz", size))
			if group == "cli" {
				label := "speed"
				if size {
					label = "size"
				}
				name = filepath.Join(root, "cli", label, "joint-evidence.json.gz")
			}
			f, err := os.Open(name)
			if err != nil {
				panic(err)
			}
			z, err := gzip.NewReader(f)
			if err != nil {
				panic(err)
			}
			var e pipeline.ForwardCopySiteEvidence
			if err := json.NewDecoder(z).Decode(&e); err != nil {
				panic(err)
			}
			z.Close()
			f.Close()
			saved := e.SHA256
			e.SHA256 = ""
			b, err := json.Marshal(e)
			if err != nil {
				panic(err)
			}
			actual := fmt.Sprintf("%x", sha256.Sum256(b))
			if saved != actual {
				panic("typed evidence digest mismatch")
			}
			for i, c := range e.Candidates {
				hash := fmt.Sprintf("%x", sha256.Sum256(c.Binary))
				if hash != c.BinarySHA256 || hash != c.Run.BinarySHA256 {
					panic("binary mismatch")
				}
				rows = append(rows, map[string]any{"group": group, "size": size, "candidate": i, "evidence_sha256": saved, "binary_sha256": hash, "t_states": c.TotalTStates, "bytes": c.ModuleBytes, "selected": e.Selected == i})
			}
		}
	}
	b, _ := json.MarshalIndent(rows, "", "  ")
	fmt.Println(string(b))
}
