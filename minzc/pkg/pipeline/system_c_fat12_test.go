package pipeline

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

type fatImageDevice struct {
	image      []byte
	lba        uint16
	cursor     int
	status     byte
	reads      []uint16
	failSector int
}

func (d *fatImageDevice) WritePort(port uint16, value byte) {
	switch port {
	case 0x10e0:
		d.lba = (d.lba & 0xff00) | uint16(value)
	case 0x10e1:
		d.lba = (d.lba & 255) | uint16(value)<<8
	case 0x10e2:
		d.cursor = 0
		d.status = 0
		d.reads = append(d.reads, d.lba)
		if value != 1 || int(d.lba)*512+512 > len(d.image) || int(d.lba) == d.failSector {
			d.status = 1
		}
	default:
		panic(fmt.Sprintf("unexpected output port %04x", port))
	}
}
func (d *fatImageDevice) ReadPort(port uint16) byte {
	switch port {
	case 0x10e2:
		return d.status
	case 0x10e3:
		if d.status != 0 || d.cursor >= 512 {
			panic("invalid sector data read")
		}
		v := d.image[int(d.lba)*512+d.cursor]
		d.cursor++
		return v
	default:
		panic(fmt.Sprintf("unexpected input port %04x", port))
	}
}
func bindFatImageVM(vm *mir2.VM, d *fatImageDevice) {
	vm.Hosts["fat12__lba_low"] = func(v []mir2.Value) ([]mir2.Value, error) { d.WritePort(0x10e0, byte(v[0].I)); return nil, nil }
	vm.Hosts["fat12__lba_high"] = func(v []mir2.Value) ([]mir2.Value, error) { d.WritePort(0x10e1, byte(v[0].I)); return nil, nil }
	vm.Hosts["fat12__read_command"] = func(v []mir2.Value) ([]mir2.Value, error) { d.WritePort(0x10e2, 1); return nil, nil }
	vm.Hosts["fat12__read_status"] = func(v []mir2.Value) ([]mir2.Value, error) { return []mir2.Value{{I: int64(d.ReadPort(0x10e2))}}, nil }
	vm.Hosts["fat12__read_data"] = func(v []mir2.Value) ([]mir2.Value, error) { return []mir2.Value{{I: int64(d.ReadPort(0x10e3))}}, nil }
}

func TestSystemCFAT12ImageNativeVM(t *testing.T) {
	root := filepath.Clean("../../..")
	dir := filepath.Join(root, "examples/system-c/fat12")
	imgPath := filepath.Join(t.TempDir(), "demo.img")
	if out, e := exec.Command("python3", filepath.Join(dir, "create_image.py"), imgPath).CombinedOutput(); e != nil {
		t.Fatal(e, string(out))
	}
	image, e := os.ReadFile(imgPath)
	if e != nil {
		t.Fatal(e)
	}
	payload, e := os.ReadFile(imgPath[:len(imgPath)-4] + ".txt")
	if e != nil {
		t.Fatal(e)
	}
	ref := filepath.Join(t.TempDir(), "fatfs-reference")
	fatfs := filepath.Join(root, "examples/c89/fatfs")
	if out, e := exec.Command("gcc", "-std=c99", "-I", fatfs, filepath.Join(dir, "reference.c"), filepath.Join(fatfs, "ff.c"), "-o", ref).CombinedOutput(); e != nil {
		t.Fatal("reference build", e, string(out))
	}
	if out, e := exec.Command(ref, imgPath).Output(); e != nil || !bytes.Equal(out, payload) {
		t.Fatal("independent FatFS oracle", e, len(out), len(payload))
	}
	src, e := os.ReadFile(filepath.Join(dir, "main.nanz"))
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		m, e := nanz.ParseWithOpts(string(src), filepath.Join(dir, "main.nanz"), nanz.ParseOpts{BaseDir: dir})
		if e != nil {
			t.Fatal(e)
		}
		return m
	}

	clone := func() []byte { return append([]byte(nil), image...) }
	badBoot := clone()
	badBoot[510] = 0
	shortChain := clone()
	shortChain[512+3] = 255
	shortChain[512+4] = (shortChain[512+4] & 0xf0) | 15
	cycle := clone()
	cycle[512+3] = 2
	cycle[512+4] &= 0xf0
	empty := clone()
	for i := 26; i < 32; i++ {
		empty[19*512+i] = 0
	}
	cases := []struct {
		name           string
		image, payload []byte
		status         byte
		failSector     int
		missing        bool
	}{
		{name: "fragmented", image: image, payload: payload, failSector: -1},
		{name: "empty", image: empty, failSector: -1},
		{name: "invalid_boot", image: badBoot, status: 2, failSector: -1},
		{name: "missing", image: image, status: 3, failSector: -1, missing: true},
		{name: "io_boot", image: image, status: 1, failSector: 0},
		{name: "io_data", image: image, status: 1, failSector: 33},
		{name: "short_chain", image: shortChain, status: 4, failSector: -1},
		{name: "cycle", image: cycle, status: 4, failSector: -1},
	}
	for _, mode := range []string{"default", "grace", "size"} {
		for _, tc := range cases {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				var checksum uint16
				for _, v := range tc.payload {
					checksum = checksum*33 + uint16(v)
				}
				raw := hir.LowerModule(parse())
				steps, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none", UseHIRGrace: mode != "default", UseGrace: mode != "default", OptSize: mode == "size"})
				if e != nil {
					t.Fatal(e)
				}
				observations := [][]uint16{}
				for stage, m := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
					vm := mir2.NewVM(m)
					vm.MaxSteps = 3000000
					if tc.missing {
						vm.WriteHeap(vm.GlobalAddr("filename"), 88)
					}
					d := &fatImageDevice{image: tc.image, failSector: tc.failSector}
					bindFatImageVM(vm, d)
					var output []byte
					vm.Hosts["@mir.io.console.log"] = func(v []mir2.Value) ([]mir2.Value, error) { output = append(output, byte(v[0].I)); return nil, nil }
					if _, e := vm.Call("run", nil); e != nil {
						t.Fatal(stage, e)
					}
					status := vm.ReadHeap(vm.GlobalAddr("status"), 1)[0]
					n := binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("bytes_read"), 4))
					sum := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("checksum"), 2))
					if status != tc.status || n != uint32(len(tc.payload)) || sum != checksum || !bytes.Equal(output, tc.payload) {
						t.Fatalf("%s status%d bytes%d checksum%04x want%d %04x requests%v", stage, status, n, sum, len(tc.payload), checksum, d.reads)
					}
					observations = append(observations, d.reads)
				}
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil || len(bin.Errors) > 0 {
					t.Fatal(e, bin.Errors)
				}
				symbol := func(name string) int {
					v := bin.Symbols[name]
					if v == 0 {
						v = bin.Symbols["v_"+name]
					}
					if v == 0 {
						t.Fatal("missing symbol", name)
					}
					return v
				}
				z := emulator.NewRemogattoZ80()
				z.LoadMemory(0x8000, bin.Binary)
				z.LoadMemory(0xeffe, []byte{0, 0xf1})
				d := &fatImageDevice{image: tc.image, failSector: tc.failSector}
				var output []byte
				z.SetIOHandlers(d.ReadPort, func(port uint16, v byte) {
					if port&255 == 0x23 {
						output = append(output, v)
					} else {
						d.WritePort(port, v)
					}
				})
				if tc.missing {
					z.Memory()[symbol("filename")] = 88
				}
				z.SetRegisters(emulator.Registers{PC: uint16(symbol("run")), SP: 0xeffe})
				for i := 0; i < 10000000 && z.GetRegisters().PC != 0xf100; i++ {
					z.Step()
				}
				r := z.GetRegisters()
				status := z.Memory()[symbol("status")]
				n := binary.LittleEndian.Uint32(z.Memory()[symbol("bytes_read") : symbol("bytes_read")+4])
				sum := binary.LittleEndian.Uint16(z.Memory()[symbol("checksum") : symbol("checksum")+2])
				if status != tc.status || n != uint32(len(tc.payload)) || sum != checksum || !bytes.Equal(output, tc.payload) || r.PC != 0xf100 || r.SP != 0xf000 {
					os.WriteFile("/tmp/minz-system-fat12-failure.a80", []byte(steps.Assembly), 0644)
					a := symbol("fat12__fat_sector")
					t.Fatalf("native status%d bytes%d checksum%04x want%d %04x regs%v reads%v BPB%x", status, n, sum, len(tc.payload), checksum, r, d.reads, z.Memory()[a:a+32])
				}
				for _, reads := range observations {
					if !bytes.Equal(uint16Bytes(reads), uint16Bytes(d.reads)) {
						t.Fatal("sector sequence differs", reads, d.reads)
					}
				}
				if tc.name == "fragmented" && !bytes.Equal(uint16Bytes(d.reads), uint16Bytes([]uint16{0, 19, 33, 1, 38, 1, 372, 1, 2, 373})) {
					t.Fatal("unexpected FAT chain", d.reads)
				}
				if export := os.Getenv("SYSTEM_C_FAT12_EXPORT"); export != "" && tc.name == "fragmented" {
					if e := os.MkdirAll(export, 0755); e != nil {
						t.Fatal(e)
					}
					for name, data := range map[string][]byte{mode + ".a80": []byte(steps.Assembly), mode + ".bin": bin.Binary, mode + "-read.txt": output, "demo.img": image, "expected.txt": payload} {
						if e := os.WriteFile(filepath.Join(export, name), data, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
				t.Logf("FAT12 %d bytes checksum %04x sector requests%v native BIN%d", len(tc.payload), checksum, d.reads, len(bin.Binary))
			})
		}
	}
}
func uint16Bytes(values []uint16) []byte {
	out := make([]byte, len(values)*2)
	for i, v := range values {
		binary.LittleEndian.PutUint16(out[2*i:], v)
	}
	return out
}
