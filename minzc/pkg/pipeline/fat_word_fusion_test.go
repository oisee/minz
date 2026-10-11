package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFATSequentialWordFusionNative(t *testing.T) {
	const path = "../../../stdlib/fs/fat12.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		m, e := nanz.Parse(string(src), path)
		if e != nil {
			t.Fatal(e)
		}
		return m
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_FAT_WORD_FUSION_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e = os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		switch x := v.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
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
	buffer := make([]byte, 1026)
	for i := range buffer {
		buffer[i] = byte(i*29 + 17)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{"fat": buffer}}
	for _, offset := range []int{1, 255, 512} {
		for v := int64(0); v < 256; v++ {
			value := v | ((255 - v) << 8)
			w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "st_word", Arguments: []hir.ProfileArgument{{Resource: "fat", Offset: offset}, workloadLiteral(value)}}, hir.ProfileInvocation{Function: "ld_word", Arguments: []hir.ProfileArgument{{Resource: "fat", Offset: offset}}})
		}
	}
	save("source.nanz", src)
	save("workload.json", w)
	source := parse()
	raw, e := hir.RunLoopProfile(source, w)
	if e != nil {
		t.Fatal(e)
	}
	save("raw.json", raw)
	ram := append([]byte(nil), buffer...)
	for i, c := range w.Calls {
		off := c.Arguments[0].Offset
		if c.Function == "st_word" {
			v := uint16(*c.Arguments[1].Value)
			ram[off], ram[off+1] = byte(v), byte(v>>8)
			if len(raw.Results[i]) != 0 {
				t.Fatal("void store")
			}
		} else if len(raw.Results[i]) != 1 || raw.Results[i][0].I != int64(uint16(ram[off])|uint16(ram[off+1])<<8) {
			t.Fatal("word model", i)
		}
		a := raw.Accesses[i].Resources["fat"]
		for j := range buffer {
			var rd, wr uint64
			if j == off || j == off+1 {
				if c.Function == "ld_word" {
					rd = 1
				} else {
					wr = 1
				}
			}
			if a.Reads[j] != rd || a.Writes[j] != wr {
				t.Fatal("independent per-byte counts", i, j)
			}
		}
		if !bytes.Equal(raw.Snapshots[i].Resources["fat"], ram) {
			t.Fatal("full buffer", i)
		}
	}
	var baselineT int64
	var baselineBytes int
	for _, fused := range []bool{false, true} {
		h := parse()
		steps, e := CompileHIRSteps(h, Options{AssertMode: "none", FuseByteWords: fused})
		if e != nil {
			t.Fatal(e)
		}
		save(fmt.Sprintf("fused-%v/final.mir2", fused), steps.MIR2Module.Dump())
		save(fmt.Sprintf("fused-%v/target.a80", fused), steps.Assembly)
		program, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil || len(program.Errors) > 0 {
			t.Fatal(e, program.Errors)
		}
		save(fmt.Sprintf("fused-%v/target.bin", fused), program.Binary)
		count := 0
		for _, name := range []string{"ld_word", "st_word"} {
			for _, b := range steps.MIR2Module.FuncByName(name).Blocks {
				for _, i := range b.Insts {
					if i.SequentialBytes {
						count++
					}
				}
			}
		}
		if fused && count != 2 {
			t.Fatal("helpers did not fuse", count)
		}
		if !fused && count != 0 {
			t.Fatal("default changed")
		}
		vm := mir2.NewVM(steps.MIR2Module)
		base := vm.AllocHeap(buffer).I
		vm.MaxSteps = 1000000
		for i, c := range w.Calls {
			var events []mir2.VMDataAccess
			vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
			args := []mir2.Value{{I: base + int64(c.Arguments[0].Offset)}}
			if c.Function == "st_word" {
				args = append(args, mir2.Value{I: *c.Arguments[1].Value})
			}
			r, e := vm.Call(c.Function, args)
			off := c.Arguments[0].Offset
			op := mir2.OpLoad
			if c.Function == "st_word" {
				op = mir2.OpStore
			}
			snap := raw.Snapshots[i].Resources["fat"]
			want := []mir2.VMDataAccess{{Op: op, Addr: base + int64(off), Value: snap[off]}, {Op: op, Addr: base + int64(off+1), Value: snap[off+1]}}
			if !reflect.DeepEqual(events, want) {
				t.Fatal("ordered byte model", fused, i, events, want)
			}
			if e != nil || len(r) != len(raw.Results[i]) || len(r) > 0 && r[0].I != raw.Results[i][0].I || !bytes.Equal(vm.ReadHeap(base, len(buffer)), raw.Snapshots[i].Resources["fat"]) {
				t.Fatal("final VM", fused, i, r, e)
			}
		}
		native, e := RunZ80ProfileWorkload(steps, source, w, *raw)
		if e != nil {
			t.Fatal("native", fused, e)
		}
		save(fmt.Sprintf("fused-%v/native.json", fused), native)
		if !fused {
			baselineT = native.TotalTStates
			baselineBytes = len(program.Binary)
		} else if native.TotalTStates >= baselineT || len(program.Binary) > baselineBytes {
			t.Fatal("no useful benefit", native.TotalTStates, baselineT, len(program.Binary), baselineBytes)
		}
		t.Logf("fused%v %dcalls %dT %dB SHA%x", fused, len(w.Calls), native.TotalTStates, len(program.Binary), sha256.Sum256(program.Binary))
	}
}

type wordWrapMemory struct {
	memoryHostZ80Memory
	events []mir2.VMDataAccess
}

func (m *wordWrapMemory) ReadByte(a uint16) byte { m.add(3); return m.ReadByteInternal(a) }
func (m *wordWrapMemory) ReadByteInternal(a uint16) byte {
	v := m.data[a]
	if a == 0 || a == 65535 {
		m.events = append(m.events, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(a), Value: v})
	}
	return v
}
func (m *wordWrapMemory) WriteByte(a uint16, v byte) { m.add(3); m.WriteByteInternal(a, v) }
func (m *wordWrapMemory) WriteByteInternal(a uint16, v byte) {
	if a == 0 || a == 65535 {
		m.events = append(m.events, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(a), Value: v})
	}
	m.data[a] = v
}
func TestFATSequentialWordFusionNativeWrap(t *testing.T) {
	const path = "../../../stdlib/fs/fat12.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	for _, fused := range []bool{false, true} {
		h, e := nanz.Parse(string(src), path)
		if e != nil {
			t.Fatal(e)
		}
		s, e := CompileHIRSteps(h, Options{AssertMode: "none", FuseByteWords: fused})
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		mem := &wordWrapMemory{}
		copy(mem.data[0x8000:], a.Binary)
		cpu := z80.NewZ80(mem, memoryHostNoPorts{})
		mem.cpu = cpu
		for v := 0; v < 256; v++ {
			for _, name := range []string{"st_word", "ld_word"} {
				val := uint16(v) | uint16(255-v)<<8
				mem.events = nil
				before := mem.data
				mem.data[0xeff0], mem.data[0xeff1] = 0, 3
				cpu.SetSP(0xeff0)
				cpu.SetPC(uint16(a.Symbols[name]))
				f := s.MIR2Module.FuncByName(name)
				if e := memoryHostSetArgument(&mem.memoryHostZ80Memory, assertParamLocation(f.Contract.Params[0], s.Allocation), 65535); e != nil {
					t.Fatal(e)
				}
				if name == "st_word" {
					if e := memoryHostSetArgument(&mem.memoryHostZ80Memory, assertParamLocation(f.Contract.Params[1], s.Allocation), int64(val)); e != nil {
						t.Fatal(e)
					}
				}
				for n := 0; n < 1000 && cpu.PC() != 0x300; n++ {
					cpu.DoOpcode()
				}
				if cpu.PC() != 0x300 || cpu.SP() != 0xeff2 {
					t.Fatal("native return", fused, name, v, cpu.PC(), cpu.SP())
				}
				op := mir2.OpLoad
				if name == "st_word" {
					op = mir2.OpStore
				}
				want := []mir2.VMDataAccess{{Op: op, Addr: 65535, Value: byte(val)}, {Op: op, Addr: 0, Value: byte(val >> 8)}}
				if !reflect.DeepEqual(mem.events, want) {
					t.Fatal("native wrap order", fused, name, v, mem.events, want)
				}
				if name == "ld_word" && cpu.HL() != val {
					t.Fatal("native word", fused, v, cpu.HL(), val)
				}
				for addr := range mem.data {
					if addr == 0 || addr == 65535 || addr >= 0xef00 && addr <= 0xeff1 {
						continue
					}
					if mem.data[addr] != before[addr] {
						t.Fatal("unrelated RAM", fused, name, addr)
					}
				}
				if !bytes.Equal(mem.data[0x8000:0x8000+len(a.Binary)], a.Binary) {
					t.Fatal("code mutation")
				}
			}
		}
	}
}

func TestByteWordFusionCompilationIdentity(t *testing.T) {
	a, e := ProfileCompilationID(DefaultOptions(), "default")
	if e != nil {
		t.Fatal(e)
	}
	opts := DefaultOptions()
	opts.FuseByteWords = true
	b, e := ProfileCompilationID(opts, "default")
	if e != nil || a == b {
		t.Fatal("fusion not fingerprinted", e)
	}
}

func TestSequentialByteGlobalOriginAuthority(t *testing.T) {
	m := &mir2.Module{}
	m.Globals = []mir2.Global{{Name: "bytes", Ty: &mir2.ArrayTy{Elem: mir2.TyU8, Len: 4}, Init: []byte{52, 18, 7, 9}}}
	f := m.AddFunc("word")
	builder := mir2.NewBuilder(f)
	builder.SwitchToNewBlock("entry")
	p := builder.AddrOf("bytes", mir2.ClassPointer)
	p1 := builder.PtrAdd(p, builder.Const(1, mir2.TyU16, mir2.ClassPair), mir2.ClassPointer)
	lo := builder.Ext(builder.Load(p, mir2.TyU8, mir2.ClassGeneral), mir2.TyU8, mir2.TyU16, mir2.ClassPair)
	hi := builder.Ext(builder.Load(p1, mir2.TyU8, mir2.ClassGeneral), mir2.TyU8, mir2.TyU16, mir2.ClassPair)
	builder.Ret(builder.Or(lo, builder.Shl(hi, builder.Const(8, mir2.TyU16, mir2.ClassPair), mir2.TyU16, mir2.ClassPair), mir2.TyU16, mir2.ClassPair))
	if mir2.FuseSequentialByteWords(f) != 1 {
		t.Fatal("global didn't fuse", m.Dump())
	}
	var wide *mir2.Inst
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x.SequentialBytes {
				wide = x
			}
		}
	}
	origins := hir.MemoryHostGlobalAccesses(m)
	for _, o := range wide.SequentialByteOrigins {
		if origins[o] != "bytes" {
			t.Fatal("original global provenance lost")
		}
	}
	forged := *wide.SequentialByteOrigins[1]
	forged.Src[0] = wide.Src[0]
	wide.SequentialByteOrigins[1] = &forged
	if mir2.Verify(m) == nil {
		t.Fatal("forged global origin passes verifier")
	}
	if _, granted := hir.MemoryHostGlobalAccesses(m)[&forged]; granted {
		t.Fatal("forged global origin obtained authority")
	}
}
