package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"github.com/minz/minzc/pkg/z80asm"
	"io"
	"os"
	"path/filepath"
)

func main() {
	root := os.Args[1]
	prior := "/tmp/minz-grace-pgo-resume/docs/grace-ordered-byte-function-artifacts/budget-check"
	nowSrc, e := readSaved(filepath.Join(root, "source.nanz"))
	if e != nil {
		panic(e)
	}
	oldSrc, e := readSaved(filepath.Join(prior, "source.nanz"))
	if e != nil || !bytes.Equal(nowSrc, oldSrc) {
		panic("budget source changed")
	}
	fmt.Printf("PASS source SHA256=%x\n", sha256.Sum256(nowSrc))
	for mask := 0; mask < 4; mask++ {
		name := fmt.Sprintf("size-false-mask-%d", mask)
		now, e := readSaved(filepath.Join(root, "current", name+".a80"))
		if e != nil {
			panic(e)
		}
		old, e := readSaved(filepath.Join(prior, "current", name+".a80"))
		if e != nil || !bytes.Equal(now, old) {
			panic("ASM changed " + name)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + string(now))
		if e != nil || a == nil || len(a.Errors) > 0 {
			panic(fmt.Sprintf("assemble %v %+v", e, a))
		}
		oldbin, e := os.ReadFile(filepath.Join(prior, "current", name+".bin"))
		if e != nil || !bytes.Equal(oldbin, a.Binary) {
			panic("binary changed " + name)
		}
		if e = os.WriteFile(filepath.Join(root, "current", name+".bin"), a.Binary, 0644); e != nil {
			panic(e)
		}
		fmt.Printf("PASS mask%d ASM_SHA256=%x binary_SHA256=%x binary_bytes=%d\n", mask, sha256.Sum256(now), sha256.Sum256(a.Binary), len(a.Binary))
	}
}

func readSaved(path string) ([]byte, error) {
	b, e := os.ReadFile(path)
	if e == nil {
		return b, nil
	}
	f, e := os.Open(path + ".gz")
	if e != nil {
		return nil, e
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		return nil, e
	}
	defer z.Close()
	return io.ReadAll(z)
}
