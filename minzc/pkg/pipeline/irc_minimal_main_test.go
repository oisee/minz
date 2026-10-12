package pipeline

import (
	"bytes"
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

func TestIRCMinimalWholeMain(t *testing.T) {
	root := "../../../reports/irc-minimal-main/"
	input, e := os.ReadFile(root + "input.bin")
	if e != nil {
		t.Fatal(e)
	}
	expected, e := os.ReadFile(root + "stdout.bin")
	if e != nil {
		t.Fatal(e)
	}
	wire, e := os.ReadFile(root + "wire.bin")
	if e != nil {
		t.Fatal(e)
	}
	source, e := os.ReadFile("../../../examples/nanz/test_irc_minimal.nanz")
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(root + "freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SHA string `json:"source_sha256"`
	}
	if e = json.Unmarshal(b, &freeze); e != nil || fmt.Sprintf("%x", sha256.Sum256(source)) != freeze.SHA {
		t.Fatal("freeze", e)
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			parse := func() *hir.Module {
				h, e := nanz.ParseWithOpts(string(source), "test_irc_minimal.nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			raw := hir.LowerModule(parse())
			s, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			for _, m := range []*mir2.Module{raw, s.MIR2Module} {
				vm := mir2.NewVM(m)
				vm.MaxSteps = 1000000
				var out, sent []byte
				reads := 0
				for i := 0; i < m.Strings.Len(); i++ {
					if _, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", i)); e != nil {
						t.Fatal(e)
					}
				}
				vm.EnsureHeap(65536)
				before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
				vm.Hosts["net_read"] = func([]mir2.Value) ([]mir2.Value, error) {
					if reads >= len(input) {
						return nil, fmt.Errorf("extra network read")
					}
					v := input[reads]
					reads++
					return []mir2.Value{{I: int64(v)}}, nil
				}
				vm.Hosts["net_write"] = func(v []mir2.Value) ([]mir2.Value, error) { sent = append(sent, byte(v[0].I)); return nil, nil }
				vm.Hosts["@mir.io.print.char"] = func(v []mir2.Value) ([]mir2.Value, error) { out = append(out, byte(v[0].I)); return nil, nil }
				vm.Hosts["@mir.io.print.nl"] = func([]mir2.Value) ([]mir2.Value, error) { out = append(out, 10); return nil, nil }
				for _, name := range []string{"ctl_read", "ctl_write"} {
					name := name
					vm.Hosts[name] = func([]mir2.Value) ([]mir2.Value, error) { return nil, fmt.Errorf("unexpected control host %s", name) }
				}
				if _, e := vm.Call("main", nil); e != nil {
					t.Fatal(e)
				}
				if reads != 500 || !bytes.Equal(sent, wire) || !bytes.Equal(out, expected) || !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
					t.Fatalf("VM main reads%d wire%q stdout%q", reads, sent, out)
				}
			}
			asm := s.Assembly
			for _, name := range []string{"net_read", "net_write", "ctl_read", "ctl_write"} {
				asm = strings.ReplaceAll(asm, name+":\n", name+":\n    RET\n")
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x100\n" + asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.LoadMemory(0x100, a.Binary)
			mem := z.Memory()
			main := a.Symbols["main"]
			z.LoadMemory(0x7000, []byte{0xcd, byte(main), byte(main >> 8), 0x76})
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000})
			before := append([]byte(nil), mem...)
			var out, sent []byte
			reads := 0
			z.SetIOHandlers(func(p uint16) byte { t.Fatal("unexpected IN", p); return 0 }, func(p uint16, v byte) {
				if byte(p) != 0x23 || byte(p>>8) != v {
					t.Fatal("print port", p, v)
				}
				out = append(out, v)
			})
			minSP := uint16(0xf000)
			done := false
			cycles := 0
			for k := 0; k < 1000000; k++ {
				pc := z.GetPC()
				if pc == 0x7003 && z.GetSP() == 0xf000 {
					done = true
					break
				}
				r := z.GetRegisters()
				switch int(pc) {
				case a.Symbols["net_read"]:
					if reads >= len(input) {
						t.Fatal("extra native read")
					}
					r.A = input[reads]
					reads++
					z.SetRegisters(r)
				case a.Symbols["net_write"]:
					loc := assertParamLocation(s.MIR2Module.FuncByName("net_write").Contract.Params[0], s.Allocation)
					if loc.Name != "A" {
						t.Fatal("host ABI", loc)
					}
					sent = append(sent, r.A)
				case a.Symbols["ctl_read"], a.Symbols["ctl_write"]:
					t.Fatal("unexpected control entry")
				}
				if pc != 0x7000 && (pc < 0x100 || int(pc) >= 0x100+len(a.Binary)) {
					t.Fatal("foreign PC", pc)
				}
				cycles += z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			if !done || reads != 500 || !bytes.Equal(sent, wire) || !bytes.Equal(out, expected) {
				t.Fatalf("native done%v reads%d wire%q stdout%q", done, reads, sent, out)
			}
			for i, b := range before {
				if i >= int(minSP) && i < 0xf000 {
					continue
				}
				if mem[i] != b {
					t.Fatalf("RAM%x", i)
				}
			}
			if dir := os.Getenv("MINZ_IRC_MINIMAL_EXPORT"); dir != "" {
				dir = filepath.Join(dir, mode)
				if e := os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				for name, data := range map[string][]byte{"linked.a80": []byte("ORG 0x100\n" + asm), "unlinked.a80": []byte("ORG 0x100\n" + s.Assembly), "program.bin": a.Binary, "before.ram": before, "after.ram": mem, "stdout.bin": out, "wire.bin": sent, "final.mir": []byte(s.MIR2Module.Dump())} {
					if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			t.Logf("whole main %d B %d T,500 reads,%d wire,%d stdout,PC7003/SPf000", len(a.Binary), cycles, len(sent), len(out))
		})
	}
}
