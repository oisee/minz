package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Turbo Pascal subset Byte is TyU8 (pascal/lower.go); Book v7 §3.1 byte
// representation and chapter6 recursion. Overflow model is modulo256.
func TestOriginalPascalRecursiveFullByteDomain(t *testing.T) {
	const path = "../../../examples/pascal/recursive_test.pas"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile("../../../reports/pascal-recursive-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	type call struct {
		Entry    string `json:"entry"`
		Input    int64  `json:"input"`
		Expected int64  `json:"expected"`
	}
	var freeze struct {
		SourceSHA256 string `json:"source_sha256"`
		Cases        []call `json:"cases"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil || freeze.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(source)) {
		t.Fatal("freeze", e)
	}
	if len(freeze.Cases) != 2048 {
		t.Fatal("frozen complete byte domain", len(freeze.Cases))
	}
	seen := map[string]bool{}
	for _, c := range freeze.Cases {
		key := fmt.Sprintf("%s/%d", c.Entry, c.Input)
		if seen[key] || c.Input < 0 || c.Input > 255 {
			t.Fatal("duplicate/domain", c)
		}
		seen[key] = true
		n := c.Input
		var want int64
		switch c.Entry {
		case "Inc2":
			want = (n + 2) & 255
		case "Dec2":
			want = (n - 2) & 255
		case "Negate":
			want = (-n) & 255
		case "Identity":
			want = n
		case "Zero":
			want = 0
		case "IsPositive":
			if n > 0 {
				want = 1
			}
		case "Fact":
			want = 1
			for j := int64(2); j <= n; j++ {
				want = want * j & 255
			}
		case "SumTo":
			for j := int64(0); j <= n; j++ {
				want = (want + j) & 255
			}
		default:
			t.Fatal("unknown originalentry", c.Entry)
		}
		if want != c.Expected {
			t.Fatal("independent frozen model", c, want)
		}
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_PASCAL_RECURSIVE_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		switch x := v.(type) {
		case string:
			b = []byte(x)
		case []byte:
			b = x
		default:
			b, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	parse := func() *hir.Module {
		h, e := pascal.Compile(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		raw := hir.LowerModule(parse())
		h := parse()
		s, e := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", OptSize: mode == "grace-size"})
		if e != nil {
			t.Fatal(mode, e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		if len(a.Binary) > 0x6000 {
			t.Fatal("image stack overlap")
		}
		save(mode+"/target.a80", s.Assembly)
		save(mode+"/target.bin", a.Binary)
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", s.MIR2Module.Dump())
		save(mode+"/allocation.json", s.Allocation)
		spills := map[int]bool{}
		for _, l := range a.Listing {
			if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
				for j := range l.Bytes {
					spills[l.Address+j] = true
				}
			}
		}
		cases := append([]call(nil), freeze.Cases...)
		for j := range cases {
			cases[j].Entry = strings.ToUpper(cases[j].Entry)
		}
		for _, assert := range h.Asserts {
			if len(assert.Args) != 1 {
				t.Fatal("sourceassertshape", assert)
			}
			cases = append(cases, call{assert.FuncName, assert.Args[0], assert.Expected})
		}
		cases = append(cases, call{Entry: "main"})
		z := emulator.NewRemogattoZ80()
		copy(z.Memory(), bytes.Repeat([]byte{0xa5}, 65536))
		z.LoadMemory(0x8000, a.Binary)
		totalT := 0
		maxDepth := uint16(0)
		for _, c := range cases {
			for name, m := range map[string]*mir2.Module{"raw": raw, "final": s.MIR2Module} {
				vm := mir2.NewVM(m)
				vm.EnsureHeap(65536)
				before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				var args []mir2.Value
				if c.Entry != "main" {
					args = []mir2.Value{{I: c.Input}}
				}
				v, e := vm.Call(c.Entry, args)
				if e != nil {
					t.Fatal(mode, name, c, e)
				}
				if c.Entry != "main" && (len(v) != 1 || v[0].I != c.Expected) {
					t.Fatal(mode, name, c, v)
				}
				if !bytes.Equal(vm.ReadHeap(0, 65536), before) {
					t.Fatal("VMunexpectedmemory", c)
				}
			}
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			before := append([]byte(nil), mem...)
			r := emulator.Registers{PC: uint16(a.Symbols[c.Entry]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999}
			f := s.MIR2Module.FuncByName(c.Entry)
			if f == nil {
				t.Fatal("missingentry", c)
			}
			if c.Entry != "main" {
				loc := assertParamLocation(f.Contract.Params[0], s.Allocation)
				switch loc.Name {
				case "A":
					r.A = byte(c.Input)
				case "B":
					r.BC = r.BC&255 | uint16(c.Input)<<8
				case "C":
					r.BC = r.BC&0xff00 | uint16(c.Input)
				default:
					t.Fatal("parameterhome", loc)
				}
			}
			z.SetRegisters(r)
			start := z.GetCycles()
			minSP := uint16(0xeff0)
			done := false
			for n := 0; n < 1000000; n++ {
				if (c.Entry == "main" && z.IsHalted() && z.GetSP() == 0xeff0) || (c.Entry != "main" && z.GetPC() == 0x7000 && z.GetSP() == 0xeff2) {
					done = true
					break
				}
				if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("stackbounds", c)
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if c.Entry == "main" && z.GetRegisters().A != 0 {
				t.Fatal("main status", z.GetRegisters())
			}
			if !done {
				t.Fatal("native notreturned", mode, c, z.GetRegisters())
			}
			if c.Entry != "main" {
				if e = checkAssertZ80Result(z, hir.Assert{FuncName: c.Entry, Args: []int64{c.Input}, Expected: c.Expected}, f, map[string]*hir.Func{c.Entry: h.FuncByName(c.Entry)}); e != nil {
					t.Fatal(mode, c, e)
				}
			}
			copy(before[minSP:0xeff0], mem[minSP:0xeff0])
			for p := range spills {
				before[p] = mem[p]
			}
			if !bytes.Equal(mem, before) {
				for p := range mem {
					if mem[p] != before[p] {
						t.Fatalf("%s %+v RAM%x got%x want%x", mode, c, p, mem[p], before[p])
					}
				}
			}
			if len(z.GetOutput()) != 0 {
				t.Fatal("unexpectedoutput", c)
			}
			totalT += z.GetCycles() - start
			if d := 0xeff0 - minSP; d > maxDepth {
				maxDepth = d
			}
		}
		save(mode+"/evidence.json", map[string]any{"calls": len(cases), "source_asserts": len(h.Asserts), "max_stack_bytes": maxDepth, "T_states": totalT, "source_sha256": freeze.SourceSHA256, "bin_sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary))})
		t.Log(mode, len(cases), "exactmain/sourceasserts/fullByte8entries raw/final/native RAM/SP/image PASS")
	}
}
