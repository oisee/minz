// Assemble saved before/after ASM only; no compiler or workload execution.
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
)

func main() {
	root := os.Args[1]
	for mask := 0; mask < 4; mask++ {
		name := fmt.Sprintf("size-false-mask-%d", mask)
		var prior []byte
		for _, tree := range []string{"budget-b9ad1f73", "budget-current"} {
			p := filepath.Join(root, tree, name+".a80")
			src, e := os.ReadFile(p)
			if e != nil {
				panic(e)
			}
			if prior != nil && !bytes.Equal(prior, src) {
				panic("assembly mismatch " + name)
			}
			prior = src
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + string(src))
			if e != nil || a == nil || len(a.Errors) > 0 || len(a.Binary) == 0 {
				panic(fmt.Sprintf("assembly failed %v %+v", e, a))
			}
			if e = os.WriteFile(filepath.Join(root, tree, name+".bin"), a.Binary, 0644); e != nil {
				panic(e)
			}
			fmt.Printf("PASS %s/%s ASM_SHA256=%x binary_SHA256=%x binary_bytes=%d\n", tree, name, sha256.Sum256(src), sha256.Sum256(a.Binary), len(a.Binary))
		}
		old, _ := os.ReadFile(filepath.Join(root, "budget-b9ad1f73", name+".bin"))
		now, _ := os.ReadFile(filepath.Join(root, "budget-current", name+".bin"))
		if !bytes.Equal(old, now) {
			panic("binary mismatch " + name)
		}

	}
}
