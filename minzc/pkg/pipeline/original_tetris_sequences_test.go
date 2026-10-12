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

func TestOriginalTetrisSpawnHoldAndSequences(t *testing.T) {
	data, e := os.ReadFile("../../../reports/tetris-sequences-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		Sources           map[string]string `json:"sources"`
		Seeds             []int64           `json:"seeds"`
		ModelDependencies map[string]string `json:"model_dependencies"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil {
		t.Fatal(e)
	}
	var geometry struct {
		Geometry map[string][][][][]int `json:"geometry"`
	}
	previous, e := os.ReadFile("../../../reports/tetris-family-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(previous, &geometry); e != nil {
		t.Fatal(e)
	}
	dependencies := map[string]string{}
	for p, h := range freeze.Sources {
		dependencies[p] = h
	}
	for p, h := range freeze.ModelDependencies {
		dependencies[p] = h
	}
	for p, h := range dependencies {
		b, e := os.ReadFile("../../../" + p)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != h {
			t.Fatal("freeze", p, e)
		}
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_TETRIS_SEQUENCE_ARTIFACTS")
		if dir == "" {
			return
		}
		p := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(p), 0755)
		var b []byte
		switch v := v.(type) {
		case string:
			b = []byte(v)
		case []byte:
			b = v
		default:
			b, _ = json.MarshalIndent(v, "", "  ")
		}
		if e = os.WriteFile(p, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	for _, module := range []string{"tetris_cpm", "tetris_tui"} {
		src, e := os.ReadFile("../../../examples/nanz/" + module + ".nanz")
		if e != nil {
			t.Fatal(e)
		}
		parse := func() *hir.Module {
			h, e := nanz.ParseWithOpts(string(src), module+".nanz", nanz.ParseOpts{BaseDir: "../../../examples/nanz", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			raw := hir.LowerModule(parse())
			h := parse()
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size", ContractOpt: true})
			if e != nil {
				t.Fatal(module, mode, e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + s.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(module, mode, e, a.Errors)
			}
			if 0x4000+len(a.Binary) > 0xc000 {
				t.Fatal("image exceeds harness partition")
			}
			prefix := module + "/" + mode
			save(prefix+"/target.a80", s.Assembly)
			save(prefix+"/target.bin", a.Binary)
			save(prefix+"/raw.mir", raw.Dump())
			save(prefix+"/final.mir", s.MIR2Module.Dump())
			vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
			vmBases := [][]byte{}
			for _, vm := range vms {
				for i := 0; i < vm.Module.Strings.Len(); i++ {
					if _, e := vm.ResolveSymbol(fmt.Sprintf("@mir2.str.%d", i)); e != nil {
						t.Fatal(e)
					}
				}
				vm.EnsureHeap(65536)
				vmBases = append(vmBases, append([]byte(nil), vm.ReadHeap(0, 65536)...))
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			base := bytes.Repeat([]byte{0xa5}, 65536)
			copy(base[0x4000:], a.Binary)
			base[0xeff0], base[0xeff1] = 0, 0x70
			allowed := map[string]bool{}
			for _, name := range []string{"rng_next", "random_piece", "piece_dx", "piece_dy", "board_get", "board_set", "can_place", "lock_piece", "clear_lines", "ghost_y", "try_rotate", "score_lines", "spawn_piece", "hold_piece", "do_lock_and_clear"} {
				allowed[name] = true
			}
			fetch := make([]bool, 65536)
			for _, f := range s.MIR2Module.Funcs {
				if !allowed[f.Name] {
					continue
				}
				lo, ok := a.Symbols[f.Name]
				if !ok {
					t.Fatal("entry", f.Name)
				}
				hi := 0x4000 + len(a.Binary)
				for _, other := range s.MIR2Module.Funcs {
					if p, ok := a.Symbols[other.Name]; ok && p > lo && p < hi {
						hi = p
					}
				}
				for p := lo; p < hi; p++ {
					fetch[p] = true
				}
				for _, b := range f.Blocks {
					for _, i := range b.Insts {
						if i.Op == mir2.OpAsm || i.Op == mir2.OpCallIndirect || i.Op == mir2.OpCall && !allowed[i.Sym] {
							t.Fatal("opaque closure", f.Name, i)
						}
					}
				}
			}
			spills := map[int]bool{}
			for _, l := range a.Listing {
				if strings.HasPrefix(l.Label, "_spill_") {
					for j := range l.Bytes {
						spills[l.Address+j] = true
					}
				}
			}
			widths := map[string]int{}
			for _, g := range s.MIR2Module.Globals {
				widths[g.Name] = mir2.ByteWidth(g.Ty)
			}
			apply := func(mem []byte, state tetrisFamilyState, addr func(string) int) {
				copy(mem[addr("board"):], state.Board)
				for n, v := range state.Globals {
					p := addr(n)
					for j := 0; j < widths[n]; j++ {
						mem[p+j] = byte(v >> uint(8*j))
					}
				}
			}
			count := 0
			byEntry := map[string]int{}
			persistent := false
			check := func(entry string, args []int64, in tetrisFamilyState) tetrisFamilyState {
				want, out, expected := tetrisSequenceModel(entry, args, in, geometry.Geometry[module])
				count++
				if count%65536 == 0 {
					t.Log(module, mode, count, "completed calls")
				}
				byEntry[entry]++
				for j, vm := range vms {
					before := append([]byte(nil), vmBases[j]...)
					addr := func(n string) int {
						p, e := vm.ResolveSymbol(n)
						if e != nil {
							t.Fatal(e)
						}
						return int(p)
					}
					apply(before, in, addr)
					if persistent {
						if !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
							t.Fatal("persistent VM checkpoint input", entry)
						}
					} else {
						vm.WriteHeapSlice(0, before)
					}
					after := append([]byte(nil), before...)
					apply(after, out, addr)
					var events []tetrisFamilyEvent
					board := int64(addr("board"))
					vm.ObserveDataAccess = func(v mir2.VMDataAccess) {
						if v.Addr >= board && v.Addr < board+200 {
							events = append(events, tetrisFamilyEvent{v.Op, int(v.Addr - board), v.Value})
						}
					}
					vs := []mir2.Value{}
					for _, v := range args {
						vs = append(vs, mir2.Value{I: v})
					}
					vm.MaxSteps = vm.StepsExecuted() + 5000000
					got, e := vm.Call(entry, vs)
					if e != nil || len(got) > 0 && got[0].I != want {
						t.Fatal(module, mode, "VM", j, entry, args, got, want, e)
					}
					if fmt.Sprint(events) != fmt.Sprint(expected) {
						save(prefix+"/failure.json", map[string]any{"engine": j, "entry": entry, "args": args, "input": in, "expected": out, "expected_events": expected, "actual_events": events, "actual_return": got})
						t.Fatal(module, mode, "VM board sequence", j, entry, args, len(events), len(expected))
					}
					if !bytes.Equal(after, vm.ReadHeap(0, 65536)) {
						t.Fatal(module, mode, "VM fullstate", entry, args)
					}
				}
				if !persistent {
					copy(z.Memory(), base)
					apply(z.Memory(), in, func(n string) int { return a.Symbols[n] })
				} else {
					expectedInput := append([]byte(nil), z.Memory()...)
					apply(expectedInput, in, func(n string) int { return a.Symbols[n] })
					if !bytes.Equal(expectedInput, z.Memory()) {
						t.Fatal("persistent native checkpoint input", entry)
					}
				}
				before := append([]byte(nil), z.Memory()...)
				after := append([]byte(nil), before...)
				apply(after, out, func(n string) int { return a.Symbols[n] })
				r := emulator.Registers{PC: uint16(a.Symbols[entry]), SP: 0xeff0, A: 0x42, BC: 0x5555, DE: 0x6666, HL: 0x7777, IX: 0x8888, IY: 0x9999}
				f := s.MIR2Module.FuncByName(entry)
				for i, v := range args {
					loc := assertParamLocation(f.Contract.Params[i], s.Allocation)
					switch loc.Name {
					case "A":
						r.A = byte(v)
					case "B":
						r.BC = r.BC&255 | uint16(byte(v))<<8
					case "C":
						r.BC = r.BC&0xff00 | uint16(byte(v))
					case "D":
						r.DE = r.DE&255 | uint16(byte(v))<<8
					case "E":
						r.DE = r.DE&0xff00 | uint16(byte(v))
					case "H":
						r.HL = r.HL&255 | uint16(byte(v))<<8
					case "L":
						r.HL = r.HL&0xff00 | uint16(byte(v))
					case "IXH":
						r.IX = r.IX&255 | uint16(byte(v))<<8
					case "IXL":
						r.IX = r.IX&0xff00 | uint16(byte(v))
					case "IYH":
						r.IY = r.IY&255 | uint16(byte(v))<<8
					case "IYL":
						r.IY = r.IY&0xff00 | uint16(byte(v))
					case "HL":
						r.HL = uint16(v)
					case "BC":
						r.BC = uint16(v)
					case "DE":
						r.DE = uint16(v)
					default:
						t.Fatal("ABI", loc)
					}
				}
				z.SetRegisters(r)
				minSP := uint16(0xeff0)
				done := false
				var events []tetrisFamilyEvent
				board := int64(a.Symbols["board"])
				for step := 0; step < 2000000; step++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						done = true
						break
					}
					if !fetch[z.GetPC()] {
						t.Fatal("runtime/foreign fetch", module, mode, entry, z.GetPC())
					}
					for _, v := range lizpTokenizerNativeAccess(z.Memory(), z.GetPC(), z.GetRegisters()) {
						if v.Addr >= board && v.Addr < board+200 {
							events = append(events, tetrisFamilyEvent{v.Op, int(v.Addr - board), v.Value})
						}
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
					if minSP < 0xe000 || z.GetSP() > 0xeff2 {
						t.Fatal("stack partition")
					}
				}
				if !done {
					t.Fatal(module, mode, "native termination", entry, args)
				}
				if len(f.Contract.Returns) > 0 {
					if e := checkAssertZ80Result(z.RemogattoZ80, hir.Assert{FuncName: entry, Args: args, Expected: want}, f, map[string]*hir.Func{entry: h.FuncByName(entry)}); e != nil {
						t.Fatal(module, mode, entry, args, want, e)
					}
				}
				if fmt.Sprint(events) != fmt.Sprint(expected) {
					t.Fatal(module, mode, "native board sequence", entry, args, events, expected)
				}
				copy(after[minSP:0xeff0], z.Memory()[minSP:0xeff0])
				for p := range spills {
					after[p] = z.Memory()[p]
				}
				if !bytes.Equal(after, z.Memory()) {
					for p := range after {
						if after[p] != z.Memory()[p] {
							t.Fatalf("%s %s %s %v fullRAM %04x got%d want%d", module, mode, entry, args, p, z.Memory()[p], after[p])
						}
					}
				}
				return out
			}
			state := tetrisFamilyState{make([]byte, 200), map[string]int64{"rng_state": 0, "score": 0, "lines_cleared": 0, "drop_speed": 8, "cur_type": 0, "cur_rot": 0, "cur_x": 3, "cur_y": 0}}
			state.Globals["next_type"] = 0
			state.Globals["hold_type"] = 255
			state.Globals["hold_used"] = 0
			state.Globals["game_over"] = 0
			boardFor := func(pattern int) []byte {
				b := make([]byte, 200)
				if pattern == 1 {
					for i := 0; i < 20; i++ {
						b[i] = 7
					}
				}
				if pattern == 2 {
					for i := 180; i < 200; i++ {
						b[i] = 7
					}
					b[193] = 0
				}
				return b
			}
			for typ := int64(0); typ < 7; typ++ {
				for _, seed := range freeze.Seeds {
					for pattern := 0; pattern < 3; pattern++ {
						for over := int64(0); over < 2; over++ {
							in := tetrisFamilyCopy(state)
							in.Board = boardFor(pattern)
							in.Globals["next_type"], in.Globals["rng_state"], in.Globals["game_over"] = typ, seed, over
							check("spawn_piece", nil, in)
						}
					}
				}
			}
			for typ := int64(0); typ < 7; typ++ {
				for _, hold := range []int64{255, 0, 1, 2, 3, 4, 5, 6} {
					for used := int64(0); used < 3; used++ {
						for _, seed := range freeze.Seeds {
							for pattern := 0; pattern < 2; pattern++ {
								in := tetrisFamilyCopy(state)
								in.Board = boardFor(pattern)
								in.Globals["cur_type"], in.Globals["hold_type"], in.Globals["hold_used"], in.Globals["rng_state"] = typ, hold, used, seed
								check("hold_piece", nil, in)
							}
						}
					}
				}
			}
			for typ := int64(0); typ < 7; typ++ {
				for rot := int64(0); rot < 4; rot++ {
					for pattern := 0; pattern < 3; pattern++ {
						for _, seed := range freeze.Seeds {
							in := tetrisFamilyCopy(state)
							in.Board = boardFor(pattern)
							in.Globals["cur_type"], in.Globals["cur_rot"], in.Globals["cur_y"], in.Globals["rng_state"], in.Globals["next_type"] = typ, rot, 16, seed, (typ+1)%7
							check("do_lock_and_clear", nil, in)
						}
					}
				}
			}
			// Actual VM/native RAM persists throughout each finite sequence.
			// Test controller applies only the explicitly frozen descent value.
			sequences := 0
			for typ := int64(0); typ < 7; typ++ {
				for _, seed := range freeze.Seeds {
					for pattern := 0; pattern < 2; pattern++ {
						in := tetrisFamilyCopy(state)
						in.Board = boardFor(pattern)
						in.Globals["next_type"], in.Globals["rng_state"] = typ, seed
						persistent = false
						in = check("spawn_piece", nil, in)
						persistent = true
						in = check("hold_piece", nil, in)
						in = check("hold_piece", nil, in)
						in = check("try_rotate", nil, in)
						gy, _, _ := tetrisSequenceModel("ghost_y", nil, in, geometry.Geometry[module])
						in = check("ghost_y", nil, in)
						in.Globals["cur_y"] = gy
						for _, vm := range vms {
							p, e := vm.ResolveSymbol("cur_y")
							if e != nil {
								t.Fatal(e)
							}
							vm.WriteHeapSlice(p, []byte{byte(gy)})
						}
						z.Memory()[a.Symbols["cur_y"]] = byte(gy)
						in = check("do_lock_and_clear", nil, in)
						in = check("hold_piece", nil, in)
						in = check("do_lock_and_clear", nil, in)
						persistent = false
						sequences++
					}
				}
			}

			save(prefix+"/summary.json", map[string]any{"calls": count, "by_entry": byEntry, "result": "PASS", "source_sha256": freeze.Sources["examples/nanz/"+module+".nanz"], "scope": "spawn/hold/composed plus 8entry persistent sequences, no device/main", "sequences": sequences})
			t.Log(module, mode, count, "fullstate orderedboard VM/native PASS")
		}
	}
}

func tetrisSequenceModel(entry string, args []int64, in tetrisFamilyState, geo [][][][]int) (int64, tetrisFamilyState, []tetrisFamilyEvent) {
	out := tetrisFamilyCopy(in)
	var events []tetrisFamilyEvent
	call := func(name string, args []int64) int64 {
		v, next, e := tetrisFamilyModel(name, args, out, geo)
		out = next
		events = append(events, e...)
		return v
	}
	spawn := func() {
		out.Globals["cur_type"] = out.Globals["next_type"]
		out.Globals["next_type"] = call("random_piece", nil)
		out.Globals["cur_rot"], out.Globals["cur_x"], out.Globals["cur_y"], out.Globals["hold_used"] = 0, 3, 0, 0
		if call("can_place", []int64{out.Globals["cur_type"], 0, 3, 0}) == 0 {
			out.Globals["game_over"] = 1
		}
	}
	switch entry {
	case "spawn_piece":
		spawn()
	case "hold_piece":
		if out.Globals["hold_used"] == 1 {
			return 0, out, nil
		}
		out.Globals["hold_used"] = 1
		if out.Globals["hold_type"] == 255 {
			out.Globals["hold_type"] = out.Globals["cur_type"]
			spawn()
		} else {
			out.Globals["cur_type"], out.Globals["hold_type"] = out.Globals["hold_type"], out.Globals["cur_type"]
			out.Globals["cur_rot"], out.Globals["cur_x"], out.Globals["cur_y"] = 0, 3, 0
		}
	case "do_lock_and_clear":
		call("lock_piece", nil)
		cl := call("clear_lines", nil)
		call("score_lines", []int64{cl})
		spawn()
	default:
		return tetrisFamilyModel(entry, args, in, geo)
	}
	return 0, out, events
}

func TestTetrisSequenceIndependentModelControls(t *testing.T) {
	geo := [][][][]int{{{{0, 0}, {1, 0}, {2, 0}, {3, 0}}}}
	in := tetrisFamilyState{make([]byte, 200), map[string]int64{"cur_type": 0, "next_type": 0, "hold_type": 255, "hold_used": 0, "cur_rot": 3, "cur_x": 8, "cur_y": 19, "rng_state": 1, "game_over": 0}}
	_, out, _ := tetrisSequenceModel("hold_piece", nil, in, geo)
	if out.Globals["hold_type"] != 0 || out.Globals["hold_used"] != 0 || out.Globals["rng_state"] != 0xb400 || out.Globals["cur_x"] != 3 {
		t.Fatal("empty hold original spawn/reset", out)
	}
	in.Globals["hold_used"] = 1
	_, out, _ = tetrisSequenceModel("hold_piece", nil, in, geo)
	if fmt.Sprint(out) != fmt.Sprint(in) {
		t.Fatal("used hold must be unchanged", out, in)
	}
	in.Globals["hold_used"] = 0
	in.Globals["hold_type"] = 0
	in.Board[3] = 7
	_, out, events := tetrisSequenceModel("hold_piece", nil, in, geo)
	if out.Globals["rng_state"] != 1 || out.Globals["hold_used"] != 1 || out.Globals["game_over"] != 0 || len(events) != 0 {
		t.Fatal("nonempty hold original no RNG/collision", out, events)
	}
	_, out, _ = tetrisSequenceModel("spawn_piece", nil, in, geo)
	if out.Globals["game_over"] != 1 {
		t.Fatal("blocked spawn", out)
	}
}
