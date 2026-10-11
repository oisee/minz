package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Book v7 §2.7/§4.1 defines byte-offset named struct fields; §3.1 defines
// unsigned byte width. Expected L1 arithmetic is computed independently here.
func TestOriginalManhattanNativeModel(t *testing.T) {
	const path = "../../../reports/showcase-src/2026-03-13/ex19_assert_forms.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../reports/showcase-manhattan/workload.json")
	if e != nil {
		t.Fatal(e)
	}
	var spec struct {
		SourceSHA256      string `json:"source_sha256"`
		Coords            []int  `json:"coords"`
		AccessorOtherArgs []int  `json:"accessor_other_args"`
	}
	if e = json.Unmarshal(frozen, &spec); e != nil || spec.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(src)) {
		t.Fatal("source freeze", e)
	}
	type call struct {
		Entry    string
		Args     []int64
		Expected int64
	}
	var calls []call
	for _, px := range spec.Coords {
		for _, py := range spec.Coords {
			for _, qx := range spec.Coords {
				for _, qy := range spec.Coords {
					dx := px - qx
					if dx < 0 {
						dx = -dx
					}
					dy := py - qy
					if dy < 0 {
						dy = -dy
					}
					calls = append(calls, call{"manhattan", []int64{int64(px), int64(py), int64(qx), int64(qy)}, int64(byte(dx + dy))})
				}
			}
		}
	}
	for x := 0; x < 256; x++ {
		for _, y := range spec.AccessorOtherArgs {
			calls = append(calls, call{"get_x", []int64{int64(x), int64(y)}, int64(x)}, call{"get_y", []int64{int64(y), int64(x)}, int64(x)})
		}
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_MANHATTAN_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var data []byte
		switch x := v.(type) {
		case string:
			data = []byte(x)
		case []byte:
			data = x
		default:
			data, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(p, data, 0644); e != nil {
			t.Fatal(e)
		}
	}
	save("source.nanz", src)
	save("calls.json", calls)
	for _, mode := range []string{"default", "grace-size", "grace-speed"} {
		parse := func() *hir.Module {
			h, e := nanz.Parse(string(src), path)
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		rawModule := hir.LowerModule(parse())
		h := parse()
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: mode != "default", OptSize: mode == "grace-size"})
		if e != nil {
			t.Fatal(mode, e)
		}
		save(mode+"/target.a80", steps.Assembly)
		save(mode+"/raw.mir", rawModule.Dump())
		save(mode+"/final.mir", steps.MIR2Module.Dump())
		save(mode+"/allocation.json", steps.Allocation)
		// Add only invocation drivers; keep every original function and assertion.
		allowedSpill := map[int]bool{}
		var drivers strings.Builder
		for _, entry := range []string{"manhattan", "get_x", "get_y"} {
			drivers.WriteString(fmt.Sprintf("\noracle_%s:\n CALL %s\n RET\n", entry, entry))
		}

		assembled, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly + drivers.String())
		if e != nil || len(assembled.Errors) > 0 {
			t.Fatal(e, assembled.Errors)
		}
		if len(assembled.Binary) > 0x6000 {
			t.Fatal("driver/image stack overlap")
		}
		save(mode+"/target.bin", assembled.Binary)
		for _, l := range assembled.Listing {
			if strings.HasPrefix(l.Label, "_spill_") {
				for j := range l.Bytes {
					allowedSpill[l.Address+j] = true
				}
			}
		}
		z := emulator.NewRemogattoZ80()
		if e = z.LoadMemory(0x8000, assembled.Binary); e != nil {
			t.Fatal(e)
		}
		for i, c := range calls {
			for label, m := range map[string]*mir2.Module{"raw": rawModule, "final": steps.MIR2Module} {
				vm := mir2.NewVM(m)
				var args []mir2.Value
				for _, a := range c.Args {
					args = append(args, mir2.Value{I: a})
				}
				var events []mir2.VMDataAccess
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
				v, e := vm.Call(c.Entry, args)
				if e != nil || len(v) != 1 || v[0].I != c.Expected {
					t.Fatalf("%s %s call%d %+v got%v err%v", mode, label, i, c, v, e)
				}
				writes, reads := 0, 0
				for _, a := range events {
					if a.Op == mir2.OpStore {
						writes++
					} else {
						reads++
					}
				}
				wantReads, wantWrites := 1, 2
				if c.Entry == "manhattan" {
					wantReads, wantWrites = 4, 4
				}
				if writes != wantWrites || reads != wantReads {
					t.Fatalf("struct observations %s %s writes%d reads%d", label, c.Entry, writes, reads)
				}
				var wantValues []byte
				for _, a := range c.Args {
					wantValues = append(wantValues, byte(a))
				}
				if c.Entry == "manhattan" {
					wantValues = append(wantValues, byte(c.Args[0]), byte(c.Args[2]), byte(c.Args[1]), byte(c.Args[3]))
				} else if c.Entry == "get_x" {
					wantValues = append(wantValues, byte(c.Args[0]))
				} else {
					wantValues = append(wantValues, byte(c.Args[1]))
				}
				for j, event := range events {
					if event.Value != wantValues[j] {
						t.Fatalf("VM struct byte %s %s event%d got%d want%d", label, c.Entry, j, event.Value, wantValues[j])
					}
				}
			}
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			mem[0xe000], mem[0xeff2] = 0x9b, 0xad
			regs := emulator.Registers{PC: uint16(assembled.Symbols["oracle_"+c.Entry]), SP: 0xeff0, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999, A: 0x42}
			f := steps.MIR2Module.FuncByName(c.Entry)
			for j, arg := range c.Args {
				loc := assertParamLocation(f.Contract.Params[j], steps.Allocation)
				v := uint16(arg)
				switch loc.Name {
				case "A":
					regs.A = byte(v)
				case "B":
					regs.BC = regs.BC&255 | v<<8
				case "C":
					regs.BC = regs.BC&0xff00 | v
				case "D":
					regs.DE = regs.DE&255 | v<<8
				case "E":
					regs.DE = regs.DE&0xff00 | v
				case "H":
					regs.HL = regs.HL&255 | v<<8
				case "L":
					regs.HL = regs.HL&0xff00 | v
				case "IXH":
					regs.IX = regs.IX&255 | v<<8
				case "IXL":
					regs.IX = regs.IX&0xff00 | v
				case "IYH":
					regs.IY = regs.IY&255 | v<<8
				case "IYL":
					regs.IY = regs.IY&0xff00 | v
				default:
					if loc.Kind != mir2.LocMem {
						t.Fatal("unhandled entry ABI", loc)
					}
					p := assembled.Symbols[mir2.Z80SpillLabel(f.Name, f.Contract.Params[j].Reg)]
					mem[p] = byte(arg)
				}
			}
			z.SetRegisters(regs)
			prof := emulator.NewProfiler()
			z.SetProfiler(prof)
			before := append([]byte(nil), mem...)
			minSP := uint16(0xeff0)
			start := z.GetCycles()
			done := false
			for j := 0; j < 10000; j++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					done = true
					break
				}
				if z.GetSP() <= 0xe000 || z.GetSP() > 0xeff2 {
					t.Fatal("stack bounds", c)
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if !done {
				t.Fatal("native did not return", c)
			}
			addresses := []uint16{0xefec, 0xefed}
			if c.Entry == "manhattan" {
				addresses = append(addresses, 0xefea, 0xefeb)
			}
			for j, p := range addresses {
				wantReads := uint32(1)
				if c.Entry == "get_x" && j == 1 || c.Entry == "get_y" && j == 0 {
					wantReads = 0
				}
				wantWrites := uint32(1)
				if c.Entry == "manhattan" && j >= 2 {
					wantWrites++
					wantReads++
				} // earlier PUSH/POP AF temporarily occupies future q storage
				if prof.WriteCount[p] != wantWrites || prof.ReadCount[p] != wantReads || mem[p] != byte(c.Args[j]) {
					t.Fatalf("native struct byte %s call%d addr%x writes%d reads%d value%d want%d", mode, i, p, prof.WriteCount[p], prof.ReadCount[p], mem[p], c.Args[j])
				}
			}

			if e = checkAssertZ80Result(z, hir.Assert{FuncName: c.Entry, Args: c.Args, Expected: c.Expected}, f, map[string]*hir.Func{c.Entry: h.FuncByName(c.Entry)}); e != nil {
				t.Fatalf("%s call%d %+v %v", mode, i, c, e)
			}
			for p := 0; p < 65536; p++ {
				if mem[p] == before[p] {
					continue
				}
				if p >= int(minSP) && p < int(0xeff0) {
					continue
				}
				spill := allowedSpill[p]
				if !spill {
					t.Fatalf("unexpected memory %s call%d %x %x->%x minSP%x", mode, i, p, before[p], mem[p], minSP)
				}
			}

			if i < 3 {
				t.Logf("%s %+v %dT native/raw/final model code/SP/RAM PASS", mode, c, z.GetCycles()-start)
			}
		}
		t.Logf("%s %d original calls PASS; %d source assertions retained", mode, len(calls), len(h.Asserts))
	}
}
