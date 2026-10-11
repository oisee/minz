package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Nanz Language Book v7 §3.2: i8 comparisons use signed S xor V semantics.
// The unsigned return stores the absolute distance modulo 256; the largest
// distance between two signed bytes is exactly 255.
func TestSignedSubCmpBoundaryVMZ80(t *testing.T) {
	source, err := os.ReadFile("../../../reports/showcase-src/2026-03-10/ex4a_abs_diff_iar.nanz")
	if err != nil {
		t.Fatal(err)
	}
	const funcs = `
 fun less_sub(a:i8,b:i8)->u8 { if a<b {return a-b}
 return 0 }
 fun equal_sub(a:i8,b:i8)->u8 { var c:i8=a-b
 if a==b {return 100}
 return u8(c) }
 fun greater_sub(a:i8,b:i8)->u8 { var c:i8=a-b
 if a>b {return u8(c)}
 return 0 }
 fun unsigned_sub(a:u8,b:u8)->u8 { var c:u8=a-b
 if a>=b {return c}
 return -c }
 `
	values := []int64{-128, -127, -10, -1, 0, 1, 10, 126, 127}
	for _, grace := range []bool{false, true} {
		t.Run(fmt.Sprintf("grace=%v", grace), func(t *testing.T) {
			h, err := nanz.Parse(string(source)+funcs, "signed_sub_cmp")
			if err != nil {
				t.Fatal(err)
			}
			h.Asserts = nil
			raw := hir.LowerModule(h)
			var observations strings.Builder
			for _, a := range values {
				for _, b := range values {
					distance := a - b
					if distance < 0 {
						distance = -distance
					}
					expectations := map[string]int64{"abs_diff": distance, "less_sub": 0, "equal_sub": int64(uint8(a - b)), "greater_sub": 0}
					if a < b {
						expectations["less_sub"] = int64(uint8(a - b))
					}
					if a == b {
						expectations["equal_sub"] = 100
					}
					if a > b {
						expectations["greater_sub"] = int64(uint8(a - b))
					}
					ua, ub := int64(uint8(a)), int64(uint8(b))
					ud := ua - ub
					if ud < 0 {
						ud = -ud
					}
					expectations["unsigned_sub"] = ud
					for _, fn := range []string{"abs_diff", "less_sub", "equal_sub", "greater_sub", "unsigned_sub"} {
						want := expectations[fn]
						args := []mir2.Value{{I: a}, {I: b}}
						if fn == "unsigned_sub" {
							args = []mir2.Value{{I: ua}, {I: ub}}
						}
						got, err := mir2.NewVM(raw).Call(fn, args)
						if err != nil || len(got) != 1 || got[0].I != want {
							t.Fatalf("raw %s(%d,%d)=%v err=%v want=%d", fn, a, b, got, err, want)
						}
						h.Asserts = append(h.Asserts, hir.Assert{FuncName: fn, Args: []int64{args[0].I, args[1].I}, Expected: want, Source: fmt.Sprintf("%s(%d,%d)==%d", fn, a, b, want)})
						fmt.Fprintf(&observations, "%s,%d,%d,%d\n", fn, args[0].I, args[1].I, want)
					}
				}
			}
			s, err := CompileHIRSteps(h, Options{UseGrace: grace})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%d cases PASS raw MIR2 + optimized MIR2 + assembled Z80", len(h.Asserts))
			if dir := os.Getenv("MINZ_SIGNED_SUB_CMP_ARTIFACTS"); dir != "" {
				dir = filepath.Join(dir, fmt.Sprintf("grace-%v", grace))
				if err = os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				asm := z80asm.NewAssembler()
				binary, asmErr := asm.AssembleString(s.Assembly)
				if asmErr != nil {
					t.Fatal(asmErr)
				}
				if len(binary.Errors) != 0 {
					t.Fatal(binary.Errors)
				}
				if err = os.WriteFile(filepath.Join(dir, "target.bin"), binary.Binary, 0644); err != nil {
					t.Fatal(err)
				}
				for name, data := range map[string]string{"source.nanz": string(source) + funcs, "raw.mir2": raw.Dump(), "final.mir2": s.MIR2Module.Dump(), "target.a80": s.Assembly, "oracle.csv": observations.String()} {
					if err = os.WriteFile(filepath.Join(dir, name), []byte(data), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
