package pipeline

import (
	"bytes"
	"encoding/binary"
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

// Full unchanged FAT12 module. Native disk adapters implement only the declared
// success/failure ABI; all directory/body/control flow comes from the source.
func TestFATDirectoryWriteNativeState(t *testing.T) {
	const path = "../../../stdlib/fs/fat12.nanz"
	source, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_FAT_ADDRESS_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		switch x := v.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		default:
			var err error
			b, err = json.MarshalIndent(v, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
		}
		if e := os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	save("source.nanz", source)
	for _, mode := range []string{"default", "grace-size", "grace-speed"} {
		h, e := nanz.Parse(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "default", OptSize: mode == "grace-size"})
		if e != nil {
			t.Fatal(e)
		}
		assembly := strings.Replace(s.Assembly, "disk_read:\n", "disk_read:\n LD A,1\n RET\n", 1)
		assembly = strings.Replace(assembly, "disk_write:\n", "disk_write:\n XOR A\n RET\n", 1)
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		save(mode+"/target.a80", assembly)
		save(mode+"/target.bin", bin.Binary)
		save(mode+"/final.mir2", s.MIR2Module.Dump())
		save(mode+"/allocation.json", s.Allocation)
		for _, slot := range []int64{0, 1, 5, 15, 16, 255} {
			t.Run(fmt.Sprintf("%s/slot%d", mode, slot), func(t *testing.T) {
				sector := uint16(7 + slot/16)
				offset := int(slot&15) * 32
				name := []byte("README  TXT")
				want := make([]byte, 512)
				for i := range want {
					want[i] = byte(i*37 + 11)
				}
				copy(want[offset:], name)
				want[offset+11] = 32
				clear(want[offset+12 : offset+26])
				binary.LittleEndian.PutUint16(want[offset+26:], 0x1234)
				binary.LittleEndian.PutUint16(want[offset+28:], 0x5678)
				want[offset+30] = 0
				want[offset+31] = 0
				vmCheck := func(m *mir2.Module) {
					vm := mir2.NewVM(m)
					vm.EnsureHeap(65536)
					vm.WriteHeap(vm.GlobalAddr("fs_dirbase"), 7)
					vm.WriteHeap(vm.GlobalAddr("fs_dirbase")+1, 0)
					vm.WriteHeap(vm.GlobalAddr("win_sect"), byte(sector))
					vm.WriteHeap(vm.GlobalAddr("win_sect")+1, byte(sector>>8))
					for i := 0; i < 512; i++ {
						vm.WriteHeap(vm.GlobalAddr("win")+int64(i), byte(i*37+11))
					}
					for i, b := range name {
						vm.WriteHeap(0x4000+int64(i), b)
					}
					calls := 0
					vm.Hosts["disk_read"] = func([]mir2.Value) ([]mir2.Value, error) { t.Fatal("unexpected VM disk_read"); return nil, nil }
					vm.Hosts["disk_write"] = func(a []mir2.Value) ([]mir2.Value, error) {
						calls++
						if len(a) != 4 || a[0].I != 0 || a[1].I != vm.GlobalAddr("win") || a[2].I != int64(sector) || a[3].I != 1 || !bytes.Equal(vm.ReadHeap(a[1].I, 512), want) {
							t.Fatal("VM disk write oracle", a)
						}
						return []mir2.Value{{I: 0}}, nil
					}
					v, e := vm.Call("write_dir_entry", []mir2.Value{{I: 0}, {I: slot}, {I: 0x4000}, {I: 32}, {I: 0x1234}, {I: 0x5678}})
					if e != nil || len(v) != 1 || v[0].I != 0 || calls != 1 || !bytes.Equal(vm.ReadHeap(vm.GlobalAddr("win"), 512), want) || vm.ReadHeap(vm.GlobalAddr("win_dirty"), 1)[0] != 0 {
						t.Fatal("VM final FAT oracle", v, e, calls)
					}
				}
				raw := hir.LowerModule(h)
				vmCheck(raw)
				vmCheck(s.MIR2Module)
				f := s.MIR2Module.FuncByName("write_dir_entry")
				assert := hir.Assert{FuncName: f.Name, Args: []int64{0, slot, 0x4000, 32, 0x1234, 0x5678}, Expected: 0}
				boot := buildAssertBootstrap(assertLoadAddr, assert, f, s.Allocation)
				a, e := z80asm.NewAssembler().AssembleString(boot + "\n" + assembly)
				if e != nil || len(a.Errors) > 0 {
					t.Fatal(e, a.Errors)
				}
				platform := emulator.NewRemogattoZ80WithScreen()
				platform.SetROMEnd(0)
				z := platform.RemogattoZ80
				z.SetSMCTracker(nil)
				z.LoadMemory(assertLoadAddr, a.Binary)
				for name, v := range map[string]uint16{"fs_dirbase": 7, "win_sect": sector} {
					p := uint16(a.Symbols[name])
					z.SetMemory(p, byte(v))
					z.SetMemory(p+1, byte(v>>8))
				}
				win := uint16(a.Symbols["win"])
				for i := 0; i < 512; i++ {
					z.SetMemory(win+uint16(i), byte(i*37+11))
				}
				for i, b := range name {
					z.SetMemory(0x4000+uint16(i), b)
				}
				var before [65536]byte
				for p := range before {
					before[p] = z.GetMemory(uint16(p))
				}
				z.SetPC(assertLoadAddr)
				for i := 0; z.GetPC() != uint16(a.Symbols[f.Name]); i++ {
					if i > 1000 {
						t.Fatal("bootstrap runaway")
					}
					z.Step()
				}
				sp := z.GetSP()
				ret := uint16(z.GetMemory(sp)) | uint16(z.GetMemory(sp+1))<<8
				start := z.GetCycles()
				diskCalls := 0
				minSP := sp
				for i := 0; ; i++ {
					if i > 100000 {
						t.Fatal("native runaway")
					}
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
					if z.GetPC() == uint16(a.Symbols["disk_read"]) {
						t.Fatal("unexpected native disk_read")
					}
					if z.GetPC() == uint16(a.Symbols["disk_write"]) {
						diskCalls++
						r := platform.GetRegisters()
						got := make([]byte, 512)
						for j := range got {
							got[j] = z.GetMemory(r.HL + uint16(j))
						}
						if r.A != 0 || r.HL != win || r.DE != sector || byte(r.BC) != 1 || !bytes.Equal(got, want) {
							t.Fatal("native disk write oracle", r)
						}
					}
					z.Step()
					if z.GetPC() == ret && z.GetSP() == sp+2 {
						break
					}
				}
				if e := checkAssertZ80Result(z, assert, f, map[string]*hir.Func{f.Name: h.FuncByName(f.Name)}); e != nil {
					t.Fatal(e)
				}
				if diskCalls != 1 {
					t.Fatal("disk call count", diskCalls)
				}
				allowed := map[uint16]bool{}
				for _, g := range s.MIR2Module.Globals {
					p := uint16(a.Symbols[g.Name])
					for i := 0; i < mir2.ByteWidth(g.Ty); i++ {
						addr := p + uint16(i)
						allowed[addr] = true
						if g.Name != "win" && g.Name != "win_dirty" && z.GetMemory(addr) != before[addr] {
							t.Fatalf("unexpected global mutation %s at %04x", g.Name, addr)
						}
					}
				}
				for _, l := range a.Listing {
					if strings.HasPrefix(l.Label, "_spill_") {
						for i := range l.Bytes {
							allowed[uint16(l.Address+i)] = true
						}
					}
				}
				for p := minSP; p < 0xff00; p++ {
					allowed[p] = true
				}
				for p, b := range before {
					if !allowed[uint16(p)] && z.GetMemory(uint16(p)) != b {
						t.Fatalf("unexpected64KiB write %04x", p)
					}
				}
				got := make([]byte, 512)
				for i := range got {
					got[i] = z.GetMemory(win + uint16(i))
				}
				if !bytes.Equal(got, want) || z.GetMemory(uint16(a.Symbols["win_dirty"])) != 0 {
					t.Fatal("native window final")
				}
				save(fmt.Sprintf("%s/slot%d.json", mode, slot), map[string]any{"t_states": z.GetCycles() - start, "binary_bytes": len(bin.Binary), "window": got, "disk_calls": diskCalls, "sp": z.GetSP(), "oracle": want})
				t.Logf("%s slot%d %dT %dB rawVM/finalVM/native512+64KiB/SP PASS", mode, slot, z.GetCycles()-start, len(bin.Binary))
			})
		}
	}
}
