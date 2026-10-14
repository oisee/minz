package pipeline

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Geometry is independently drawn, not read from the program's PIECE_DX/DY.
// Pixel ownership is derived from the six occupied scanlines in each 8x8 cell.
// Nanz integer widths and counter loops: frozen Language Book v7, chapters 2/6.
// This model uses row filtering for line deletion, not the source's shifting loop.
type pixelGameCell struct{ x, y int }

var pixelGameDrawings = [7][4]string{
	{"####", "#/#/#/#", "####", "#/#/#/#"},
	{"##/##", "##/##", "##/##", "##/##"},
	{".#./###", "#./##/#.", "###/.#.", ".#/##/.#"},
	{".##/##.", "#./##/.#", ".##/##.", "#./##/.#"},
	{"##./.##", ".#/##/#.", "##./.##", ".#/##/#."},
	{"..#/###", "#./#./##", "###/#..", "##/.#/.#"},
	{"#../###", "##/#./#.", "###/..#", ".#/.#/##"},
}

func pixelGameCells(kind, rot int) []pixelGameCell {
	var out []pixelGameCell
	for y, row := range strings.Split(pixelGameDrawings[kind][rot], "/") {
		for x, ch := range row {
			if ch == '#' {
				out = append(out, pixelGameCell{x, y})
			}
		}
	}
	return out
}

type pixelGameModel struct {
	v                  map[string]int
	board              [200]byte
	fixedDifficulty    bool
	rowFirst, rowLimit int
}

func newPixelGameModel() pixelGameModel {
	return pixelGameModel{rowFirst: 20, v: map[string]int{"cur_type": 2, "cur_rot": 0, "cur_x": 3, "cur_y": 5, "pixel_phase": 0,
		"drop_timer": 0, "drop_speed": 30, "frame_count": 0, "score": 0, "lines_cleared": 0, "game_over": 0,
		"prev_keys": 0, "rng_state": 0xace1, "next_type": 1, "hold_type": 255, "hold_used": 0, "last_was_tspin": 0, "board_dirty": 0}}
}
func (m pixelGameModel) clone() pixelGameModel {
	v := map[string]int{}
	for n, x := range m.v {
		v[n] = x
	}
	m.v = v
	return m
}
func (m *pixelGameModel) occupied(x, y int) bool {
	return x < 0 || x >= 10 || y < 0 || y >= 20 || m.board[y*10+x] != 0
}
func (m *pixelGameModel) fits(kind, rot, x, y, phase int) bool {
	for _, c := range pixelGameCells(kind, rot) {
		// Require logical ownership and every physical body pixel to be free.
		if m.occupied(x+c.x, y+c.y) {
			return false
		}
		for row := 1; row <= 6; row++ {
			if m.occupied(x+c.x, (8*(y+c.y)+phase+row)/8) {
				return false
			}
		}
	}
	return true
}
func (m *pixelGameModel) currentFits(x, y, phase int) bool {
	return m.fits(m.v["cur_type"], m.v["cur_rot"], x, y, phase)
}
func (m *pixelGameModel) spawn() {
	v := m.v
	v["cur_type"] = v["next_type"]
	r := v["rng_state"]
	bit := r & 1
	r >>= 1
	if bit != 0 {
		r ^= 0xb400
	}
	v["rng_state"] = r
	v["next_type"] = r & 7
	if v["next_type"] == 7 {
		v["next_type"] = 0
	}
	v["cur_rot"] = 0
	v["cur_x"] = 3
	v["cur_y"] = 0
	v["pixel_phase"] = 0
	v["hold_used"] = 0
	v["last_was_tspin"] = 0
	if !m.currentFits(3, 0, 0) {
		v["game_over"] = 1
	}
}
func (m *pixelGameModel) lock() {
	v := m.v
	tspin := false
	if v["cur_type"] == 2 {
		corners := 0
		cx, cy := v["cur_x"]+1, v["cur_y"]
		for _, d := range []pixelGameCell{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			if m.occupied(cx+d.x, cy+d.y) {
				corners++
			}
		}
		tspin = corners >= 3
	}
	for _, c := range pixelGameCells(v["cur_type"], v["cur_rot"]) {
		y := v["cur_y"] + c.y
		if y < m.rowFirst {
			m.rowFirst = y
		}
		if y+1 > m.rowLimit {
			m.rowLimit = y + 1
		}
		m.board[y*10+v["cur_x"]+c.x] = byte(v["cur_type"] + 1)
	}
	v["board_dirty"] = 1
	var rows [][10]byte
	cleared := 0
	for y := 0; y < 20; y++ {
		var row [10]byte
		copy(row[:], m.board[y*10:(y+1)*10])
		full := true
		for _, b := range row {
			if b == 0 {
				full = false
			}
		}
		if full {
			m.rowFirst = 0
			if y+1 > m.rowLimit {
				m.rowLimit = y + 1
			}
			cleared++
		} else {
			rows = append(rows, row)
		}
	}
	clear(m.board[:])
	for y, row := range rows {
		copy(m.board[(cleared+y)*10:], row[:])
	}
	bonus := 0
	if tspin {
		if cleared <= 3 {
			bonus = (cleared + 1) * 400
		}
	} else {
		bonus = map[int]int{1: 100, 2: 300, 3: 500, 4: 800}[cleared]
	}
	v["score"] = (v["score"] + bonus) & 65535
	if cleared > 0 {
		v["lines_cleared"] = (v["lines_cleared"] + cleared) & 65535
		if !m.fixedDifficulty && v["drop_speed"] > 5 && v["lines_cleared"]&7 == 0 {
			v["drop_speed"] -= 2
		}
	}
	m.spawn()
	v["drop_timer"] = 0
}
func (m *pixelGameModel) step(keys, ticks int) {
	v := m.v
	if v["game_over"] != 0 {
		return
	}
	v["frame_count"] = (v["frame_count"] + ticks) & 65535
	pressed := keys & (keys ^ v["prev_keys"])
	v["prev_keys"] = keys
	for _, move := range []struct{ bit, dx int }{{1, -1}, {2, 1}} {
		if pressed&move.bit != 0 && m.currentFits(v["cur_x"]+move.dx, v["cur_y"], v["pixel_phase"]) {
			v["cur_x"] += move.dx
			v["last_was_tspin"] = 0
		}
	}
	if pressed&4 != 0 {
		rot := (v["cur_rot"] + 1) & 3
		kicks := []pixelGameCell{{0, 0}, {1, 0}, {-1, 0}, {0, -1}, {1, -1}, {-1, -1}}
		if v["cur_type"] == 0 {
			kicks = append(kicks, pixelGameCell{2, 0}, pixelGameCell{-2, 0})
		}
		for _, d := range kicks {
			x, y := v["cur_x"]+d.x, v["cur_y"]+d.y
			if m.fits(v["cur_type"], rot, x, y, v["pixel_phase"]) {
				v["cur_x"] = x
				v["cur_y"] = y
				v["cur_rot"] = rot
				break
			}
		}
	}
	if pressed&16 != 0 {
		for m.currentFits(v["cur_x"], v["cur_y"]+1, 0) {
			v["cur_y"]++
			v["score"] = (v["score"] + 2) & 65535
		}
		m.lock()
		return
	}
	if pressed&32 != 0 && v["hold_used"] == 0 {
		old := v["cur_type"]
		if v["hold_type"] == 255 {
			v["hold_type"] = old
			m.spawn()
		} else {
			v["cur_type"] = v["hold_type"]
			v["hold_type"] = old
			v["cur_rot"] = 0
			v["cur_x"] = 3
			v["cur_y"] = 0
			v["pixel_phase"] = 0
			if !m.currentFits(3, 0, 0) {
				v["game_over"] = 1
			}
		}
		v["hold_used"] = 1
		v["drop_timer"] = 0
		return
	}
	if v["drop_speed"] == 0 {
		v["drop_speed"] = 1
	}
	budget := v["drop_timer"] + ticks*8
	if keys&8 != 0 {
		budget += v["drop_speed"] * 8
	}
	for budget >= v["drop_speed"] {
		budget -= v["drop_speed"]
		if v["pixel_phase"] == 0 {
			v["pixel_phase"] = 1
			continue
		}
		if !m.currentFits(v["cur_x"], v["cur_y"]+1, 0) {
			m.lock()
			return
		}
		v["pixel_phase"]++
		if v["pixel_phase"] == 8 {
			v["pixel_phase"] = 0
			v["cur_y"]++
		}
	}
	v["drop_timer"] = budget
}

func TestTetrisPixelPhysicsNativeVM(t *testing.T) {
	src, e := os.ReadFile("../../../examples/zx/tetris_playable.nanz")
	if e != nil {
		t.Fatal(e)
	}
	source := string(src) + "\nglobal physics_keys:u8=0\nglobal physics_ticks:u8=0\nfun physics_probe(){game_step(physics_keys,physics_ticks)}\n"
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(source, "pixel-physics.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	type trial struct {
		name        string
		initial     pixelGameModel
		keys, ticks int
	}
	var trials []trial
	add := func(name string, m pixelGameModel, keys, ticks int) {
		trials = append(trials, trial{name, m.clone(), keys, ticks})
	}
	for kind := 0; kind < 7; kind++ {
		for rot := 0; rot < 4; rot++ {
			for phase := 0; phase < 8; phase++ {
				m := newPixelGameModel()
				m.v["cur_type"] = kind
				m.v["cur_rot"] = rot
				m.v["pixel_phase"] = phase
				for _, keys := range []int{0, 1, 2, 4, 8, 16, 32} {
					add(fmt.Sprintf("shape%d/rot%d/phase%d/keys%d", kind, rot, phase, keys), m, keys, 4)
				}
			}
		}
	}
	for phase := 0; phase < 8; phase++ {
		for _, x := range []int{0, 8} {
			for _, keys := range []int{1, 2, 4} {
				m := newPixelGameModel()
				m.v["cur_type"] = 1
				m.v["cur_x"] = x
				m.v["pixel_phase"] = phase
				m.board[7*10+1] = 7
				m.board[7*10+8] = 7
				if m.currentFits(x, 5, phase) {
					add(fmt.Sprintf("edge%d/phase%d/key%d", x, phase, keys), m, keys, 0)
				}
			}
		}
	}
	for _, phase := range []int{0, 1} {
		m := newPixelGameModel()
		m.v["cur_type"] = 1
		m.v["cur_y"] = 18
		m.v["pixel_phase"] = phase
		add(fmt.Sprintf("floor/phase%d", phase), m, 0, 4)
		for x := 0; x < 10; x++ {
			if x != 3 && x != 4 {
				m.board[190+x] = 7
			}
		}
		m.v["lines_cleared"] = 7
		m.v["score"] = 65500
		add(fmt.Sprintf("clear-and-speed/phase%d", phase), m, 0, 30)
	}
	m := newPixelGameModel()
	m.v["cur_type"] = 0
	m.v["cur_rot"] = 1
	m.v["cur_y"] = 16
	for y := 16; y < 20; y++ {
		for x := 0; x < 10; x++ {
			if x != 3 {
				m.board[y*10+x] = 6
			}
		}
	}
	add("four-lines", m, 16, 255)
	// Distinct row/cell colours make a backwards overlapping copy observable:
	// preserving only the number of cleared lines would miss row reordering.
	for seed := 0; seed < 32; seed++ {
		m := newPixelGameModel()
		for y := 0; y < 20; y++ {
			full := (y+seed*3)%5 == 0
			for x := 0; x < 10; x++ {
				if full || (x+y*3+seed)%4 != 0 {
					m.board[y*10+x] = byte(1 + (y*2+x+seed)%7)
				}
			}
		}
		// Keep the initial actor's four cells free; the rest remains a
		// deterministic mixture of full and partial rows throughout the board.
		for _, c := range pixelGameCells(m.v["cur_type"], m.v["cur_rot"]) {
			m.board[(m.v["cur_y"]+c.y)*10+m.v["cur_x"]+c.x] = 0
		}
		add(fmt.Sprintf("coloured-row-order/%d", seed), m, 16, 0)
	}
	m = newPixelGameModel()
	for i := range m.board {
		m.board[i] = byte(1 + i%7)
	}
	add("clear-all-twenty-rows", m, 16, 0)
	for _, hold := range []int{0, 6, 255} {
		m := newPixelGameModel()
		m.v["hold_type"] = hold
		m.v["pixel_phase"] = 7
		add(fmt.Sprintf("hold%d", hold), m, 32, 255)
		m.v["hold_used"] = 1
		add(fmt.Sprintf("hold%d/reused", hold), m, 32, 4)
	}
	m = newPixelGameModel()
	m.v["frame_count"] = 65534
	m.v["drop_speed"] = 0
	add("clock-wrap/zero-speed", m, 0, 255)
	m = newPixelGameModel()
	m.v["score"] = 65535
	add("hard-drop-score-wrap", m, 16, 0)
	m = newPixelGameModel()
	m.v["prev_keys"] = 3
	add("held-keys-no-repeat", m, 3, 0)
	m = newPixelGameModel()
	m.v["game_over"] = 1
	add("game-over-frozen", m, 63, 255)
	m = newPixelGameModel()
	for x := 0; x < 10; x++ {
		m.board[x] = 7
	}
	add("blocked-spawn/hold", m, 32, 0)
	add("blocked-spawn/lock", m, 16, 0)
	raw := hir.LowerModule(parse())
	for _, mode := range []string{"ordinary", "grace-speed", "grace-size"} {
		t.Run(mode, func(t *testing.T) {
			steps, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x6000\n" + steps.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			if 0x6000+len(a.Binary) >= 0xe000 {
				t.Fatalf("module overlaps shadow: %d bytes", len(a.Binary))
			}
			// Only reachable compiler spill objects and 128 bytes below the caller's
			// stack may differ outside the independently predicted game state.
			reachable := map[string]bool{}
			var visit func(string)
			visit = func(name string) {
				if reachable[name] {
					return
				}
				reachable[name] = true
				if f := steps.MIR2Module.FuncByName(name); f != nil {
					for _, b := range f.Blocks {
						for _, in := range b.Insts {
							if in.Op == mir2.OpCall || in.Op == mir2.OpCallCond {
								visit(in.Sym)
							}
						}
					}
				}
			}
			visit("physics_probe")
			allowed := map[int]bool{}
			for _, line := range strings.Split(steps.Assembly, "\n") {
				fields := strings.Fields(line)
				if len(fields) < 3 || !strings.HasPrefix(fields[0], "_spill_") {
					continue
				}
				name := strings.TrimSuffix(fields[0], ":")
				owned := false
				for owner := range reachable {
					if strings.HasPrefix(name, "_spill_"+owner+"_r") {
						owned = true
						break
					}
				}
				if !owned {
					continue
				}
				size := 1
				if fields[1] == "DW" {
					size = 2
				} else if fields[1] != "DB" {
					t.Fatal("spill declaration", line)
				}
				size *= len(strings.Split(strings.Join(fields[2:], ""), ","))
				for i := 0; i < size; i++ {
					allowed[a.Symbols[name]+i] = true
				}
			}
			for _, tc := range trials {
				want := tc.initial.clone()
				want.step(tc.keys, tc.ticks)
				platform := emulator.NewRemogattoZ80WithScreen()
				platform.SetROMEnd(0)
				z := platform.RemogattoZ80
				z.SetSMCTracker(nil)
				mem := z.Memory()
				for at := range mem {
					mem[at] = byte(at*37 + 13)
				}
				copy(mem[0x6000:], a.Binary)
				put := func(name string, value int) {
					addr, ok := a.Symbols[name]
					if !ok {
						t.Fatal(name)
					}
					mem[addr] = byte(value)
					if name == "score" || name == "lines_cleared" || name == "frame_count" || name == "rng_state" {
						mem[addr+1] = byte(value >> 8)
					}
				}
				for name, value := range tc.initial.v {
					put(name, value)
				}
				put("board_first", tc.initial.rowFirst)
				put("board_limit", tc.initial.rowLimit)
				put("physics_keys", tc.keys)
				put("physics_ticks", tc.ticks)
				copy(mem[a.Symbols["board"]:], tc.initial.board[:])
				const sp = 0xfce0
				const ret = 0x5e00
				mem[sp] = byte(ret & 255)
				mem[sp+1] = byte(ret >> 8)
				expectedRAM := append([]byte(nil), mem...)
				for name, value := range want.v {
					at := a.Symbols[name]
					expectedRAM[at] = byte(value)
					if name == "score" || name == "lines_cleared" || name == "frame_count" || name == "rng_state" {
						expectedRAM[at+1] = byte(value >> 8)
					}
				}
				expectedRAM[a.Symbols["board_first"]] = byte(want.rowFirst)
				expectedRAM[a.Symbols["board_limit"]] = byte(want.rowLimit)
				copy(expectedRAM[a.Symbols["board"]:], want.board[:])
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["physics_probe"]), SP: sp})
				for i := 0; i < 500000 && (z.GetPC() != ret || z.GetSP() != sp+2); i++ {
					z.Step()
				}
				if z.GetPC() != ret || z.GetSP() != sp+2 {
					t.Fatalf("%s native PC/SP %04x/%04x", tc.name, z.GetPC(), z.GetSP())
				}
				check := func(label string, read func(string, int) []byte) {
					for name, value := range want.v {
						size := 1
						if name == "score" || name == "lines_cleared" || name == "frame_count" || name == "rng_state" {
							size = 2
						}
						bs := read(name, size)
						got := int(bs[0])
						if size == 2 {
							got |= int(bs[1]) << 8
						}
						if got != value {
							t.Fatalf("%s %s %s got %d want %d", tc.name, label, name, got, value)
						}
					}
					for name, w := range map[string]int{"board_first": want.rowFirst, "board_limit": want.rowLimit} {
						if got := int(read(name, 1)[0]); got != w {
							t.Fatalf("%s %s %s=%d want%d", tc.name, label, name, got, w)
						}
					}
					b := read("board", 200)
					for i, value := range want.board {
						if b[i] != value {
							t.Fatalf("%s %s board[%d]=%d want %d", tc.name, label, i, b[i], value)
						}
					}
				}
				check("Z80", func(n string, s int) []byte { return mem[a.Symbols[n] : a.Symbols[n]+s] })
				for at, b := range expectedRAM {
					if (at >= sp-128 && at < sp) || allowed[at] {
						continue
					}
					if mem[at] != b {
						t.Fatalf("%s native unexpected memory %04x: %02x want %02x", tc.name, at, mem[at], b)
					}
				}
				for _, mod := range []*mir2.Module{raw, steps.MIR2Module} {
					vm := mir2.NewVM(mod)
					vm.EnsureHeap(0x6000)
					for name, value := range tc.initial.v {
						bs := []byte{byte(value)}
						if name == "score" || name == "lines_cleared" || name == "frame_count" || name == "rng_state" {
							bs = append(bs, byte(value>>8))
						}
						vm.WriteHeapSlice(vm.GlobalAddr(name), bs)
					}
					vm.WriteHeap(vm.GlobalAddr("board_first"), byte(tc.initial.rowFirst))
					vm.WriteHeap(vm.GlobalAddr("board_limit"), byte(tc.initial.rowLimit))
					vm.WriteHeapSlice(vm.GlobalAddr("board"), tc.initial.board[:])
					vm.WriteHeap(vm.GlobalAddr("physics_keys"), byte(tc.keys))
					vm.WriteHeap(vm.GlobalAddr("physics_ticks"), byte(tc.ticks))
					if _, e := vm.Call("physics_probe", nil); e != nil {
						t.Fatalf("%s VM: %v", tc.name, e)
					}
					check("VM", func(n string, s int) []byte { return vm.ReadHeap(vm.GlobalAddr(n), s) })
				}
			}
			t.Logf("%d physics cases: native Z80 + raw/optimized VM, full board/state and PC/SP", len(trials))
		})
	}
}
