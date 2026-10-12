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

type tetrisFamilyState struct {
	Board   []byte
	Globals map[string]int64
}
type tetrisFamilyEvent struct {
	Op     mir2.Op
	Offset int
	Value  byte
}

func tetrisFamilyCopy(in tetrisFamilyState) tetrisFamilyState {
	out := tetrisFamilyState{append([]byte(nil), in.Board...), map[string]int64{}}
	for k, v := range in.Globals {
		out.Globals[k] = v
	}
	return out
}
func tetrisFamilyModel(entry string, args []int64, in tetrisFamilyState, geo [][][][]int) (int64, tetrisFamilyState, []tetrisFamilyEvent) {
	out := tetrisFamilyCopy(in)
	var events []tetrisFamilyEvent
	read := func(x, y int) byte {
		i := y*10 + x
		v := out.Board[i]
		events = append(events, tetrisFamilyEvent{mir2.OpLoad, i, v})
		return v
	}
	write := func(x, y int, v byte) {
		i := y*10 + x
		events = append(events, tetrisFamilyEvent{mir2.OpStore, i, v})
		out.Board[i] = v
	}
	place := func(typ, rot, x, y int) bool {
		for _, xy := range geo[typ][rot] {
			cx, cy := int(byte(x+xy[0])), int(byte(y+xy[1]))
			if cx >= 10 || cy >= 20 {
				return false
			}
			if read(cx, cy) != 0 {
				return false
			}
		}
		return true
	}
	switch entry {
	case "piece_dx", "piece_dy":
		axis := 0
		if entry == "piece_dy" {
			axis = 1
		}
		return int64(geo[args[0]][args[1]][args[2]][axis]), out, nil
	case "board_get":
		return int64(read(int(args[0]), int(args[1]))), out, events
	case "board_set":
		write(int(args[0]), int(args[1]), byte(args[2]))
	case "rng_next", "random_piece":
		s := uint16(out.Globals["rng_state"])
		n := s >> 1
		if s&1 != 0 {
			n ^= 0xb400
		}
		out.Globals["rng_state"] = int64(n)
		v := byte(n)
		if entry == "random_piece" {
			v &= 7
			if v == 7 {
				v = 0
			}
		}
		return int64(v), out, nil
	case "score_lines":
		cl := byte(args[0])
		if cl != 0 {
			inc := map[byte]uint16{1: 100, 2: 300, 3: 500, 4: 800}[cl]
			out.Globals["score"] = int64(uint16(out.Globals["score"]) + inc)
			out.Globals["lines_cleared"] = int64(uint16(out.Globals["lines_cleared"]) + uint16(cl))
			if out.Globals["drop_speed"] > 2 && byte(out.Globals["lines_cleared"])&7 == 0 {
				out.Globals["drop_speed"]--
			}
		}
	case "can_place":
		if place(int(args[0]), int(args[1]), int(args[2]), int(args[3])) {
			return 1, out, events
		}
		return 0, out, events
	case "lock_piece":
		typ, rot := int(out.Globals["cur_type"]), int(out.Globals["cur_rot"])
		for _, xy := range geo[typ][rot] {
			x, y := int(byte(int(out.Globals["cur_x"])+xy[0])), int(byte(int(out.Globals["cur_y"])+xy[1]))
			if x < 10 && y < 20 {
				write(x, y, byte(typ+1))
			}
		}
	case "ghost_y":
		typ, rot, x := int(out.Globals["cur_type"]), int(out.Globals["cur_rot"]), int(out.Globals["cur_x"])
		y := byte(out.Globals["cur_y"])
		for place(typ, rot, x, int(y+1)) {
			y++
		}
		return int64(y), out, events
	case "try_rotate":
		typ, rot, x, y := int(out.Globals["cur_type"]), int((out.Globals["cur_rot"]+1)&3), int(out.Globals["cur_x"]), int(out.Globals["cur_y"])
		positions := [][2]int{{x, y}}
		if x < 9 {
			positions = append(positions, [2]int{x + 1, y})
		}
		if x > 0 {
			positions = append(positions, [2]int{x - 1, y})
		}
		if y > 0 {
			positions = append(positions, [2]int{x, y - 1})
		}
		for _, p := range positions {
			if place(typ, rot, p[0], p[1]) {
				out.Globals["cur_x"], out.Globals["cur_y"], out.Globals["cur_rot"] = int64(p[0]), int64(p[1]), int64(rot)
				return 1, out, events
			}
		}
		return 0, out, events
	case "clear_lines":
		count := 0
		for y := 19; y >= 0; {
			full := true
			for x := 0; x < 10; x++ {
				if read(x, y) == 0 {
					full = false
				}
			}
			if !full {
				y--
				continue
			}
			for sy := y; sy > 0; sy-- {
				for x := 0; x < 10; x++ {
					write(x, sy, read(x, sy-1))
				}
			}
			for x := 0; x < 10; x++ {
				write(x, 0, 0)
			}
			count++
		}
		return int64(count), out, events
	default:
		panic(entry)
	}
	return 0, out, events
}

func TestOriginalTetrisFamilyMechanics(t *testing.T) {
	tetrisVerifyHistoricalModel(t)
	data, e := os.ReadFile("../../../reports/tetris-family-native/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	var freeze struct {
		Sources  map[string]string      `json:"sources"`
		Geometry map[string][][][][]int `json:"geometry"`
	}
	if e = json.Unmarshal(data, &freeze); e != nil {
		t.Fatal(e)
	}
	for p, h := range freeze.Sources {
		path := "../../../" + p
		if p == "examples/nanz/tetris_cpm.nanz" {
			path = "../../../reports/tetris-family-native/tetris_cpm.nanz"
		}
		if p == "examples/nanz/tetris_tui.nanz" {
			path = "../../../reports/tetris-family-native/tetris_tui.nanz"
		}
		b, e := os.ReadFile(path)
		if e != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != h {
			t.Fatal("freeze", p, e)
		}
	}
	save := func(name string, v any) {
		dir := os.Getenv("MINZ_TETRIS_FAMILY_ARTIFACTS")
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
		src, e := os.ReadFile("../../../reports/tetris-family-native/" + module + ".nanz")
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
			for _, name := range []string{"rng_next", "random_piece", "piece_dx", "piece_dy", "board_get", "board_set", "can_place", "lock_piece", "clear_lines", "ghost_y", "try_rotate", "score_lines"} {
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
			check := func(entry string, args []int64, in tetrisFamilyState) {
				want, out, expected := tetrisFamilyModel(entry, args, in, freeze.Geometry[module])
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
					vm.WriteHeapSlice(0, before)
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
				copy(z.Memory(), base)
				apply(z.Memory(), in, func(n string) int { return a.Symbols[n] })
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
			}
			state := tetrisFamilyState{make([]byte, 200), map[string]int64{"rng_state": 0, "score": 0, "lines_cleared": 0, "drop_speed": 8, "cur_type": 0, "cur_rot": 0, "cur_x": 3, "cur_y": 0}}
			for _, pattern := range []int{20, 23, 21, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 22} {
				state.Board = make([]byte, 200)
				for y := 0; y < 20; y++ {
					for x := 0; x < 10; x++ {
						if pattern == 23 || pattern == y || pattern == 20 && y >= 16 || pattern == 21 && y%2 == 0 || pattern == 22 && y == 0 || x != y%10 {
							state.Board[y*10+x] = byte(1 + (x+3*y)%7)
						}
					}
				}
				check("clear_lines", nil, state)
			}
			for seed := 0; seed < 65536; seed++ {
				state.Globals["rng_state"] = int64(seed)
				check("rng_next", nil, state)
				check("random_piece", nil, state)
			}
			for _, tuple := range [][3]int64{{0, 0, 8}, {65535, 65535, 8}, {65500, 7, 3}, {32767, 255, 2}, {100, 256, 1}, {65400, 65528, 255}, {0, 8, 0}, {65535, 65534, 3}} {
				state.Globals["score"], state.Globals["lines_cleared"], state.Globals["drop_speed"] = tuple[0], tuple[1], tuple[2]
				for cl := int64(0); cl < 256; cl++ {
					check("score_lines", []int64{cl}, state)
				}
			}
			for y := int64(0); y < 20; y++ {
				for x := int64(0); x < 10; x++ {
					for v := int64(0); v < 256; v++ {
						check("board_set", []int64{x, y, v}, state)
						state.Board[y*10+x] = byte(v)
						check("board_get", []int64{x, y}, state)
					}
				}
			}
			for typ := 0; typ < 7; typ++ {
				for rot := 0; rot < 4; rot++ {
					for c := 0; c < 4; c++ {
						check("piece_dx", []int64{int64(typ), int64(rot), int64(c)}, state)
						check("piece_dy", []int64{int64(typ), int64(rot), int64(c)}, state)
					}
					for _, pos := range [][2]int{{0, 0}, {3, 0}, {8, 0}, {9, 18}, {0, 19}, {255, 0}, {0, 255}, {254, 254}} {
						for pattern := 0; pattern < 3; pattern++ {
							state.Board = make([]byte, 200)
							if pattern == 1 {
								for i := 0; i < 200; i++ {
									if i%13 == 0 {
										state.Board[i] = byte(i%7 + 1)
									}
								}
							}
							if pattern == 2 {
								for i := 190; i < 200; i++ {
									state.Board[i] = 7
								}
							}
							state.Globals["cur_type"], state.Globals["cur_rot"], state.Globals["cur_x"], state.Globals["cur_y"] = int64(typ), int64(rot), int64(pos[0]), int64(pos[1])
							check("can_place", []int64{int64(typ), int64(rot), int64(pos[0]), int64(pos[1])}, state)
							check("lock_piece", nil, state)
							check("try_rotate", nil, state)
							check("ghost_y", nil, state)
						}
					}
				}
			}
			save(prefix+"/summary.json", map[string]any{"calls": count, "by_entry": byEntry, "result": "PASS", "source_sha256": freeze.Sources["examples/nanz/"+module+".nanz"], "scope": "12 computational entries, full module retained; no terminal/main execution"})
			t.Log(module, mode, count, "fullstate orderedboard VM/native PASS")
		}
	}
}

func TestTetrisFamilyIndependentModelControls(t *testing.T) {
	geo := [][][][]int{{{{0, 0}, {1, 0}, {2, 0}, {3, 0}}}}
	state := tetrisFamilyState{make([]byte, 200), map[string]int64{"cur_type": 0, "cur_rot": 0, "cur_x": 8, "cur_y": 19, "score": 65500, "lines_cleared": 65535, "drop_speed": 3}}
	got, after, events := tetrisFamilyModel("lock_piece", nil, state, geo)
	if got != 0 || after.Board[198] != 1 || after.Board[199] != 1 || len(events) != 2 || events[0].Offset != 198 || events[1].Offset != 199 {
		t.Fatal("edge clipping", got, after, events)
	}
	got, _, events = tetrisFamilyModel("can_place", []int64{0, 0, 255, 0}, state, geo)
	if got != 0 || len(events) != 0 {
		t.Fatal("first boundary exits before board read", got, events)
	}
	_, after, _ = tetrisFamilyModel("score_lines", []int64{1}, state, geo)
	if after.Globals["score"] != 64 || after.Globals["lines_cleared"] != 0 || after.Globals["drop_speed"] != 2 {
		t.Fatal("independent modular scoring", after)
	}
	state.Board = make([]byte, 200)
	for i := 190; i < 200; i++ {
		state.Board[i] = 7
	}
	state.Board[183] = 5
	got, after, _ = tetrisFamilyModel("clear_lines", nil, state, geo)
	if got != 1 || after.Board[193] != 5 || after.Board[183] != 0 {
		t.Fatal("independent row compaction", got, after)
	}
}
