package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Independent row masks for all seven tetrominoes, including their rotations.
// This reference never reads PIECE_DX/DY or invokes Nanz can_place.
func TestTetrisGhostLandingNativeVM(t *testing.T) {
	descriptions := [][4]string{
		{"####", "#/#/#/#", "####", "#/#/#/#"},
		{"##/##", "##/##", "##/##", "##/##"},
		{".#./###", "#./##/#.", "###/.#.", ".#/##/.#"},
		{".##/##.", "#./##/.#", ".##/##.", "#./##/.#"},
		{"##./.##", ".#/##/#.", "##./.##", ".#/##/#."},
		{"..#/###", "#./#./##", "###/#..", "##/.#/.#"},
		{"#../###", "##/#./#.", "###/..#", ".#/.#/##"},
	}
	type cell struct{ x, y int }
	var shapes [7][4][]cell
	for kind, rs := range descriptions {
		for rot, s := range rs {
			for y, row := range strings.Split(s, "/") {
				for x, ch := range row {
					if ch == '#' {
						shapes[kind][rot] = append(shapes[kind][rot], cell{x, y})
					}
				}
			}
			if len(shapes[kind][rot]) != 4 {
				t.Fatal("reference geometry")
			}
		}
	}
	src, e := os.ReadFile("../../../examples/zx/tetris_playable.nanz")
	if e != nil {
		t.Fatal(e)
	}
	source := string(src) + "\nglobal ghost_answer:u8=0\nfun ghost_probe() { ghost_answer=ghost_y() }\n"
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.ParseWithOpts(source, "ghost-landing.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			steps, e := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors)
			}
			v := mir2.NewVM(steps.MIR2Module)
			v.EnsureHeap(65536)
			z := emulator.NewRemogattoZ80()
			if err := z.LoadMemory(0x8000, a.Binary); err != nil {
				t.Fatal(err)
			}
			mem := z.Memory()
			set := func(n string, b byte) {
				addr, ok := a.Symbols[n]
				if !ok {
					t.Fatal("symbol", n)
				}
				mem[addr] = b
				v.WriteHeapSlice(v.GlobalAddr(n), []byte{b})
			}
			checked := 0
			for pattern := 0; pattern < 4; pattern++ {
				board := make([]byte, 200)
				for y := 4; y < 20; y++ {
					for x := 0; x < 10; x++ {
						occupied := false
						switch pattern {
						case 0:
							occupied = y == 19
						case 1:
							occupied = y >= 14+x%3
						case 2:
							occupied = (y == 10 && x >= 2 && x <= 6) || (y == 18 && x%3 != 0)
						case 3:
							occupied = (x*13+y*7)%17 == 0
						}
						if occupied {
							board[y*10+x] = byte((x+y)%7 + 1)
						}
					}
				}
				copy(mem[a.Symbols["board"]:], board)
				v.WriteHeapSlice(v.GlobalAddr("board"), board)
				for kind := 0; kind < 7; kind++ {
					for rot := 0; rot < 4; rot++ {
						for x := 0; x < 10; x++ {
							for _, y := range []int{0, 3, 8, 12, 16} {
								valid := func(row int) bool {
									for _, c := range shapes[kind][rot] {
										cx, cy := x+c.x, row+c.y
										if cx >= 10 || cy >= 20 || board[cy*10+cx] != 0 {
											return false
										}
									}
									return true
								}
								if !valid(y) {
									continue
								}
								want := y
								for valid(want + 1) {
									want++
								}
								set("cur_type", byte(kind))
								set("cur_rot", byte(rot))
								set("cur_x", byte(x))
								set("cur_y", byte(y))
								set("ghost_answer", 255)
								if _, e := v.Call("ghost_probe", nil); e != nil {
									t.Fatal(e)
								}
								if got := v.ReadHeap(v.GlobalAddr("ghost_answer"), 1)[0]; got != byte(want) {
									t.Fatalf("VM %s got %d want %d", fmt.Sprint(pattern, kind, rot, x, y), got, want)
								}
								mem[0xeff0], mem[0xeff1] = 0, 0x70
								z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["ghost_probe"]), SP: 0xeff0})
								returned := false
								for i := 0; i < 50000; i++ {
									if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
										returned = true
										break
									}
									z.Step()
								}
								if !returned {
									t.Fatal("ghost PC/SP")
								}
								if got := mem[a.Symbols["ghost_answer"]]; got != byte(want) {
									t.Fatalf("Z80 %s got %d want %d", fmt.Sprint(pattern, kind, rot, x, y), got, want)
								}
								for i, b := range board {
									if mem[a.Symbols["board"]+i] != b || v.ReadHeap(v.GlobalAddr("board")+int64(i), 1)[0] != b {
										t.Fatal("ghost changed board")
									}
								}
								checked++
							}
						}
					}
				}
			}
			if d := os.Getenv("MINZ_TETRIS_GHOST_EXPORT"); d != "" {
				dir := filepath.Join(d, mode)
				if e := os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				receipt, _ := json.MarshalIndent(map[string]any{"PASS": true, "mode": mode, "poses": checked, "backends": []string{"MIR2 VM", "native Z80"}, "shapes": 7, "rotations": 4, "board_patterns": 4, "start_heights": []int{0, 3, 8, 12, 16}, "binary_bytes": len(a.Binary), "return_PC_SP": true, "board_unchanged": true}, "", "  ")
				for name, data := range map[string][]byte{"source.nanz": []byte(source), "program.a80": []byte("ORG 0x8000\n" + steps.Assembly), "program.bin": a.Binary, "result.json": receipt} {
					if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			t.Logf("%d landing poses × VM/native, 7 shapes ×4 rotations, floor/stairs/overhangs/scattered cells", checked)
		})
	}
}
