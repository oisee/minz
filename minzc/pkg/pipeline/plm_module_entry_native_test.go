package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/plm"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// PL/M module statements are executable; string constants contain their exact
// character bytes, not an inferred C terminator. Original HELLO is run under
// an explicit test-owned trailing-zero memory convention (not a CP/M claim).
// Retain this historical source checkpoint; current Hello has a whole-CP/M gate.
func TestPLMModuleEntryNative(t *testing.T) {
	save := func(name string, v any) {
		if dir := os.Getenv("MINZ_PLM_MODULE_ARTIFACTS"); dir != "" {
			var b []byte
			switch x := v.(type) {
			case string:
				b = []byte(x)
			case []byte:
				b = x
			default:
				b, _ = json.MarshalIndent(v, "", "  ")
			}
			p := filepath.Join(dir, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	freezeBytes, err := os.ReadFile("../../../reports/plm-top-level-entry/freeze.json")
	if err != nil {
		t.Fatal(err)
	}
	var freeze map[string]any
	if err = json.Unmarshal(freezeBytes, &freeze); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []string{"hello", "process"} {
		path := "../../../reports/plm-top-level-entry/process.plm"
		if fixture == "hello" {
			path = "../../../reports/plm-top-level-entry/source.plm"
		}
		src, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		key := "process_sha256"
		if fixture == "hello" {
			key = "source_sha256"
		}
		if fmt.Sprintf("%x", sha256.Sum256(src)) != freeze[key] {
			t.Fatal("source freeze changed", fixture)
		}
		if fixture == "hello" && fmt.Sprintf("%x", sha256.Sum256(src)) != "fdb0a98768bdd5ad347c8220ed409e4f964230541712c8af9b0b4561d259b84c" {
			t.Fatal("original source changed")
		}
		parse := func() *hir.Module {
			h, e := plm.Compile(string(src))
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		h := parse()
		texts := []string{"aB'c!", "zY"}
		if fixture == "hello" {
			texts = []string{"Hello from PL/M-80!"}
		}
		var names []string
		for _, g := range h.Globals {
			if len(g.Init) > 0 {
				i := len(names)
				if i >= len(texts) || string(g.Init) != texts[i] || mir2.ByteWidth(g.Ty) != len(texts[i]) {
					t.Fatal("exact literal", g)
				}
				names = append(names, g.Name)
			}
		}
		if len(names) != len(texts) {
			t.Fatal("literal count")
		}
		var expectedTrace byte
		for _, s := range texts {
			for _, b := range []byte(s) {
				expectedTrace = expectedTrace*3 + b
			}
		}
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			raw := hir.LowerModule(parse())
			steps, e := CompileHIRSteps(parse(), Options{AssertMode: "none", UseGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			callerAssembly := calledMainTestAssembly(t, steps)
			asm, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + callerAssembly)
			if e != nil || len(asm.Errors) > 0 {
				t.Fatal(e, asm.Errors)
			}
			prefix := fixture + "/" + mode
			save(prefix+"/raw.mir", raw.Dump())
			save(prefix+"/final.mir", steps.MIR2Module.Dump())
			save(prefix+"/target.a80", callerAssembly)
			save(prefix+"/target.bin", asm.Binary)
			type event struct {
				Op     string
				Object string
				Offset int64
				Value  byte
			}
			model := func() []event {
				var es []event
				var v byte
				if fixture == "process" {
					es = append(es, event{"write", "TRACE", 0, 0})
				}
				for i, s := range texts {
					for j, b := range []byte(s) {
						es = append(es, event{"read", names[i], int64(j), b})
						if fixture == "process" {
							v = v*3 + b
							es = append(es, event{"write", "TRACE", 0, v})
						}
					}
					if fixture == "hello" {
						es = append(es, event{"read", names[i], int64(len(s)), 0})
					}
				}
				return es
			}
			// Normalize relocations by exact object identity+offset; private stack/spill
			// traffic is excluded from object events but checked by whole-RAM guards.
			normalize := func(a mir2.VMDataAccess, addresses map[string]int64) (event, bool) {
				for i, n := range names {
					p := addresses[n]
					length := len(texts[i])
					if fixture == "hello" {
						length++
					}
					if a.Addr >= p && a.Addr < p+int64(length) {
						op := "read"
						if a.Op == mir2.OpStore {
							op = "write"
						}
						return event{op, n, a.Addr - p, a.Value}, true
					}
				}
				if fixture == "process" && a.Addr == addresses["TRACE"] {
					op := "read"
					if a.Op == mir2.OpStore {
						op = "write"
					}
					return event{op, "TRACE", 0, a.Value}, true
				}
				return event{}, false
			}
			// Trace loads used to compute the recurrence are genuine reads too.
			expected := model()
			if fixture == "process" {
				var expanded []event
				for _, v := range expected {
					if v.Op == "write" && v.Object == "TRACE" && len(expanded) > 0 {
						previous := byte(0)
						for j := len(expanded) - 1; j >= 0; j-- {
							if expanded[j].Op == "write" && expanded[j].Object == "TRACE" {
								previous = expanded[j].Value
								break
							}
						}
						expanded = append(expanded, event{"read", "TRACE", 0, previous})
					}
					expanded = append(expanded, v)
				}
				expected = expanded
			}
			for label, m := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
				vm := mir2.NewVM(m)
				vm.EnsureHeap(65536)
				addresses := map[string]int64{}
				for _, n := range names {
					p, e := vm.ResolveSymbol(n)
					if e != nil {
						t.Fatal(e)
					}
					addresses[n] = p
				}
				if fixture == "process" {
					p, e := vm.ResolveSymbol("TRACE")
					if e != nil {
						t.Fatal(e)
					}
					addresses["TRACE"] = p
				}
				if fixture == "hello" {
					vm.WriteHeap(addresses[names[0]]+int64(len(texts[0])), 0)
				}
				before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				var es []event
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
					if v, ok := normalize(a, addresses); ok {
						es = append(es, v)
					}
				}
				if _, e := vm.Call("main", nil); e != nil {
					t.Fatal(fixture, mode, label, e)
				}
				if fixture == "process" {
					before[addresses["TRACE"]] = expectedTrace
				}
				if !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
					t.Fatal("VM whole RAM", fixture, mode, label)
				}
				if fmt.Sprint(es) != fmt.Sprint(expected) {
					t.Fatal("VM effects", fixture, mode, label, es, expected)
				}
				save(prefix+"/"+label+"-events.json", es)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			mem := z.Memory()
			copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
			z.LoadMemory(0x4000, asm.Binary)
			addresses := map[string]int64{}
			for _, n := range names {
				addresses[n] = int64(asm.Symbols[n])
			}
			if fixture == "process" {
				addresses["TRACE"] = int64(asm.Symbols["TRACE"])
			}
			if fixture == "hello" {
				p := addresses[names[0]] + int64(len(texts[0]))
				if p != 0x4000+int64(len(asm.Binary)) {
					t.Fatal("trailing-zero convention must not overwrite another object", p, len(asm.Binary))
				}
				mem[p] = 0
			}
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			before := append([]byte(nil), mem...)
			z.SetRegisters(emulator.Registers{PC: uint16(asm.Symbols["main"]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777})
			minSP := uint16(0xeff0)
			var es []event
			done := false
			for i := 0; i < 100000; i++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					done = true
					break
				}
				for _, a := range lizpTokenizerNativeAccess(mem, z.GetPC(), z.GetRegisters()) {
					if v, ok := normalize(a, addresses); ok {
						es = append(es, v)
					}
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
				if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("native SP")
				}
			}
			if !done {
				t.Fatal("native return", fixture, mode)
			}
			if fixture == "process" {
				before[addresses["TRACE"]] = expectedTrace
			}
			copy(before[minSP:0xeff0], mem[minSP:0xeff0])
			for _, l := range asm.Listing {
				if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
					for j := range l.Bytes {
						before[l.Address+j] = mem[l.Address+j]
					}
				}
			}
			if !bytes.Equal(before, mem) {
				for i, b := range mem {
					if before[i] != b {
						t.Fatal("native whole RAM", fixture, mode, i)
					}
				}
			}
			if len(z.GetOutput()) != 0 {
				t.Fatal("empty PRINTCHAR is not an intrinsic")
			}
			if fmt.Sprint(es) != fmt.Sprint(expected) {
				t.Fatal("native effects", fixture, mode, es, expected)
			}
			save(prefix+"/native-events.json", es)
		}
	}
}
