package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

type smoothGameObserver struct {
	*tetrisSceneObserver
	before  func()
	profile *directProfileObserver
}

func (c *smoothGameObserver) DoOpcode() {
	c.before()
	if c.profile != nil {
		c.profile.DoOpcode()
	} else {
		c.tetrisSceneObserver.DoOpcode()
	}
}

// Executes the frozen shadow-candidate main through its real loader, keyboard matrix,
// IM2 timer, renderer and game-over return. The independent physics model owns
// the expected state across successive calls; it is never reseeded from the game.
func TestTetrisShadowMainMachine(t *testing.T) {
	runTetrisMainMachine(t, "../../../reports/advanced-demos/tetris/shadow-candidate.nanz", true, 0)
}

func TestTetrisDirectMainMachine(t *testing.T) {
	for _, level := range []int{1, 2} {
		t.Run(fmt.Sprintf("level-%d", level), func(t *testing.T) { runTetrisMainMachine(t, "../../../examples/zx/tetris_playable.nanz", false, level) })
	}
}

// A seeded checkpoint enters the unchanged main before its first publication.
// Each scene has coloured rows and a vertical-I gap for an exact line count.
func TestTetrisDirectLineClearMachine(t *testing.T) {
	for _, rows := range []int{1, 2, 4} {
		t.Run(fmt.Sprintf("clear-%d", rows), func(t *testing.T) {
			runTetrisMainMachine(t, "../../../examples/zx/tetris_playable.nanz", false, 2, rows)
		})
	}
}

func runTetrisMainMachine(t *testing.T, sourcePath string, shadow bool, selectedLevel int, seededRows ...int) {
	t.Helper()
	clearRows := 0
	if len(seededRows) != 0 {
		clearRows = seededRows[0]
	}
	if clearRows < 0 || clearRows > 4 || (shadow && clearRows != 0) {
		t.Fatal("invalid seeded line-clear scene")
	}
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
			origin := 0x8000
			if shadow {
				origin = 0x6000
			}
			loader := strings.Replace(string(boot), "ORG $6000", fmt.Sprintf("ORG $%04X", origin), 1)
			asm := loader + "\n" + s.Assembly
			a, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			limit := 0xfc60
			if shadow {
				limit = 0xe000
			}
			if origin+len(a.Binary) >= limit {
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
				m.Memory.Write(uint16(origin)+uint16(i), b, false)
			}
			m.CPU.SetPC(uint16(origin))
			m.CPU.SetSP(0xfce0)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			model := newPixelGameModel()
			if selectedLevel != 0 {
				model.fixedDifficulty = true
				model.v["drop_speed"] = 16 / selectedLevel
			}
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
				if !shadow {
					for name, w := range map[string]int{"board_first": model.rowFirst, "board_limit": model.rowLimit} {
						if got := int(read(name, 1)[0]); got != w {
							t.Fatalf("%s %s=%d want%d", where, name, got, w)
						}
					}
				}
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
			quietSteps := 0
			if selectedLevel != 0 && clearRows == 0 {
				quietSteps = 80
				commands = append(make([]int, quietSteps), commands...)
			}
			if clearRows != 0 {
				// Four idle reads separate the initial display from the hard drop.
				commands = append([]int{0, 0, 0, 0, 16, 0}, commands...)
			}
			fixtureApplied := false
			var stepReturn, stepSP, renderReturn, renderSP uint16
			stepActive, renderActive, clockReady := false, false, false
			previousClock := byte(0)
			base := &tetrisSceneObserver{CPUCore: m.CPU, t: t, m: m, minSP: 0xfce0, returnPC: sym("demo_done"), returnSP: 0xfce0, phaseMax: map[string]int64{}}
			c := &smoothGameObserver{tetrisSceneObserver: base}
			var stepCosts map[string]int64
			var profileSamples []map[string]any
			if os.Getenv("MINZ_TETRIS_MAIN_EXPORT") != "" {
				c.profile = &directProfileObserver{tetrisSceneObserver: base, costs: map[string]int64{}}
				for _, line := range strings.Split(asm, "\n") {
					if !strings.HasPrefix(line, "; fun ") {
						continue
					}
					name := strings.Split(strings.Fields(strings.TrimPrefix(line, "; fun "))[0], "(")[0]
					if at, ok := a.Symbols[name]; ok {
						c.profile.labels = append(c.profile.labels, struct {
							address uint16
							name    string
						}{uint16(at), name})
					}
				}
				sort.Slice(c.profile.labels, func(i, j int) bool { return c.profile.labels[i].address < c.profile.labels[j].address })
			}
			m.CPU = c
			publishActive := false
			publishedRows := []int{}
			publishFirst, publishLimit := 0, 0
			partialPublications := 0
			actionTicks := map[int]map[int]int{}
			placementQueries, queryLimit, queryChecks := 0, 0, 0
			queryProof := false
			pendingLock := false
			pendingClear := false
			clearTicks := map[int]int{}
			lockTicks := map[int]int{}
			c.before = func() {
				if clearRows != 0 && !fixtureApplied && c.PC() == sym("render_smooth") {
					// Seed from independent constants, never by reading the game.
					for y := 20 - clearRows; y < 20; y++ {
						for x := 0; x < 10; x++ {
							if x != 3 {
								model.board[y*10+x] = byte(1 + (y+x)%7)
							}
						}
					}
					// Distinct partial rows above the clear exercise stable row order.
					model.board[120], model.board[121], model.board[149] = 2, 5, 7
					for i, b := range model.board {
						m.Memory.Write(sym("board")+uint16(i), b, false)
					}
					for name, value := range map[string]int{"cur_type": 0, "cur_rot": 1, "cur_x": 3, "cur_y": 0, "pixel_phase": 0, "board_dirty": 1} {
						model.v[name] = value
						m.Memory.Write(sym(name), byte(value), false)
					}
					model.rowFirst, model.rowLimit = 0, 20
					m.Memory.Write(sym("board_first"), 0, false)
					m.Memory.Write(sym("board_limit"), 20, false)
					fixtureApplied = true
				}
				if !shadow && c.PC() == sym("pixel_publish_direct") {
					publishActive = true
					publishedRows = nil
					publishFirst, publishLimit = 0, 0
					early, last := 0, 255
					if model.v["game_over"] == 0 {
						minRow, maxRow := 20, 0
						for _, cell := range pixelGameCells(model.v["cur_type"], model.v["cur_rot"]) {
							y := model.v["cur_y"] + cell.y
							first := (8*y + model.v["pixel_phase"] + 1) / 8
							end := (8*y + model.v["pixel_phase"] + 6) / 8
							if first < minRow {
								minRow = first
							}
							if end > maxRow {
								maxRow = end
							}
						}
						last = maxRow
						if model.v["board_dirty"] != 0 {
							publishFirst, publishLimit = model.rowFirst, model.rowLimit
							if publishFirst >= publishLimit || read("drawn_valid", 1)[0] == 0 {
								publishFirst, publishLimit = 0, 20
							}
							if maxRow < publishFirst || minRow >= publishLimit {
								early = 1
							} else if maxRow >= publishLimit {
								last = publishLimit - 1
							}
						}
					} else if model.v["board_dirty"] != 0 {
						publishFirst, publishLimit = model.rowFirst, model.rowLimit
						if publishFirst >= publishLimit {
							publishFirst, publishLimit = 0, 20
						}
					}
					expected := map[string]int{"direct_actor_early": early, "direct_actor_lastrow": last, "direct_refresh": model.v["board_dirty"]}
					if model.v["board_dirty"] != 0 {
						expected["direct_board_first"] = publishFirst
						expected["direct_board_limit"] = publishLimit
						if publishLimit-publishFirst < 20 {
							partialPublications++
						}
					}
					for name, w := range expected {
						if got := int(read(name, 1)[0]); got != w {
							t.Fatalf("step%d prepared %s=%d want%d", steps, name, got, w)
						}
					}
				}
				if stepActive && !shadow && c.PC() == sym("can_place") {
					placementQueries++
				}
				if !shadow && publishActive && c.PC() == sym("zx__tile_cell__tile_row_fill") {
					row := publishFirst + len(publishedRows)
					source := sym("board") + uint16(row*10)
					scan := (row + 2) * 8
					destination := uint16(0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + 11)
					if row >= publishLimit || c.HL() != source || c.DE() != destination {
						t.Fatalf("step%d row kernel outside prepared [%d,%d): row%d HL%04x DE%04x", steps, publishFirst, publishLimit, row, c.HL(), c.DE())
					}
					publishedRows = append(publishedRows, row)
				}
				if selectedLevel != 0 && c.PC() == sym("choose_difficulty") {
					m.Keyboard.Reset()
					m.Keyboard.KeyPress(3, selectedLevel-1)
				}
				if renderActive && c.PC() == renderReturn && c.SP() == renderSP+2 {
					if publishActive && len(publishedRows) != publishLimit-publishFirst {
						t.Fatalf("step%d published rows %v expected [%d,%d)", steps, publishedRows, publishFirst, publishLimit)
					}
					publishActive = false
					renderActive = false
					if !shadow && pendingLock {
						elapsed := byte(m.Memory.ReadByteInternal(0x5c78) - previousClock)
						// One IRQ remains the target. Keep measured ceilings separate
						// for ordinary locks and the seeded line-clear scenes.
						budget := byte(2)
						if clearRows != 0 {
							budget = 3
						}
						if elapsed < 1 || elapsed > budget {
							t.Fatalf("step%d lock exceeded measured budget: %d IRQ", steps, elapsed)
						}
						lockTicks[int(elapsed)]++
						if pendingClear {
							clearTicks[int(elapsed)]++
							pendingClear = false
						}
						pendingLock = false
					}
					if !shadow && selected != 0 && clockReady {
						elapsed := int(byte(m.Memory.ReadByteInternal(0x5c78) - previousClock))
						if actionTicks[selected] == nil {
							actionTicks[selected] = map[int]int{}
						}
						actionTicks[selected][elapsed]++
					}
					checkImage()
					shown = fieldImage()
					model.v["board_dirty"] = 0
					model.rowFirst = 20
					model.rowLimit = 0
					if stepCosts != nil {
						costs := map[string]int64{}
						for name, total := range c.profile.costs {
							if delta := total - stepCosts[name]; delta != 0 {
								costs[name] = delta
							}
						}
						profileSamples = append(profileSamples, map[string]any{"step": steps, "keys": selected, "phase": model.v["pixel_phase"], "costs_T": costs})
						stepCosts = nil
					}
					if !clockReady {
						if selectedLevel != 0 && int(read("difficulty_mode", 1)[0]) != selectedLevel {
							t.Fatal("keyboard menu selected the wrong difficulty")
						}
						if selectedLevel != 0 {
							for row, text := range []string{"1: 1 PX / 2 FRAMES", "2: 1 PX / FRAME"} {
								for col, ch := range text {
									for scan := 0; scan < 8; scan++ {
										address := uint16(0x4000 + scan*256 + row*32 + 6 + col)
										want := rom[0x3d00+(int(ch)-32)*8+scan]
										if got := m.Memory.ReadByteInternal(address); got != want {
											t.Fatalf("menu glyph row%d col%d scan%d=%02x want%02x", row, col, scan, got, want)
										}
									}
									if got := m.Memory.ReadByteInternal(uint16(0x5800 + row*32 + 6 + col)); got != 7 {
										t.Fatalf("menu attribute row%d col%d=%02x", row, col, got)
									}
								}
							}
						}
						previousClock = m.Memory.ReadByteInternal(0x5c78)
						clockReady = true
					}
				}
				if stepActive && c.PC() == stepReturn && c.SP() == stepSP+2 {
					if queryProof {
						if placementQueries == 0 || placementQueries > queryLimit {
							t.Fatalf("step%d soft drop made %d placement queries; crossed-row limit %d", steps, placementQueries, queryLimit)
						}
						queryChecks++
						queryProof = false
					}
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
					if c.profile != nil {
						stepCosts = map[string]int64{}
						for name, cost := range c.profile.costs {
							stepCosts[name] = cost
						}
					}
					check(fmt.Sprintf("step%d/entry", steps))
					now := read("clock_seen", 1)[0]
					ticks := int(byte(now - previousClock))
					tickHistogram[ticks]++
					if steps >= 2 && steps < quietSteps && ticks != 1 {
						t.Fatalf("steady level%d step%d missed its frame: elapsed=%d", selectedLevel, steps, ticks)
					}
					previousClock = now
					if arg(0) != selected || arg(1) != ticks {
						t.Fatalf("step%d arguments keys%d/%d ticks%d/%d", steps, arg(0), selected, arg(1), ticks)
					}
					oldRng := model.v["rng_state"]
					oldY := model.v["cur_y"]
					oldLines := model.v["lines_cleared"]
					model.step(selected, ticks)
					placementQueries = 0
					queryProof = !shadow && selected == 8 && model.v["rng_state"] == oldRng
					queryLimit = model.v["cur_y"] - oldY + 1
					if model.v["lines_cleared"] != oldLines {
						pendingClear = true
					}
					if clearRows != 0 && steps == 4 && model.v["lines_cleared"]-oldLines != clearRows {
						t.Fatalf("seeded hard drop cleared %d lines want %d", model.v["lines_cleared"]-oldLines, clearRows)
					}
					if model.v["rng_state"] != oldRng && selected != 32 {
						pendingLock = true
						locks++
					}
					stepActive = true
					stepSP = c.SP()
					stepReturn = uint16(m.Memory.ReadByteInternal(c.SP())) | uint16(m.Memory.ReadByteInternal(c.SP()+1))<<8
				}
			}
			videoDir := ""
			if root := os.Getenv("MINZ_TETRIS_MAIN_VIDEO"); root != "" && selectedLevel == 2 && mode == "ordinary" && clearRows == 0 {
				videoDir = filepath.Join(root, "frames")
				if e := os.MkdirAll(videoDir, 0755); e != nil {
					t.Fatal(e)
				}
			}
			var videoTimeline []map[string]int
			var fullVideoTimeline []map[string]int
			dumpVideo := func(dir string, number int) {
				if e := os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				img := image.NewRGBA(image.Rect(0, 0, m.ScreenWidth(), m.ScreenHeight()))
				copy(img.Pix, m.Framebuffer())
				f, e := os.Create(filepath.Join(dir, fmt.Sprintf("frame_%04d.png", number)))
				if e != nil {
					t.Fatal(e)
				}
				encodeErr := png.Encode(f, img)
				closeErr := f.Close()
				if encodeErr != nil {
					t.Fatal(encodeErr)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			}
			lastVideoFrame := -1
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
					if videoDir != "" {
						fullVideoTimeline = append(fullVideoTimeline, map[string]int{"hardware_frame": frames, "step": steps, "grid_y": model.v["cur_y"], "pixel_phase": model.v["pixel_phase"]})
						dumpVideo(filepath.Join(filepath.Dir(videoDir), "full-frames"), len(fullVideoTimeline))
					}
					if videoDir != "" && steps >= 2 && steps < quietSteps {
						if lastVideoFrame >= 0 && frames != lastVideoFrame+1 {
							t.Fatal("capture skipped an emulated frame")
						}
						lastVideoFrame = frames
						videoTimeline = append(videoTimeline, map[string]int{"hardware_frame": frames, "step": steps, "grid_y": model.v["cur_y"], "pixel_phase": model.v["pixel_phase"]})
						dumpVideo(videoDir, len(videoTimeline))
					}
				}
			}
			if !c.done || model.v["game_over"] != 1 {
				t.Fatalf("main did not finish game-over: frames%d steps%d pc%04x", frames, steps, c.PC())
			}
			if steps < 20 || nonzeroPhases < 3 || locks < 3 || c.irq < 20 {
				t.Fatalf("insufficient gameplay coverage steps%d phases%d locks%d IRQ%d", steps, nonzeroPhases, locks, c.irq)
			}
			if !shadow && clearRows == 0 && queryChecks < 3 {
				t.Fatal("insufficient soft-drop query coverage", queryChecks)
			}
			t.Logf("%d soft-drop steps bounded by independently observed grid-row changes", queryChecks)
			t.Logf("input IRQ histograms %v", actionTicks)
			if !shadow && partialPublications < 3 {
				t.Fatal("insufficient partial board publications", partialPublications)
			}
			measuredLocks := 0
			for _, count := range lockTicks {
				measuredLocks += count
			}
			if !shadow && measuredLocks != locks {
				t.Fatalf("lock timing coverage %d/%d", measuredLocks, locks)
			}
			t.Logf("%d partial board publications: exact row-kernel inputs and ordering verified; lock IRQ histogram %v; line-clear IRQ histogram %v", partialPublications, lockTicks, clearTicks)
			if clearRows != 0 && (!fixtureApplied || len(clearTicks) == 0) {
				t.Fatal("missing seeded main line-clear coverage")
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
				if selectedLevel != 0 {
					dir = filepath.Join(root, fmt.Sprintf("level-%d", selectedLevel), mode)
				}
				if clearRows != 0 {
					dir = filepath.Join(dir, fmt.Sprintf("clear-%d", clearRows))
				}
				if e := os.MkdirAll(dir, 0755); e != nil {
					t.Fatal(e)
				}
				receipt := fmt.Sprintf("PASS\nfetched_complete_field_frames=%d\nframes=%d\nsteps=%d\npixel_phase_samples=%d\nlocks=%d\ninterrupts=%d\nminimum_SP=0x%04x\nbytes=%d\n", fetchedFrames, frames, steps, nonzeroPhases, locks, c.irq, c.minSP, len(a.Binary))
				profileJSON, e := json.MarshalIndent(map[string]any{"attribution": "nearest preceding emitted function label; HALT includes wait", "per_step": profileSamples, "total_costs_T": c.profile.costs, "clock_delta_histogram": tickHistogram, "input_irq_histogram": actionTicks}, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				for name, data := range map[string][]byte{"source.nanz": src, "program.a80": []byte(asm), "program.bin": a.Binary, "receipt.txt": []byte(receipt), "instruction-costs.json": profileJSON} {
					if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			if videoDir != "" {
				if len(videoTimeline) < 70 {
					t.Fatal("too few consecutive video frames", len(videoTimeline))
				}
				data, e := json.MarshalIndent(map[string]any{"fps": 50, "source": "actual ULA framebuffer", "frames": videoTimeline, "full_session_frames": fullVideoTimeline, "interpolation": false}, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(filepath.Dir(videoDir), "timeline.json"), data, 0644); e != nil {
					t.Fatal(e)
				}
			}
			t.Logf("%d frames, %d independently checked main steps, %d fractional poses, %d locks, %d IRQ; game-over returned with SP restored; clock-delta histogram %v", frames, steps, nonzeroPhases, locks, c.irq, tickHistogram)
		})
	}
}
