package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Audit of unchanged FAT12 helpers: every memory byte, full RAM and native
// return stack are independently checked. No memory-fusion rule is enabled.
func TestByteWordFusionAudit(t *testing.T) {
	const path = "../../../stdlib/fs/fat12.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	save := func(name string, value any) {
		dir := os.Getenv("MINZ_BYTE_WORD_AUDIT")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		var b []byte
		switch v := value.(type) {
		case string:
			b = []byte(v)
		case []byte:
			b = v
		default:
			var e error
			b, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if err := os.WriteFile(p, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	save("source.nanz", source)
	for _, mode := range []string{"default", "grace-size", "grace-speed", "direct16-study"} {
		options := Options{ContractOpt: true, UseGrace: mode == "grace-size" || mode == "grace-speed", OptSize: mode == "grace-size", AssertMode: "none"}
		h, err := nanz.Parse(string(source), path)
		if err != nil {
			t.Fatal(err)
		}
		s, err := CompileHIRSteps(h, options)
		if err != nil {
			t.Fatal(err)
		}
		if mode == "direct16-study" {
			// Counterfactual only: not a production rule. Dropping original
			// byte Inst identities deliberately fails the callback gate below.
			for _, name := range []string{"ld_word", "st_word"} {
				f := s.MIR2Module.FuncByName(name)
				if name == "ld_word" {
					ret := f.Entry().Term.(*mir2.TermRet).Vals[0]
					f.Entry().Insts = []*mir2.Inst{{Op: mir2.OpLoad, Dst: ret, Src: [2]mir2.Reg{f.Contract.Params[0].Reg}, Ty: mir2.TyU16}}
				} else {
					f.Entry().Insts = []*mir2.Inst{{Op: mir2.OpStore, Src: [2]mir2.Reg{f.Contract.Params[0].Reg, f.Contract.Params[1].Reg}, Ty: mir2.TyU16}}
				}
			}
			var err error
			s.Assembly, err = mir2.Z80Codegen(s.MIR2Module, s.Allocation)
			if err != nil {
				t.Fatal(err)
			}
		}
		save(mode+"/final.mir", s.MIR2Module.Dump())
		save(mode+"/target.a80", s.Assembly)
		save(mode+"/allocation.json", s.Allocation)
		size, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
		if err != nil || len(size.Errors) > 0 {
			t.Fatal(err, size.Errors)
		}
		save(mode+"/target.bin", size.Binary)
		for _, name := range []string{"ld_word", "st_word"} {
			for _, base := range []int64{0x4000, 65535} {
				for _, value := range []int64{0, 1, 0x1234, 0x80ff, 65535} {
					vm := mir2.NewVM(s.MIR2Module)
					vm.EnsureHeap(65536)
					vm.WriteHeap(base, byte(value))
					vm.WriteHeap((base+1)&65535, byte(value>>8))
					var events []mir2.VMDataAccess
					vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
					args := []int64{base}
					vargs := []mir2.Value{{I: base}}
					if name == "st_word" {
						args = append(args, value)
						vargs = append(vargs, mir2.Value{I: value})
					}
					result, e := vm.Call(name, vargs)
					if e != nil {
						t.Fatal(e)
					}
					if name == "ld_word" && (len(result) != 1 || result[0].I != value) {
						t.Fatal("independentVMword", result)
					}
					if len(events) != 2 {
						t.Fatal("byteaccesscount", events)
					}
					for j, a := range events {
						wantOp := mir2.OpLoad
						if name == "st_word" {
							wantOp = mir2.OpStore
						}
						if a.Op != wantOp || a.Addr != (base+int64(j))&65535 || a.Value != byte(value>>(8*j)) {
							t.Fatal("independent ordered byte oracle", events)
						}
					}
					f := s.MIR2Module.FuncByName(name)
					a := hir.Assert{FuncName: name, Args: args, Expected: value}
					boot := buildAssertBootstrap(assertLoadAddr, a, f, s.Allocation)
					bin, e := z80asm.NewAssembler().AssembleString(boot + "\n" + s.Assembly)
					if e != nil || len(bin.Errors) > 0 {
						t.Fatal(e, bin.Errors)
					}
					platform := emulator.NewRemogattoZ80WithScreen()
					platform.SetROMEnd(0)
					z := platform.RemogattoZ80
					z.SetSMCTracker(nil)
					if e := z.LoadMemory(assertLoadAddr, bin.Binary); e != nil {
						t.Fatal(e)
					}
					// Store starts with distinct bytes to make both writes observable.
					for j := 0; j < 2; j++ {
						v := byte(value >> (8 * j))
						if name == "st_word" {
							v = ^v
						}
						z.SetMemory(uint16(base+int64(j)), v)
					}
					z.SetPC(assertLoadAddr)
					for i := 0; z.GetPC() != uint16(bin.Symbols[name]); i++ {
						if i > 1000 {
							t.Fatal("entry")
						}
						z.Step()
					}
					prof := emulator.NewProfiler()
					z.SetProfiler(prof)
					before := append([]byte(nil), z.Memory()...)
					sp := z.GetSP()
					minSP := sp
					ret := uint16(z.GetMemory(sp)) | uint16(z.GetMemory(sp+1))<<8
					start := z.GetCycles()
					for i := 0; ; i++ {
						if i > 10000 {
							t.Fatal("return")
						}
						z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
						if z.GetPC() == ret && z.GetSP() == sp+2 {
							break
						}
					}
					for j := 0; j < 2; j++ {
						addr := uint16(base + int64(j))
						reads, writes := uint32(1), uint32(0)
						if name == "st_word" {
							reads, writes = 0, 1
						}
						if prof.ReadCount[addr] != reads || prof.WriteCount[addr] != writes {
							t.Fatal("exact native byte traffic", addr, prof.ReadCount[addr], prof.WriteCount[addr])
						}
					}
					if name == "ld_word" {
						if e := checkAssertZ80Result(z, a, f, map[string]*hir.Func{name: h.FuncByName(name)}); e != nil {
							t.Fatal(e)
						}
					}
					for p, b := range before {
						addr := uint16(p)
						if addr >= minSP && addr < sp {
							continue
						}
						if name == "st_word" && (addr == uint16(base) || addr == uint16(base+1)) {
							continue
						}
						if z.GetMemory(addr) != b {
							t.Fatalf("unexpectedmemorywrite%04x", addr)
						}
					}
					if name == "st_word" && (z.GetMemory(uint16(base)) != byte(value) || z.GetMemory(uint16(base+1)) != byte(value>>8)) {
						t.Fatal("independentnativestore")
					}
					save(fmt.Sprintf("%s/%s-%d-%d.json", mode, name, base, value), map[string]any{"name": name, "base": base, "value": value, "t_states": z.GetCycles() - start, "binary_bytes": len(size.Binary), "byte_accesses": events})
					t.Logf("%s %s base%d value%d %dT %dB VM/nativeorderedbytes/fullRAM/SP PASS", mode, name, base, value, z.GetCycles()-start, len(size.Binary))
				}
			}
		}
	}
}

// A direct width16 OpLoad keeps byte order, but changes the callback's original
// Inst/type identity. Thus byte fusion cannot preserve today's VM contract.
func TestByteWordFusionCallbackRefusal(t *testing.T) {
	const source = `fun read(p:^u8)->u16 {
 let lo:u16=p[0]
 let hi:u16=p[1]
 let shifted:u16=hi<<8
 return shifted|lo
 }`
	h, e := nanz.Parse(source, "callback.nanz")
	if e != nil {
		t.Fatal(e)
	}
	m := hir.LowerModule(h)
	f := m.FuncByName("read")
	var loads []*mir2.Inst
	for _, i := range f.Entry().Insts {
		if i.Op == mir2.OpLoad {
			loads = append(loads, i)
		}
	}
	if len(loads) != 2 {
		t.Fatal("two byte loads required")
	}
	run := func() (bool, []mir2.VMDataAccess) {
		vm := mir2.NewVM(m)
		vm.EnsureHeap(65536)
		vm.WriteHeap(65535, 0x34)
		vm.WriteHeap(0, 0x12)
		var events []mir2.VMDataAccess
		vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
		vm.CheckDataAccess = func(i *mir2.Inst, a int64) error {
			if i.Ty != mir2.TyU8 || i != loads[0] && i != loads[1] {
				return &fusionAuditDenied{}
			}
			return nil
		}
		v, e := vm.Call("read", []mir2.Value{{I: 65535}})
		return e == nil && len(v) == 1 && v[0].I == 0x1234, events
	}
	ok, events := run()
	if !ok || len(events) != 2 {
		t.Fatal("original byte oracle")
	}
	original := f.Entry().Insts
	term := f.Entry().Term
	f.Entry().Insts = []*mir2.Inst{{Op: mir2.OpLoad, Dst: loads[0].Dst, Src: loads[0].Src, Ty: mir2.TyU16}}
	f.Entry().Term = &mir2.TermRet{Vals: []mir2.Reg{loads[0].Dst}}
	ok, fusedEvents := run()
	if ok || len(fusedEvents) != 0 {
		t.Fatal("naive fusion should be rejected before first byte")
	}
	f.Entry().Insts = original
	f.Entry().Term = term
	if !bytes.Equal([]byte{byte(events[0].Value), byte(events[1].Value)}, []byte{0x34, 0x12}) {
		t.Fatal("littleendianbytes")
	}
}

type fusionAuditDenied struct{}

func (*fusionAuditDenied) Error() string { return "original byte load identity required" }
