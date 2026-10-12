package pipeline

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ISO C90 6.3.13-15: logical short circuit and selected conditional arm.
func TestC89ExpressionEffectsNative(t *testing.T) {
	src, e := os.ReadFile("../../../reports/c89-expression-effects/source.c")
	if e != nil {
		t.Fatal(e)
	}
	freezeBytes, e := os.ReadFile("../../../reports/c89-expression-effects/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze map[string]any
	if e = json.Unmarshal(freezeBytes, &freeze); e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(src)) != freeze["source_sha256"] {
		t.Fatal("source freeze")
	}
	save := func(name string, v any) {
		if dir := os.Getenv("MINZ_C89_EXPRESSION_ARTIFACTS"); dir != "" {
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
			os.MkdirAll(filepath.Dir(p), 0755)
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	parse := func() *hir.Module {
		h, e := c89.CompileWithOpts(string(src), "expression-effects.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	model := func(f string, a, b int) (uint16, []byte) {
		n := byte(b)
		x := byte(0)
		switch f {
		case "and_post":
			if a != 0 {
				x = 0
				if n != 0 {
					x = 1
				}
				n++
			}
			return uint16(n)*256 + uint16(x), nil
		case "or_post":
			x = 1
			if a == 0 {
				x = 0
				if n != 0 {
					x = 1
				}
				n++
			}
			return uint16(n)*256 + uint16(x), nil
		case "pick_post":
			if a != 0 {
				x = n
				n++
			} else {
				n++
				x = n
			}
			return uint16(n)*256 + uint16(x), nil
		case "effect_cond":
			if a != 0 {
				x = n
				n++
			} else {
				n++
				x = n
			}
			return uint16(byte(a+1))*256 + uint16(x), nil
		case "nested":
			truth := false
			if a != 0 {
				truth = n != 0
				n++
			}
			if truth {
				n++
				x = n
			} else {
				x = n
				n++
			}
			return uint16(n)*256 + uint16(x), nil
		case "trace_and":
			writes := []byte{0, byte(a)}
			x = 0
			if a != 0 {
				x = 1
				writes = append(writes, byte(a*3+7))
			}
			return uint16(x), writes
		case "trace_or":
			writes := []byte{0, byte(a)}
			x = 1
			if a == 0 {
				writes = append(writes, 7)
			}
			return uint16(x), writes
		case "trace_pick":
			v := byte(9)
			if a != 0 {
				v = 7
			}
			return uint16(v), []byte{0, byte(a), byte(a*3 + int(v))}
		case "loop_condition":
			sum := 0
			for n := 0; n < a; {
				old := n
				n++
				if old >= 3 {
					break
				}
				sum += n
			}
			return uint16(byte(sum)), nil
		}
		panic(f)
	}
	gccFile, e := os.Open("../../../reports/c89-expression-effects/gcc-results.txt.gz")
	if e != nil {
		t.Fatal(e)
	}
	gz, e := gzip.NewReader(gccFile)
	if e != nil {
		t.Fatal(e)
	}
	gccBytes, e := io.ReadAll(gz)
	if e != nil {
		t.Fatal(e)
	}
	gz.Close()
	gccFile.Close()
	for _, line := range strings.Split(strings.TrimSpace(string(gccBytes)), "\n") {
		var f string
		var a, b, v int
		if _, e := fmt.Sscanf(line, "%s %d %d %d", &f, &a, &b, &v); e != nil {
			t.Fatal(e)
		}
		if strings.HasPrefix(f, "trace_") {
			want, w := model(f, a, 0)
			if uint16(b) != want || byte(v) != w[len(w)-1] {
				t.Fatal("GCCmodel", line, want, w)
			}
		} else if f == "loop_condition" {
			want, _ := model(f, a, 0)
			if uint16(b) != want {
				t.Fatal("GCCmodel", line)
			}
		} else {
			want, _ := model(f, a, b)
			if uint16(v) != want {
				t.Fatal("GCCmodel", line, want)
			}
		}
	}
	funcs := []string{"and_post", "or_post", "pick_post", "effect_cond", "nested", "trace_and", "trace_or", "trace_pick", "loop_condition"}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		raw := hir.LowerModule(parse())
		s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", UseHIRGrace: mode != "ordinary", UseGrace: mode != "ordinary", OptSize: mode == "grace-size", ContractOpt: true})
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		save(mode+"/raw.mir", raw.Dump())
		save(mode+"/final.mir", s.MIR2Module.Dump())
		save(mode+"/target.a80", s.Assembly)
		save(mode+"/target.bin", a.Binary)
		z := emulator.NewRemogattoZ80WithScreen()
		z.SetROMEnd(0)
		mem := z.Memory()
		base := bytes.Repeat([]byte{0xa5}, 65536)
		copy(base[0x4000:], a.Binary)
		p := a.Symbols["trace"]
		vmRaw, vmFinal := mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)
		vmRaw.EnsureHeap(65536)
		vmFinal.EnsureHeap(65536)
		pr, _ := vmRaw.ResolveSymbol("trace")
		pf, _ := vmFinal.ResolveSymbol("trace")
		copies := map[*mir2.VM][]byte{vmRaw: append([]byte(nil), vmRaw.ReadHeap(0, 65536)...), vmFinal: append([]byte(nil), vmFinal.ReadHeap(0, 65536)...)}
		calls := 0
		for _, fn := range funcs {
			binary := !strings.HasPrefix(fn, "trace_") && fn != "loop_condition"
			for x := 0; x < 256; x++ {
				maxB := 1
				if binary {
					maxB = 256
				}
				for y := 0; y < maxB; y++ {
					calls++
					want, writes := model(fn, x, y)
					args := []mir2.Value{{I: int64(x)}}
					if binary {
						args = append(args, mir2.Value{I: int64(y)})
					}
					for _, vm := range []*mir2.VM{vmRaw, vmFinal} {
						addr := pr
						if vm == vmFinal {
							addr = pf
						}
						vm.WriteHeap(addr, 99)
						var gotWrites []byte
						vm.ObserveDataAccess = func(v mir2.VMDataAccess) {
							if v.Addr == addr && v.Op == mir2.OpStore {
								gotWrites = append(gotWrites, v.Value)
							}
						}
						v, e := vm.Call(fn, args)
						if e != nil || len(v) != 1 || uint16(v[0].I) != want {
							t.Fatal("VM", mode, fn, x, y, v, want, e)
						}
						if !bytes.Equal(gotWrites, writes) {
							t.Fatal("VMwrites", fn, x, y, gotWrites, writes)
						}
						expected := copies[vm]
						expected[addr] = 99
						if len(writes) > 0 {
							expected[addr] = writes[len(writes)-1]
						}
						if !bytes.Equal(expected, vm.ReadHeap(0, 65536)) {
							t.Fatal("VMRAM", fn, x, y)
						}
					}
					copy(mem, base)
					mem[p] = 99
					mem[0xeff0], mem[0xeff1] = 0, 0x70
					before := append([]byte(nil), mem...)
					r := emulator.Registers{PC: uint16(a.Symbols[fn]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777}
					f := s.MIR2Module.FuncByName(fn)
					for j, v := range args {
						loc := assertParamLocation(f.Contract.Params[j], s.Allocation)
						switch loc.Name {
						case "A":
							r.A = byte(v.I)
						case "B":
							r.BC = r.BC&255 | uint16(byte(v.I))<<8
						case "C":
							r.BC = r.BC&0xff00 | uint16(byte(v.I))
						case "D":
							r.DE = r.DE&255 | uint16(byte(v.I))<<8
						case "E":
							r.DE = r.DE&0xff00 | uint16(byte(v.I))
						default:
							t.Fatal("ABI", loc)
						}
					}
					z.SetRegisters(r)
					minSP := uint16(0xeff0)
					done := false
					var gotWrites []byte
					for i := 0; i < 10000; i++ {
						if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
							done = true
							break
						}
						for _, v := range lizpTokenizerNativeAccess(mem, z.GetPC(), z.GetRegisters()) {
							if v.Addr == int64(p) && v.Op == mir2.OpStore {
								gotWrites = append(gotWrites, v.Value)
							}
						}
						z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
						if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
							t.Fatal("SP")
						}
					}
					value := uint16(z.GetRegisters().A)
					if binary {
						value = z.GetRegisters().HL
					}
					if !done || value != want {
						t.Fatal("native", mode, fn, x, y, value, want, z.GetRegisters())
					}
					if !bytes.Equal(gotWrites, writes) {
						t.Fatal("native writes", fn, x, y, gotWrites, writes)
					}
					if len(writes) > 0 {
						before[p] = writes[len(writes)-1]
					}
					copy(before[minSP:0xeff0], mem[minSP:0xeff0])
					for _, l := range a.Listing {
						if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
							for j := range l.Bytes {
								before[l.Address+j] = mem[l.Address+j]
							}
						}
					}
					if !bytes.Equal(before, mem) {
						t.Fatal("native RAM", fn, x, y)
					}
				}
			}
		}
		save(mode+"/summary.json", map[string]any{"calls": calls, "native_raw_final": "PASS", "domain": freeze["domain"]})
		t.Log(mode, calls, "raw/final/native whole RAM SP/code ordered trace PASS")
	}
}
