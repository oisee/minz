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

// Book v7 §3.1 u8 wrap arithmetic; all original source entries and assertions.
func TestOriginalTetrisGeometryAndBoard(t *testing.T) {
	data, e := os.ReadFile("../../../reports/tetris-mir2-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		SourceSHA256 string     `json:"source_sha256"`
		Grids        [][]string `json:"geometry_grids"`
		Colors       []int64    `json:"piece_colors"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil {
		t.Fatal("freeze", e)
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_TETRIS_MIR2_ARTIFACTS")
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
	for _, module := range []string{"tetris_mir2_test"} {
		path := "../../../examples/zx/" + module + ".nanz"
		source, e := os.ReadFile(path)
		if e != nil || freeze.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(source)) {
			t.Fatal("sourcefreeze", module, e)
		}
		parse := func() *hir.Module {
			h, e := nanz.Parse(string(source), path)
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
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			if len(a.Binary) > 0x6000 {
				t.Fatal("imagestackoverlap")
			}
			prefix := module + "/" + mode
			save(prefix+"/target.a80", s.Assembly)
			save(prefix+"/target.bin", a.Binary)
			save(prefix+"/raw.mir", raw.Dump())
			save(prefix+"/final.mir", s.MIR2Module.Dump())
			save(prefix+"/allocation.json", s.Allocation)
			vms := map[string]*mir2.VM{}
			vmBefore := map[string][]byte{}
			for label, m := range map[string]*mir2.Module{"raw": raw, "final": s.MIR2Module} {
				vm := mir2.NewVM(m)
				vm.EnsureHeap(65536)
				vms[label] = vm
				vmBefore[label] = append([]byte(nil), vm.ReadHeap(0, 65536)...)
			}
			z := emulator.NewRemogattoZ80()
			copy(z.Memory(), bytes.Repeat([]byte{0xa5}, 65536))
			z.LoadMemory(0x8000, a.Binary)
			mem := z.Memory()
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			baseline := append([]byte(nil), mem...)
			spills := map[int]bool{}
			for _, l := range a.Listing {
				if len(l.Label) > 7 && l.Label[:7] == "_spill_" {
					for j := range l.Bytes {
						spills[l.Address+j] = true
					}
				}
			}
			count := 0
			totalT := 0
			check := func(entry string, args []int64, want int64) {
				count++
				for label, vm := range vms {
					vs := make([]mir2.Value, len(args))
					for j, v := range args {
						vs[j] = mir2.Value{I: v}
					}
					var events []mir2.VMDataAccess
					vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
					v, e := vm.Call(entry, vs)
					if entry == "board_get" || entry == "board_set" {
						base, e := vm.ResolveSymbol("board")
						if e != nil {
							t.Fatal(e)
						}
						op := mir2.OpLoad
						value := byte(want)
						if entry == "board_set" {
							op = mir2.OpStore
							value = byte(args[2])
						}
						if len(events) != 1 || events[0].Op != op || events[0].Addr != base+10*args[1]+args[0] || events[0].Value != value {
							t.Fatal("exactboardeffect", entry, args, events)
						}
					}
					if e != nil {
						t.Fatal(module, mode, label, entry, args, e)
					}
					decoded := int64(0)
					if len(v) == 1 {
						decoded = v[0].I

					}
					if entry != "main" && entry != "board_set" && (len(v) != 1 || decoded != want) {
						t.Fatal(module, mode, label, entry, args, v, want)
					}
					if entry == "board_set" {
						addr, e := vm.ResolveSymbol("board")
						if e != nil {
							t.Fatal(e)
						}
						vmBefore[label][addr+10*args[1]+args[0]] = byte(args[2])
					}
					if !bytes.Equal(vm.ReadHeap(0, 65536), vmBefore[label]) {
						t.Fatal("VMRAM", entry, args)
					}
				}
				f := s.MIR2Module.FuncByName(entry)
				r := emulator.Registers{PC: uint16(a.Symbols[entry]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999}
				for j, arg := range args {
					v := uint16(arg)
					loc := assertParamLocation(f.Contract.Params[j], s.Allocation)
					switch loc.Name {
					case "A":
						r.A = byte(v)
					case "B":
						r.BC = r.BC&255 | v<<8
					case "C":
						r.BC = r.BC&0xff00 | v&255
					case "D":
						r.DE = r.DE&255 | v<<8
					case "E":
						r.DE = r.DE&0xff00 | v&255
					case "H":
						r.HL = r.HL&255 | v<<8
					case "L":
						r.HL = r.HL&0xff00 | v&255
					case "HL":
						r.HL = v
					case "DE":
						r.DE = v
					case "BC":
						r.BC = v
					default:
						t.Fatal("ABIhome", entry, loc)
					}
				}
				z.SetRegisters(r)
				start := z.GetCycles()
				minSP := uint16(0xeff0)
				done := false
				var events []mir2.VMDataAccess
				for n := 0; n < 10000; n++ {
					if (entry == "main" && z.IsHalted() && z.GetSP() == 0xeff0) || (entry != "main" && z.GetPC() == 0x7000 && z.GetSP() == 0xeff2) {
						done = true
						break
					}
					if z.GetSP() < 0xe000 || z.GetSP() > 0xeff2 {
						t.Fatal("stackbounds", entry, args)
					}
					events = append(events, lizpTokenizerNativeAccess(mem, z.GetPC(), z.GetRegisters())...)
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
				}
				if entry == "board_get" || entry == "board_set" {
					op := mir2.OpLoad
					value := byte(want)
					if entry == "board_set" {
						op = mir2.OpStore
						value = byte(args[2])
					}
					var boardEvents []mir2.VMDataAccess
					for _, ev := range events {
						if ev.Addr >= int64(a.Symbols["board"]) && ev.Addr < int64(a.Symbols["board"]+200) {
							boardEvents = append(boardEvents, ev)
						}
					}
					if len(boardEvents) != 1 || boardEvents[0].Op != op || boardEvents[0].Addr != int64(a.Symbols["board"])+10*args[1]+args[0] || boardEvents[0].Value != value {
						t.Fatal("nativeexactboardeffect", entry, args, boardEvents)
					}
				}
				if !done {
					t.Fatal("nativetermination", entry, args)
				}
				if entry == "main" {
					if z.GetRegisters().A != 0 {
						t.Fatal("mainstatus")
					}
				} else if entry != "board_set" {
					if e = checkAssertZ80Result(z, hir.Assert{FuncName: entry, Args: args, Expected: want}, f, map[string]*hir.Func{entry: h.FuncByName(entry)}); e != nil {
						t.Fatal(module, mode, entry, args, want, e)
					}
				}
				if entry == "board_set" {
					baseline[a.Symbols["board"]+int(10*args[1]+args[0])] = byte(args[2])
				}
				copy(baseline[minSP:0xeff0], mem[minSP:0xeff0])
				for p := range spills {
					baseline[p] = mem[p]
				}
				if !bytes.Equal(mem, baseline) {
					for p := range mem {
						if mem[p] != baseline[p] {
							t.Fatalf("%s %s %s %v RAM%x", module, mode, entry, args, p)
						}
					}
				}
				if len(z.GetOutput()) != 0 {
					t.Fatal("output")
				}
				totalT += z.GetCycles() - start
			}
			for _, assert := range h.Asserts {
				check(assert.FuncName, assert.Args, assert.Expected)
			}
			for typ := 0; typ < 7; typ++ {
				for rot := 0; rot < 4; rot++ {
					var cells [][2]int
					for y, row := range strings.Split(freeze.Grids[typ][rot], "/") {
						for x, ch := range row {
							if ch == '#' {
								cells = append(cells, [2]int{x, y})
							}
						}
					}
					if len(cells) != 4 {
						t.Fatal("tetromino grid", typ, rot, cells)
					}
					for c, xy := range cells {
						check("piece_dx", []int64{int64(typ), int64(rot), int64(c)}, int64(xy[0]))
						check("piece_dy", []int64{int64(typ), int64(rot), int64(c)}, int64(xy[1]))
					}
				}
				check("piece_color", []int64{int64(typ)}, freeze.Colors[typ])
			}
			check("color_attr", []int64{0}, 0)
			for j, v := range freeze.Colors {
				check("color_attr", []int64{int64(j + 1)}, v)
			}
			for y := int64(0); y < 20; y++ {
				for x := int64(0); x < 10; x++ {
					for value := int64(0); value < 256; value++ {
						check("board_set", []int64{x, y, value}, 0)
						check("board_get", []int64{x, y}, value)
					}
				}
			}

			check("main", nil, 0)
			save(prefix+"/evidence.json", map[string]any{"calls": count, "T_states": totalT, "source_asserts": len(h.Asserts), "source_sha256": freeze.SourceSHA256, "bin_sha256": fmt.Sprintf("%x", sha256.Sum256(a.Binary))})
			t.Log(module, mode, count, "native/raw/final independent fullstate PASS")
		}
	}
}
