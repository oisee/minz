package pipeline

import (
	"bytes"
	"crypto/sha256"
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

func TestFATPackedFrozenNative(t *testing.T) {
	const path = "../../../stdlib/fs/fat12.nanz"
	const frozen = "../../../reports/fat-packed-prospective/workload.json"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(frozen)
	if e != nil {
		t.Fatal(e)
	}
	type sample struct{ Cluster, Base, Value int64 }
	var spec struct {
		SourceSHA256      string `json:"source_sha256"`
		Training, Heldout []sample
	}
	if e = json.Unmarshal(data, &spec); e != nil || fmt.Sprintf("%x", sha256.Sum256(src)) != spec.SourceSHA256 {
		t.Fatal("freeze/source", e)
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_FAT_PACKED_EXPORT")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, "native", name)
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
			var e error
			b, e = json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
		}
		if e := os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for _, mode := range []string{"default", "grace-size", "grace-speed"} {
		h, e := nanz.Parse(string(src), path)
		if e != nil {
			t.Fatal(e)
		}
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: mode != "default", OptSize: mode == "grace-size", AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		save(mode+"/target.a80", steps.Assembly)
		save(mode+"/final.mir", steps.MIR2Module.Dump())
		save(mode+"/allocation.json", steps.Allocation)
		image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil || len(image.Errors) > 0 {
			t.Fatal(e, image.Errors)
		}
		save(mode+"/target.bin", image.Binary)
		for _, phase := range []string{"training", "heldout"} {
			cases := spec.Training
			if phase == "heldout" {
				cases = spec.Heldout
			}
			for index, c := range cases {
				for _, bits := range []int64{12, 16} {
					for _, writing := range []bool{false, true} {
						ofs := c.Cluster * 2
						if bits == 12 {
							ofs = c.Cluster + c.Cluster/2
						}
						ofs &= 65535
						addr := uint16((c.Base + ofs) & 65535)
						initial := [2]byte{byte(uint64(addr)*29 + 17), byte(uint64(uint16(addr+1))*29 + 17)}
						raw := uint16(initial[0]) | uint16(initial[1])<<8
						value := uint16(c.Value)
						word := raw
						if writing {
							word = value
							if bits == 12 {
								if c.Cluster&1 != 0 {
									word = raw&15 | value<<4
								} else {
									word = raw&0xf000 | value&0xfff
								}
							}
						}
						want := int64(raw)
						if bits == 12 {
							if c.Cluster&1 != 0 {
								want = int64(raw >> 4)
							} else {
								want = int64(raw & 0xfff)
							}
						}
						prefix := "read"
						args := []int64{c.Base, c.Cluster}
						if writing {
							prefix = "write"
							args = append(args, c.Value)
						}
						name := fmt.Sprintf("%s_fat%d", prefix, bits)
						for _, m := range []*mir2.Module{hir.LowerModule(h), steps.MIR2Module} {
							vm := mir2.NewVM(m)
							vm.EnsureHeap(65536)
							vm.WriteHeap(int64(addr), initial[0])
							vm.WriteHeap(int64(uint16(addr+1)), initial[1])
							var events []mir2.VMDataAccess
							vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
							var va []mir2.Value
							for _, a := range args {
								va = append(va, mir2.Value{I: a})
							}
							v, e := vm.Call(name, va)
							if e != nil {
								t.Fatal(name, e)
							}
							if !writing && (len(v) != 1 || v[0].I != want) {
								t.Fatal("independentraw/finalVM read", name, c, v, want)
							}
							expected := 2
							if writing && bits == 12 {
								expected = 4
							}
							if len(events) != expected {
								t.Fatal("VM bytecount", events)
							}
							for j, a := range events {
								op := mir2.OpLoad
								wantByte := initial[j%2]
								if writing && (bits == 16 || j >= 2) {
									op = mir2.OpStore
									wantByte = byte(word >> (8 * (j % 2)))
								}
								if a.Op != op || a.Addr != int64(uint16(addr+uint16(j%2))) || a.Value != wantByte {
									t.Fatal("independentVMorderedbyte", name, c, events)
								}
							}
						}
						f := steps.MIR2Module.FuncByName(name)
						a := hir.Assert{FuncName: name, Args: args, Expected: want}
						boot := buildAssertBootstrap(assertLoadAddr, a, f, steps.Allocation)
						bin, e := z80asm.NewAssembler().AssembleString(boot + "\n" + steps.Assembly)
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
						z.SetMemory(addr, initial[0])
						z.SetMemory(addr+1, initial[1])
						z.SetPC(assertLoadAddr)
						for j := 0; z.GetPC() != uint16(bin.Symbols[name]); j++ {
							if j > 1000 {
								t.Fatal("entry")
							}
							z.Step()
						}
						sp := z.GetSP()
						minSP := sp
						ret := uint16(z.GetMemory(sp)) | uint16(z.GetMemory(sp+1))<<8
						before := append([]byte(nil), z.Memory()...)
						prof := emulator.NewProfiler()
						z.SetProfiler(prof)
						start := z.GetCycles()
						for j := 0; ; j++ {
							if j > 10000 {
								t.Fatal("return")
							}
							if z.GetPC() == uint16(bin.Symbols["disk_read"]) || z.GetPC() == uint16(bin.Symbols["disk_write"]) {
								t.Fatal("unexpecteddiskextern")
							}
							z.Step()
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
							if z.GetPC() == ret && z.GetSP() == sp+2 {
								break
							}
						}
						if !writing {
							if e := checkAssertZ80Result(z, a, f, map[string]*hir.Func{name: h.FuncByName(name)}); e != nil {
								t.Fatal(c, e)
							}
						}
						for j := 0; j < 2; j++ {
							p := addr + uint16(j)
							writes := uint32(0)
							if writing {
								writes = 1
							}
							reads := uint32(1)
							if writing && bits == 16 {
								reads = 0
							}
							if prof.ReadCount[p] != reads || prof.WriteCount[p] != writes {
								t.Fatal("nativeexactdataaccess", name, c, p, prof.ReadCount[p], prof.WriteCount[p])
							}
							if writing && z.GetMemory(p) != byte(word>>(8*j)) {
								t.Fatal("independentnativepackedstore", name, c, p, z.GetMemory(p), word)
							}
						}
						allowed := map[uint16]bool{}
						for _, l := range bin.Listing {
							if len(l.Label) >= 7 && l.Label[:7] == "_spill_" {
								for j := range l.Bytes {
									allowed[uint16(l.Address+j)] = true
								}
							}
						}
						for p, b := range before {
							q := uint16(p)
							if q >= minSP && q < sp || allowed[q] || writing && (q == addr || q == addr+1) {
								continue
							}
							if z.GetMemory(q) != b {
								t.Fatalf("unexpectedfullmemorymutation %s %04x", name, q)
							}
						}
						if !bytes.Equal(z.Memory()[uint32(addr):uint32(addr)+1], []byte{byte(word)}) {
							t.Fatal("targetlowbyte")
						}
						save(fmt.Sprintf("%s/%s-%d-%s.json", mode, phase, index, name), map[string]any{"case": c, "target_address": addr, "initial_word": raw, "final_word": word, "result": want, "t_states": z.GetCycles() - start, "binary_bytes": len(image.Binary), "native_reads": []uint32{prof.ReadCount[addr], prof.ReadCount[addr+1]}, "native_writes": []uint32{prof.WriteCount[addr], prof.WriteCount[addr+1]}, "same_frozen_binary": fmt.Sprintf("%x", sha256.Sum256(image.Binary))})
						t.Logf("%s %s %s case%d %dT %dB independentpacked/rawVM/finalVM/nativeRAM/bytes/SP PASS", mode, phase, name, index, z.GetCycles()-start, len(image.Binary))
					}
				}
			}
		}
	}
}
