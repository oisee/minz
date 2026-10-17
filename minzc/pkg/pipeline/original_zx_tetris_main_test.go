package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Unchanged main: independently modeled keyboard-driven frames and, for the
// game-over workload, its flash sequence and return. HALT wake is controlled.
func TestOriginalZXTetrisMainInitialization(t *testing.T) { testOriginalZXTetrisMainFrames(t, "idle") }
func TestOriginalZXTetrisMainMovement(t *testing.T)       { testOriginalZXTetrisMainFrames(t, "movement") }
func TestOriginalZXTetrisMainRotation(t *testing.T)       { testOriginalZXTetrisMainFrames(t, "rotation") }
func TestOriginalZXTetrisMainSoftDrop(t *testing.T)       { testOriginalZXTetrisMainFrames(t, "soft-drop") }
func TestOriginalZXTetrisMainHardDrop(t *testing.T)       { testOriginalZXTetrisMainFrames(t, "hard-drop") }
func TestOriginalZXTetrisMainHold(t *testing.T)           { testOriginalZXTetrisMainFrames(t, "hold") }
func TestOriginalZXTetrisMainGravityLock(t *testing.T) {
	testOriginalZXTetrisMainFrames(t, "gravity-lock")
}
func TestOriginalZXTetrisMainLineClear(t *testing.T) { testOriginalZXTetrisMainFrames(t, "line-clear") }
func TestOriginalZXTetrisMainGameOver(t *testing.T)  { testOriginalZXTetrisMainFrames(t, "game-over") }
func testOriginalZXTetrisMainFrames(t *testing.T, scenario string) {
	for _, source := range []struct{ name, path string }{{"current", "examples/zx/tetris.nanz"}, {"historical", "reports/showcase-src/2026-03-15/ex34_tetris.nanz"}} {
		t.Run(source.name, func(t *testing.T) {
			src, e := os.ReadFile("../../../" + source.path)
			if e != nil {
				t.Fatal(e)
			}
			frozen, e := os.ReadFile("../../../reports/zx-tetris-initial-main/frozen/" + source.name + ".nanz")
			if e != nil || !bytes.Equal(src, frozen) {
				t.Fatal("freeze", e)
			}
			save := func(n string, v any) {
				if d := os.Getenv("MINZ_ZX_TETRIS_INIT_EXPORT"); d != "" {
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
					p := filepath.Join(d, source.name, n)
					if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(p, b, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
				t.Run(mode, func(t *testing.T) {
					h, e := nanz.Parse(string(src), source.path)
					if e != nil {
						t.Fatal(e)
					}
					s, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
					if e != nil {
						t.Fatal(e)
					}
					asm := "ORG 0x8000\n" + calledMainTestAssembly(t, s)
					a, e := z80asm.NewAssembler().AssembleString(asm)
					if e != nil || len(a.Errors) > 0 {
						t.Fatal(e, a.Errors)
					}
					sym := func(n string) uint16 {
						p, ok := a.Symbols[n]
						if !ok {
							t.Fatal(n)
						}
						return uint16(p)
					}
					save(mode+"/program.a80", asm)
					save(mode+"/program.bin", a.Binary)
					save(mode+"/final.mir", s.MIR2Module.Dump())
					save(mode+"/allocation.json", s.Allocation)
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					z.Hooks = nil
					mem := z.Memory()
					copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
					copy(mem[0x8000:], a.Binary)
					p := sym("main")
					copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
					want := append([]byte(nil), mem...)
					// Direct source model: both use ascending for/while loops with the same
					// literal bounds. For-statement syntax differs from range(...).fold (Book6).
					for p := 0x4000; p < 0x5b00; p++ {
						want[p] = 0
					}
					fill := func(cx, cy int) {
						for line := 0; line < 8; line++ {
							y := cy*8 + line
							addr := 0x4000 | ((y & 0xc0) << 5) | ((y & 7) << 8) | ((y & 0x38) << 2) | (cx & 31)
							want[addr] = 255
						}
					}
					for y := 2; y < 22; y++ {
						for x := 11; x < 21; x++ {
							fill(x, y)
						}
						fill(10, y)
						want[0x5800+y*32+10] = 0x38
						fill(21, y)
						want[0x5800+y*32+21] = 0x38
					}
					for x := 10; x < 22; x++ {
						fill(x, 22)
						want[0x5800+22*32+x] = 0x38
					}
					for y := 3; y < 7; y++ {
						for x := 5; x < 9; x++ {
							fill(x, y)
						}
						for x := 23; x < 27; x++ {
							fill(x, y)
						}
					}
					// Original main advances PRNG twice: next initialization and spawn.
					rng := uint16(0xace1)
					next := func() byte {
						bit := rng & 1
						rng >>= 1
						if bit != 0 {
							rng ^= 0xb400
						}
						v := byte(rng) & 7
						if v == 7 {
							v = 0
						}
						return v
					}
					typ := next()
					queued := next()
					for _, g := range h.Globals {
						addr := sym(g.Name)
						switch g.Name {
						case "board":
							for i := 0; i < 200; i++ {
								want[addr+uint16(i)] = 0
							}
						case "rng_state":
							want[addr] = byte(rng)
							want[addr+1] = byte(rng >> 8)
						case "cur_type":
							want[addr] = typ
						case "next_type":
							want[addr] = queued
						case "cur_rot", "cur_y", "hold_used", "last_was_tspin":
							want[addr] = 0
						case "cur_x":
							want[addr] = 3
						}
					}
					borders := 0
					keyReads := 0
					keysAt := func(frame int) byte {
						if scenario == "game-over" {
							if frame%2 == 1 {
								return 16
							}
							return 0
						}
						if scenario == "line-clear" {
							switch frame {
							case 1, 3, 5:
								return 1
							case 9, 13, 15, 17, 19:
								return 2
							case 7, 11, 21:
								return 16
							}
							return 0
						}
						if scenario == "gravity-lock" {
							if frame <= 21 {
								return 8
							}
							return 0
						}
						if scenario == "hold" {
							switch frame {
							case 1, 2, 4, 6, 10, 12:
								return 32
							case 8:
								return 16
							}
							return 0
						}
						if scenario == "hard-drop" {
							if frame == 1 || frame == 2 || frame == 4 {
								return 16
							}
							return 0
						}
						if scenario == "soft-drop" {
							if frame <= 3 || (frame >= 20 && frame <= 22) {
								return 8
							}
							if frame == 4 {
								return 4
							}
							return 0
						}
						if scenario == "rotation" {
							if frame == 1 || frame == 2 || frame == 4 || frame == 6 || frame == 8 || frame == 10 {
								return 4
							}
							return 0
						}
						if scenario != "movement" {
							return 0
						}
						switch frame {
						case 1, 2:
							return 1
						case 4, 5:
							return 2
						case 7:
							return 1
						}
						if frame >= 9 && frame <= 17 && frame%2 == 1 {
							return 1
						}
						if frame >= 19 && frame <= 31 && frame%2 == 1 {
							return 2
						}
						return 0
					}
					expectedX := 3
					expectedRotation := 0
					expectedY, expectedTimer := 0, 0
					expectedType, expectedNext := typ, queued
					expectedScore := 0
					expectedLines := 0
					expectedGameOver := false
					completedFrames := 0
					flashHalts := 0
					returned := false
					expectedHold, expectedHoldUsed := byte(255), byte(0)
					var board [200]byte
					colors := []byte{0x28, 0x30, 0x18, 0x20, 0x10, 0x70, 0x08}
					// Independent literal cell layouts, matched to both source definitions.
					shape := func(t byte, rot int) [4][2]int {
						if t == 0 {
							if rot&1 == 0 {
								return [4][2]int{{0, 0}, {1, 0}, {2, 0}, {3, 0}}
							}
							return [4][2]int{{0, 0}, {0, 1}, {0, 2}, {0, 3}}
						}
						if rot != 0 {
							panic("non-I rotation outside workload")
						}
						return [7][4][2]int{{{0, 0}, {1, 0}, {2, 0}, {3, 0}}, {{0, 0}, {1, 0}, {0, 1}, {1, 1}}, {{1, 0}, {0, 1}, {1, 1}, {2, 1}}, {{1, 0}, {2, 0}, {0, 1}, {1, 1}}, {{0, 0}, {1, 0}, {1, 1}, {2, 1}}, {{2, 0}, {0, 1}, {1, 1}, {2, 1}}, {{0, 0}, {0, 1}, {1, 1}, {2, 1}}}[t]
					}
					fitsAt := func(x, y int) bool {
						for _, c := range shape(expectedType, expectedRotation) {
							x, cy := x+c[0], y+c[1]
							if x < 0 || x >= 10 || cy < 0 || cy >= 20 || board[cy*10+x] != 0 {
								return false
							}
						}
						return true
					}
					fits := func(y int) bool { return fitsAt(expectedX, y) }
					lockAndSpawn := func() {
						for _, c := range shape(expectedType, expectedRotation) {
							board[(expectedY+c[1])*10+expectedX+c[0]] = expectedType + 1
						}
						// Independent stable row compaction, rather than the source's repeated
						// bottom-up scan and in-place shifting. Preserve retained row order.
						retained := make([][10]byte, 0, 20)
						cleared := 0
						for y := 0; y < 20; y++ {
							var row [10]byte
							copy(row[:], board[y*10:y*10+10])
							full := true
							for _, v := range row {
								if v == 0 {
									full = false
								}
							}
							if full {
								cleared++
							} else {
								retained = append(retained, row)
							}
						}
						board = [200]byte{}
						for i, row := range retained {
							copy(board[(cleared+i)*10:], row[:])
						}
						if cleared > 4 {
							t.Fatal("model scoring outside workload", cleared)
						}
						expectedScore += []int{0, 100, 300, 500, 800}[cleared]
						expectedLines += cleared
						if expectedLines >= 8 {
							t.Fatal("model requires speed-change support")
						}
						expectedHoldUsed = 0
						expectedType = expectedNext
						expectedNext = next()
						expectedRotation = 0
						expectedX = 3
						expectedY = 0
						want[sym("rng_state")] = byte(rng)
						want[sym("rng_state")+1] = byte(rng >> 8)
						if !fits(0) {
							expectedGameOver = true
						}
					}
					previousKeys := byte(0)
					awake := false
					z.SetIOHandlers(func(p uint16) byte {
						rows := []byte{0xdf, 0xfb, 0xfd, 0x7f, 0xbf}
						if !awake || byte(p) != 0xfe || keyReads >= 155 || byte(p>>8) != rows[keyReads%5] {
							t.Fatal("keyboard", p, keyReads)
						}
						keys := keysAt(keyReads/5 + 1)
						row := keyReads % 5
						keyReads++
						v := byte(255)
						if row == 0 {
							if keys&1 != 0 {
								v &^= 2
							}
							if keys&2 != 0 {
								v &^= 1
							}
						}
						if row == 4 && keys&32 != 0 {
							v &^= 16
						}
						if row == 3 && keys&16 != 0 {
							v &^= 1
						}
						if row == 2 && keys&8 != 0 {
							v &^= 1
						}
						if row == 1 && keys&4 != 0 {
							v &^= 1
						}
						return v
					}, func(p uint16, v byte) {
						expectedBorder := byte(0)
						if borders > 0 {
							expectedBorder = 2
							if borders%2 == 0 {
								expectedBorder = 0
							}
						}
						if byte(p) != 0xfe || byte(p>>8) != v || v != expectedBorder || (borders > 0 && !expectedGameOver) {
							t.Fatal("border", p, v)
						}
						borders++
					})
					z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x2468, IY: 0x1357})
					minSP := uint16(0xf000)
					halt := false
					for step := 0; step < 5000000; step++ {
						pc := z.GetPC()
						r := z.GetRegisters()
						if pc != 0x7000 && (pc < 0x8000 || int(pc) >= 0x8000+len(a.Binary)) {
							t.Fatal("foreign fetch", pc)
						}
						if mem[pc] == 0x76 {
							if pc != sym("zx_halt") || r.SP >= 0xeffe {
								t.Fatal("first game HALT", r)
							}
							ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
							if ret < 0x8003 || mem[ret-3] != 0xcd || uint16(mem[ret-2])|uint16(mem[ret-1])<<8 != pc {
								t.Fatal("HALT CALL authority", ret)
							}
							halt = true
							break
						}
						z.Step()
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
						if minSP < 0xe000 || z.GetSP() > 0xf000 {
							t.Fatal("stack")
						}
					}
					save(mode+"/ram.bin", mem)
					save(mode+"/registers.json", z.GetRegisters())
					if !halt || borders != 1 || uint16(mem[0xeffe])|uint16(mem[0xefff])<<8 != 0x7003 {
						t.Fatal("incomplete prefix", halt, borders)
					}
					copy(want[minSP:0xf000], mem[minSP:0xf000])
					for _, l := range a.Listing {
						if strings.HasPrefix(l.Label, "_spill_") {
							copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
						}
					}
					if !bytes.Equal(want, mem) {
						for p := range mem {
							if mem[p] != want[p] {
								t.Fatalf("RAM %04x got %02x want %02x", p, mem[p], want[p])
							}
						}
					}
					// Controlled HALT wake, not an interrupt emulation claim.
					if typ != 0 || queued != 0 {
						t.Fatal("initial I queue", typ, queued)
					}
					for frame := 1; frame <= 31; frame++ {
						awake = true
						keys := keysAt(frame)
						pressed := keys & (keys ^ previousKeys)
						if pressed&1 != 0 && expectedX > 0 && fitsAt(expectedX-1, expectedY) {
							expectedX--
						}
						if pressed&2 != 0 && fitsAt(expectedX+1, expectedY) {
							expectedX++
						}
						if pressed&4 != 0 {
							expectedRotation = (expectedRotation + 1) & 3
						}
						if pressed&16 != 0 {
							for fits(expectedY + 1) {
								expectedY++
								expectedScore += 2
							}
							lockAndSpawn()
							expectedTimer = 0
						}
						if pressed&32 != 0 && expectedHoldUsed == 0 {
							expectedHoldUsed = 1
							if expectedHold == 255 {
								expectedHold = expectedType
								expectedType = expectedNext
								expectedNext = next()
								expectedHoldUsed = 0
							} else {
								expectedType, expectedHold = expectedHold, expectedType
							}
							expectedRotation = 0
							expectedX = 3
							expectedY = 0
							want[sym("rng_state")] = byte(rng)
							want[sym("rng_state")+1] = byte(rng >> 8)
						}
						previousKeys = keys
						// A is level-triggered. Timer reset precedes the same-frame gravity tick.
						if keys&8 != 0 {
							expectedTimer = 30
						}
						expectedTimer++
						if expectedTimer >= 30 {
							expectedTimer = 0
							if fits(expectedY + 1) {
								expectedY++
							} else {
								lockAndSpawn()
							}
						}
						if expectedGameOver {
							want[sym("game_over")] = 1
						}
						want[sym("hold_type")] = expectedHold
						want[sym("hold_used")] = expectedHoldUsed
						want[sym("cur_type")] = expectedType
						want[sym("next_type")] = expectedNext
						want[sym("lines_cleared")] = byte(expectedLines)
						want[sym("lines_cleared")+1] = byte(expectedLines >> 8)
						want[sym("score")] = byte(expectedScore)
						want[sym("score")+1] = byte(expectedScore >> 8)
						copy(want[int(sym("board")):int(sym("board"))+200], board[:])
						want[sym("cur_rot")] = byte(expectedRotation)
						want[sym("cur_x")] = byte(expectedX)
						want[sym("prev_keys")] = keys
						r := z.GetRegisters()
						r.PC++
						z.SetRegisters(r)
						for y := 0; y < 20; y++ {
							for x := 0; x < 10; x++ {
								cell := board[y*10+x]
								attr := byte(0)
								if cell != 0 {
									attr = colors[cell-1]
								}
								want[0x5800+(2+y)*32+11+x] = attr
							}
						}
						ghost := expectedY
						for fits(ghost + 1) {
							ghost++
						}
						for _, c := range shape(expectedType, expectedRotation) {
							dx, dy := c[0], c[1]
							if ghost != expectedY && board[(ghost+dy)*10+expectedX+dx] == 0 {
								want[0x5800+(2+ghost+dy)*32+11+expectedX+dx] = (colors[expectedType] >> 3) & 7
							}
							want[0x5800+(2+expectedY+dy)*32+11+expectedX+dx] = colors[expectedType]
						}
						for _, sx := range []int{23, 5} {
							for y := 0; y < 4; y++ {
								for x := 0; x < 4; x++ {
									want[0x5800+(3+y)*32+sx+x] = 0
								}
							}
						}
						for _, c := range shape(expectedNext, 0) {
							want[0x5800+(3+c[1])*32+23+c[0]] = colors[expectedNext]
						}
						if expectedHold < 7 {
							for _, c := range shape(expectedHold, 0) {
								want[0x5800+(3+c[1])*32+5+c[0]] = colors[expectedHold]
							}
						}
						want[sym("frame_count")] = byte(frame)
						want[sym("frame_count")+1] = 0
						want[sym("drop_timer")] = byte(expectedTimer)
						want[sym("cur_y")] = byte(expectedY)
						halt = false
						for step := 0; step < 5000000; step++ {
							pc := z.GetPC()
							if pc < 0x8000 || int(pc) >= 0x8000+len(a.Binary) {
								t.Fatal("frame fetch", pc)
							}
							if mem[pc] == 0x76 {
								if pc != sym("zx_halt") {
									t.Fatal("second HALT", pc)
								}
								halt = true
								break
							}
							z.Step()
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
							if minSP < 0xe000 || z.GetSP() > 0xf000 {
								t.Fatal("frame stack")
							}
						}
						expectedBorders := 1
						if expectedGameOver {
							expectedBorders = 2
						}
						if !halt || keyReads != frame*5 || borders != expectedBorders {
							t.Fatal("frame incomplete", frame, halt, keyReads, borders)
						}
						copy(want[minSP:0xf000], mem[minSP:0xf000])
						for _, l := range a.Listing {
							if strings.HasPrefix(l.Label, "_spill_") {
								copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
							}
						}
						if !bytes.Equal(want, mem) {
							for p := range mem {
								if want[p] != mem[p] {
									t.Fatalf("frame RAM %04x got %02x want %02x", p, mem[p], want[p])
								}
							}
						}
						completedFrames = frame
						if frame == 1 || frame == 30 || frame == 31 || expectedGameOver {
							save(fmt.Sprintf("%s/frame-%d-ram.bin", mode, frame), mem)
						}
						if expectedGameOver {
							break
						}
					}
					if scenario == "game-over" {
						if !expectedGameOver || completedFrames != 23 || expectedScore != 234 {
							t.Fatal("game-over workload was not reached", completedFrames, expectedScore)
						}
						for step := 0; step < 5000000; step++ {
							pc := z.GetPC()
							if pc == 0x7003 && z.GetSP() == 0xf000 {
								returned = true
								break
							}
							if pc < 0x8000 || int(pc) >= 0x8000+len(a.Binary) {
								t.Fatal("flash foreign fetch", pc)
							}
							if mem[pc] == 0x76 {
								if pc != sym("zx_halt") {
									t.Fatal("flash halt authority", pc)
								}
								flashHalts++
								if flashHalts > 120 || borders != 2+(flashHalts-1)/3 {
									t.Fatal("flash sequence", flashHalts, borders)
								}
								r := z.GetRegisters()
								ret := uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
								if ret < 0x8003 || mem[ret-3] != 0xcd || uint16(mem[ret-2])|uint16(mem[ret-1])<<8 != pc {
									t.Fatal("flash CALL authority", ret)
								}
								r.PC++
								z.SetRegisters(r)
							} else {
								z.Step()
							}
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
							if minSP < 0xe000 || z.GetSP() > 0xf000 {
								t.Fatal("flash stack")
							}
							copy(want[minSP:0xf000], mem[minSP:0xf000])
							for _, l := range a.Listing {
								if strings.HasPrefix(l.Label, "_spill_") {
									copy(want[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
								}
							}
							if !bytes.Equal(want, mem) {
								t.Fatal("flash changed non-stack/non-spill RAM", flashHalts)
							}
						}
						if !returned || flashHalts != 120 || borders != 41 || keyReads != completedFrames*5 {
							t.Fatal("game return", returned, flashHalts, borders, keyReads)
						}
						save(mode+"/returned-ram.bin", mem)
						save(mode+"/returned-registers.json", z.GetRegisters())
					}
					if scenario == "line-clear" {
						if expectedLines != 1 || expectedScore != 212 || board[197] != 5 || board[198] != 5 {
							t.Fatal("line-clear workload was not reached")
						}
						for i, v := range board {
							if i != 197 && i != 198 && v != 0 {
								t.Fatal("unexpected retained cell", i, v)
							}
						}
					}
					t.Log(completedFrames, "frames/full RAM PASS; returned", returned)
					save(mode+"/screen.bin", mem[0x4000:0x5b00])
					save(mode+"/summary.json", map[string]any{"bytes": len(a.Binary), "T_states": z.GetCycles(), "source_sha256": fmt.Sprintf("%x", sha256.Sum256(src)), "rng": rng, "type": typ, "next": queued, "minimum_sp": minSP, "full_ram": true, "scenario": scenario, "final_hold": expectedHold, "final_hold_used": expectedHoldUsed, "final_lines": expectedLines, "final_score": expectedScore, "final_type": expectedType, "final_next": expectedNext, "final_y": expectedY, "final_timer": expectedTimer, "frames": completedFrames, "flash_halts": flashHalts, "main_return": returned, "termination": map[bool]string{true: "main returned PC7003 SPf000", false: "bounded before frame32 game HALT"}[returned]})
					t.Log(len(a.Binary), z.GetCycles(), "initial main and", completedFrames, "frames/full RAM PASS")
				})
			}
		})
	}
}
