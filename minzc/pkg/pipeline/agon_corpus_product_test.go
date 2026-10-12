package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Product evidence is deliberately separate from execution: a Z80 CPU cannot
// establish ADL/MOS behavior by running these eZ80 binaries.
func TestAgonCorpusADLProducts(t *testing.T) {
	data, err := os.ReadFile("../../../reports/agon-product-audit/freeze.json")
	if err != nil {
		t.Fatal(err)
	}
	var frozen map[string]string
	if err = json.Unmarshal(data, &frozen); err != nil {
		t.Fatal(err)
	}
	if len(frozen) != 5 {
		t.Fatal("expected five declared Agon sources")
	}
	for path, digest := range frozen {
		for _, mode := range []string{"ordinary", "size", "speed"} {
			t.Run(filepath.Base(path)+"/"+mode, func(t *testing.T) {
				src, e := os.ReadFile("../../../" + path)
				if e != nil {
					t.Fatal(e)
				}
				if fmt.Sprintf("%x", sha256.Sum256(src)) != digest {
					t.Fatal("source freeze changed")
				}
				h, e := nanz.ParseWithOpts(string(src), filepath.Base(path), nanz.ParseOpts{BaseDir: filepath.Dir("../../../" + path), StdlibDir: "../../../stdlib"})
				if e != nil {
					t.Fatal(e)
				}
				s, e := CompileHIRSteps(h, Options{Backend: "ez80", AssertMode: "none", UseHIRGrace: mode != "ordinary", OptSize: mode == "size"})
				if e != nil {
					t.Fatal(e)
				}
				a, e := z80asm.NewAssembler().AssembleString(s.Assembly)
				if e != nil || len(a.Errors) != 0 {
					t.Fatal(e, a)
				}
				entry, found := a.Symbols["main"]
				if !found || entry < a.Origin || entry >= a.Origin+len(a.Binary) || a.Origin != 0x040045 || !strings.Contains(s.Assembly, ".ASSUME ADL=1") {
					t.Fatal("invalid ADL entry/product", entry, a.Origin)
				}
				if dir := os.Getenv("MINZ_AGON_CORPUS_PRODUCTS"); dir != "" {
					dir = filepath.Join(dir, filepath.Base(path), mode)
					if e = os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					evidence, e := json.MarshalIndent(map[string]any{"source_sha256": digest, "bin_sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary)), "origin": a.Origin, "main": entry, "bytes": len(a.Binary), "execution": "not verified: eZ80 ADL/MOS required"}, "", "  ")
					if e != nil {
						t.Fatal(e)
					}
					for name, b := range map[string][]byte{"source.nanz": src, "product.asm": []byte(s.Assembly), "product.bin": a.Binary, "evidence.json": evidence} {
						if e = os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
				t.Logf("%d bytes at %06x; ADL/MOS execution remains unverified", len(a.Binary), a.Origin)
			})
		}
	}
}
