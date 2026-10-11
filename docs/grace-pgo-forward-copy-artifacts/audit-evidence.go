// Run from minzc: go run ../docs/grace-pgo-forward-copy-artifacts/audit-evidence.go ../docs/grace-pgo-forward-copy-artifacts
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/pipeline"
	"io"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 2 {
		panic("artifact directory required")
	}
	for _, label := range []string{"selector-speed", "selector-size", "cli-speed", "cli-size"} {
		size := label == "selector-size" || label == "cli-size"
		name := filepath.Join(os.Args[1], "selector", fmt.Sprintf("size-%v-evidence.json", size))
		if label == "cli-speed" || label == "cli-size" {
			mode := "speed"
			if size {
				mode = "size"
			}
			name = filepath.Join(os.Args[1], "cli", mode, "joint-evidence.json")
		}
		file, err := os.Open(name)
		if err != nil {
			file, err = os.Open(name + ".gz")
		}
		if err != nil {
			panic(err)
		}
		var reader io.Reader = file
		if filepath.Ext(file.Name()) == ".gz" {
			gz, e := gzip.NewReader(file)
			if e != nil {
				panic(e)
			}
			defer gz.Close()
			reader = gz
		}
		data, err := io.ReadAll(reader)
		file.Close()
		if err != nil {
			panic(err)
		}
		var evidence pipeline.ForwardCopySiteEvidence
		if err := json.Unmarshal(data, &evidence); err != nil {
			panic(err)
		}
		expected := evidence.SHA256
		evidence.SHA256 = ""
		encoded, err := json.Marshal(&evidence)
		if err != nil {
			panic(err)
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(encoded))
		if actual != expected {
			panic(fmt.Sprintf("size=%v typed evidence SHA mismatch %s != %s", size, actual, expected))
		}
		fmt.Printf("PASS %s typed self-excluding evidence SHA256=%s\n", label, actual)
	}
}
