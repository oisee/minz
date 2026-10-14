package pipeline

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

type smoothGameObserver struct {
	*tetrisSceneObserver
	before func()
}

func (c *smoothGameObserver) DoOpcode() { c.before(); c.tetrisSceneObserver.DoOpcode() }

// Executes the frozen shadow-candidate main through its real loader, keyboard matrix,
// IM2 timer, renderer and game-over return. The independent physics model owns
// the expected state across successive calls; it is never reseeded from the game.
func TestTetrisShadowMainMachine(t *testing.T) {
	runTetrisMainMachine(t, "../../../reports/advanced-demos/tetris/shadow-candidate.nanz", true)
}

func TestTetrisDirectMainMachine(t *testing.T) {
	runTetrisMainMachine(t, "../../../examples/zx/tetris_playable.nanz", false)
}

func runTetrisMainMachine(t *testing.T, sourcePath string, shadow bool) {
	t.Helper()
	src, e := os.ReadFile(sourcePath)
	if e != nil {
		t.Fatal(e)
	}
	boot, e := os.ReadFile("../../../scripts/zx_tetris_entry.a80")
	if e != nil {
		t.Fatal(e)
	}
	rom, e := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if e != nil {
		t.Fatal(e)
	}
	stdlibDir := "../../../stdlib"
	if shadow {
		stdlibDir = historicalTetrisStdlib(t)
	}
	for _, mode := range []string{"ordinary", "grace-speed", "grace-size"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.ParseWithOpts(string(src), "tetris-smooth-main.nanz", nanz.ParseOpts{StdlibDir: stdlibDir})
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if e != nil {
				t.Fatal(e)
			}
			asm := string(boot) + "\n" + s.Assembly
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			limit := 0xfc60
			if shadow {
				limit = 0xe000
			}
			if 0x6000+len(a.Binary) >= limit {
				t.Fatalf("code overlaps reserved RAM: %d bytes", len(a.Binary))
			}
			sym := func(n string) uint16 {
				v, ok := a.Symbols[n]
				if !ok {
					t.Fatal(n)
				}
				return uint16(v)
			}
			m, e := spectrum.New48K(rom)
			if e != nil {
				t.Fatal(e)
			}
			for i, b := range a.Binary {
				m.Memory.Write(0x6000+uint16(i), b, false)
			}
			m.CPU.SetPC(0x6000)
			m.CPU.SetSP(0xfce0)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			model := newPixelGameModel()
			model.spawn()
			model.spawn()
			model.v["board_dirty"] = 0
			read := func(n string, size int) []byte {
				out := make([]byte, size)
				for i := range out {
					out[i] = m.Memory.ReadByteInternal(sym(n) + uint16(i))
				}
				return out
			}
			check := func(where string) {
				for n, want := range model.v {
					size := 1
					if n == "score" || n == "frame_count" || n == "lines_cleared" || n == "rng_state" {
						size = 2
					}
					b := read(n, size)
					got := int(b[0])
					if size == 2 {
						got |= int(b[1]) << 8
					}
					if got != want {
						t.Fatalf("%s %s got%d want%d", where, n, got, want)
					}
				}
				b := read("board", 200)
				for i, want := range model.board {
					if b[i] != want {
						t.Fatalf("%s board[%d]=%d want%d", where, i, b[i], want)
					}
				}
			}
			selected, steps, nonzeroPhases, locks := 0, 0, 0, 0
			tickHistogram := map[int]int{}
			colors := []byte{0, 5, 6, 3, 4, 2, 0x46, 1}
			checkImage := func() {
				var bitmap [160][10]byte
				var attributes [20][10]byte
				for y := 0; y < 20; y++ {
					for x := 0; x < 10; x++ {
						b := model.board[y*10+x]
						attributes[y][x] = colors[b]
						if b != 0 {
							for line := 0; line < 8; line++ {
								bitmap[y*8+line][x] = 255
							}
						}
					}
				}
				if model.v["game_over"] == 0 {
					for _, cell := range pixelGameCells(model.v["cur_type"], model.v["cur_rot"]) {
						x, y := model.v["cur_x"]+cell.x, model.v["cur_y"]+cell.y
						for row := 1; row <= 6; row++ {
							scan := y*8 + model.v["pixel_phase"] + row
							bitmap[scan][x] |= 0x7e
							attributes[scan/8][x] = colors[model.v["cur_type"]+1]
						}
					}
				}
				for y := 0; y < 160; y++ {
					for x := 0; x < 10; x++ {
						scan := y + 16
						at := uint16(0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + 11 + x)
						addresses := []uint16{at}
						if shadow {
							addresses = append(addresses, at+0xa000)
						}
						for _, address := range addresses {
							if got := m.Memory.ReadByteInternal(address); got != bitmap[y][x] {
								t.Fatalf("render%d bitmap %04x=%02x want%02x", steps, address, got, bitmap[y][x])
							}
						}
					}
				}
				for y := 0; y < 20; y++ {
					for x := 0; x < 10; x++ {
						at := uint16(0x5800 + (y+2)*32 + 11 + x)
						if got := m.Memory.ReadByteInternal(at); got != attributes[y][x] {
							t.Fatalf("render%d attr %04x=%02x want%02x", steps, at, got, attributes[y][x])
						}
					}
				}
				for _, preview := range []struct{ kind, x int }{{model.v["next_type"], 23}, {model.v["hold_type"], 5}} {
					var attrs [4][4]byte
					if preview.kind < 7 {
						for _, cell := range pixelGameCells(preview.kind, 0) {
							attrs[cell.y][cell.x] = colors[preview.kind+1]
						}
					}
					for y := 0; y < 4; y++ {
						for x := 0; x < 4; x++ {
							at := uint16(0x5800 + (y+3)*32 + preview.x + x)
							if got := m.Memory.ReadByteInternal(at); got != attrs[y][x] {
								t.Fatalf("render%d preview %04x=%02x want%02x", steps, at, got, attrs[y][x])
							}
						}
					}
				}
			}
			// ABI locations are inspected solely to read actual call arguments; their
			// values are checked against independently selected keys and clock deltas.
			arg := func(index int) int {
				f := s.MIR2Module.FuncByName("game_step")
				loc := s.Allocation.Locs[f.Contract.Params[index].Reg].Name
				switch strings.ToUpper(loc) {
				case "IXH":
					return int(m.CPU.IX() >> 8)
				case "IXL":
					return int(byte(m.CPU.IX()))
				case "IYH":
					return int(m.CPU.IY() >> 8)
				case "IYL":
					return int(byte(m.CPU.IY()))
				case "A":
					return int(m.CPU.AF() >> 8)
				case "B":
					return int(m.CPU.BC() >> 8)
				case "C":
					return int(byte(m.CPU.BC()))
				case "D":
					return int(m.CPU.DE() >> 8)
				case "E":
					return int(byte(m.CPU.DE()))
				case "H":
					return int(m.CPU.HL() >> 8)
				case "L":
					return int(byte(m.CPU.HL()))
				}
				t.Fatalf("unsupported byte argument location %s", loc)
				return 0
			}
			// Compare every fetched field frame with complete independently drawn
			// scenes encountered in that frame, never with a final-RAM screenshot.
			fieldImage := func() []byte {
				screen := make([]byte, 6912)
				for y := 0; y < 20; y++ {
					for x := 0; x < 10; x++ {
						b := model.board[y*10+x]
						screen[6144+(y+2)*32+x+11] = colors[b]
						if b != 0 {
							for line := 0; line < 8; line++ {
								scan := 16 + y*8 + line
								at := (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + x + 11
								screen[at] = 255
							}
						}
					}
				}
				if model.v["game_over"] == 0 {
					for _, cell := range pixelGameCells(model.v["cur_type"], model.v["cur_rot"]) {
						x, y := model.v["cur_x"]+cell.x, model.v["cur_y"]+cell.y
						for row := 1; row <= 6; row++ {
							scan := 16 + y*8 + model.v["pixel_phase"] + row
							at := (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + x + 11
							screen[at] |= 0x7e
							screen[6144+scan/8*32+x+11] = colors[model.v["cur_type"]+1]
						}
					}
				}
				full := shadowScreenPixels(screen)
				out := make([]byte, 80*160*4)
				for y := 0; y < 160; y++ {
					at := ((y+16)*256 + 88) * 4
					copy(out[y*80*4:], full[at:at+80*4])
				}
				return out
			}
			var shown, pending []byte
			var frameCandidates [][]byte
			fetchedFrames := 0
			commands := []int{0, 0, 0, 0, 1, 0, 2, 0, 4, 0, 32, 0, 32, 0, 8, 8, 8, 0, 4, 0}
			var stepReturn, stepSP, renderReturn, renderSP uint16
			stepActive, renderActive, clockReady := false, false, false
			previousClock := byte(0)
			base := &tetrisSceneObserver{CPUCore: m.CPU, t: t, m: m, minSP: 0xfce0, returnPC: sym("demo_done"), returnSP: 0xfce0, phaseMax: map[string]int64{}}
			c := &smoothGameObserver{tetrisSceneObserver: base}
			m.CPU = c
			c.before = func() {
				if renderActive && c.PC() == renderReturn && c.SP() == renderSP+2 {
					renderActive = false
					checkImage()
					shown = fieldImage()
					model.v["board_dirty"] = 0
					if !clockReady {
						previousClock = m.Memory.ReadByteInternal(0x5c78)
						clockReady = true
					}
				}
				if stepActive && c.PC() == stepReturn && c.SP() == stepSP+2 {
					stepActive = false
					check(fmt.Sprintf("step%d/return", steps))
					steps++
					if model.v["pixel_phase"] != 0 {
						nonzeroPhases++
					}
				}
				if c.PC() == sym("render_smooth") && !renderActive {
					renderActive = true
					pending = fieldImage()
					frameCandidates = append(frameCandidates, pending)
					renderSP = c.SP()
					renderReturn = uint16(m.Memory.ReadByteInternal(c.SP())) | uint16(m.Memory.ReadByteInternal(c.SP()+1))<<8
				}
				if c.PC() == sym("read_input") {
					m.Keyboard.Reset()
					selected = 0
					if steps < len(commands) {
						selected = commands[steps]
					} else if steps%2 == 0 {
						selected = 16
					}
					for _, key := range []struct{ bit, row, col int }{{1, 5, 1}, {2, 5, 0}, {4, 2, 0}, {8, 1, 0}, {16, 7, 0}, {32, 6, 4}} {
						if selected&key.bit != 0 {
							m.Keyboard.KeyPress(key.row, key.col)
						}
					}
				}
				if c.PC() == sym("game_step") && !stepActive {
					if !clockReady {
						t.Fatal("game_step before initial renderer returned")
					}
					model.v["board_dirty"] = 0
					check(fmt.Sprintf("step%d/entry", steps))
					now := read("clock_seen", 1)[0]
					ticks := int(byte(now - previousClock))
					tickHistogram[ticks]++
					previousClock = now
					if arg(0) != selected || arg(1) != ticks {
						t.Fatalf("step%d arguments keys%d/%d ticks%d/%d", steps, arg(0), selected, arg(1), ticks)
					}
					oldRng := model.v["rng_state"]
					model.step(selected, ticks)
					if model.v["rng_state"] != oldRng && selected != 32 {
						locks++
					}
					stepActive = true
					stepSP = c.SP()
					stepReturn = uint16(m.Memory.ReadByteInternal(c.SP())) | uint16(m.Memory.ReadByteInternal(c.SP()+1))<<8
				}
			}
			frames := 0
			for ; frames < 2000 && !c.done; frames++ {
				ready := shown != nil
				frameCandidates = nil
				if shown != nil {
					frameCandidates = append(frameCandidates, shown)
				}
				if renderActive {
					frameCandidates = append(frameCandidates, pending)
				}
				m.RunFrame()
				if ready {
					full := shadowFetchedPixels(m)
					got := make([]byte, 80*160*4)
					for y := 0; y < 160; y++ {
						at := ((y+16)*256 + 88) * 4
						copy(got[y*80*4:], full[at:at+80*4])
					}
					matched := false
					for _, image := range frameCandidates {
						if bytes.Equal(got, image) {
							matched = true
							break
						}
					}
					if !matched {
						t.Fatalf("main mixed field in fetched frame%d step%d", frames, steps)
					}
					fetchedFrames++
				}
			}
			if !c.done || model.v["game_over"] != 1 {
				t.Fatalf("main did not finish game-over: frames%d steps%d pc%04x", frames, steps, c.PC())
			}
			if steps < 20 || nonzeroPhases < 3 || locks < 3 || c.irq < 20 {
				t.Fatalf("insufficient gameplay coverage steps%d phases%d locks%d IRQ%d", steps, nonzeroPhases, locks, c.irq)
			}
			stackLimit := uint16(0xfc60)
			if shadow {
				stackLimit = 0xf860
			}
			if c.minSP < stackLimit {
				t.Fatalf("stack exceeded reserved region: %04x", c.minSP)
			}
			if root := os.Getenv("MINZ_TETRIS_MAIN_EXPORT"); root != "" {
				dir := filepath.Join(root, mode)
				if e := os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				receipt := fmt.Sprintf("PASS\nfetched_complete_field_frames=%d\nframes=%d\nsteps=%d\npixel_phase_samples=%d\nlocks=%d\ninterrupts=%d\nminimum_SP=0x%04x\nbytes=%d\n", fetchedFrames, frames, steps, nonzeroPhases, locks, c.irq, c.minSP, len(a.Binary))
				for name, data := range map[string][]byte{"source.nanz": src, "program.a80": []byte(asm), "program.bin": a.Binary, "receipt.txt": []byte(receipt)} {
					if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			t.Logf("%d frames, %d independently checked main steps, %d fractional poses, %d locks, %d IRQ; game-over returned with SP restored; clock-delta histogram %v", frames, steps, nonzeroPhases, locks, c.irq, tickHistogram)
		})
	}
}
