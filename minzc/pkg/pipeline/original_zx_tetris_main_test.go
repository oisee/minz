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

// Initialization prefix of unchanged main, bounded before the first game HALT.
// No interrupt or keyboard/game-frame claim is made by this test.
func TestOriginalZXTetrisMainInitialization(t *testing.T) { testOriginalZXTetrisMainFrames(t, false) }
func TestOriginalZXTetrisMainMovement(t *testing.T)       { testOriginalZXTetrisMainFrames(t, true) }
func testOriginalZXTetrisMainFrames(t *testing.T, movement bool) {
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
					asm := "ORG 0x8000\n" + s.Assembly
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
						if !movement {
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
						return v
					}, func(p uint16, v byte) {
						if byte(p) != 0xfe || byte(p>>8) != 0 || v != 0 || borders != 0 {
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
						if pressed&1 != 0 && expectedX > 0 {
							expectedX--
						}
						if pressed&2 != 0 && expectedX+4 < 10 {
							expectedX++
						}
						previousKeys = keys
						want[sym("cur_x")] = byte(expectedX)
						want[sym("prev_keys")] = keys
						r := z.GetRegisters()
						r.PC++
						z.SetRegisters(r)
						for y := 0; y < 20; y++ {
							for x := 0; x < 10; x++ {
								want[0x5800+(2+y)*32+11+x] = 0
							}
						}
						for x := expectedX; x < expectedX+4; x++ {
							want[0x5800+21*32+11+x] = 5
							want[0x5800+(2+frame/30)*32+11+x] = 0x28
						}
						for _, sx := range []int{23, 5} {
							for y := 0; y < 4; y++ {
								for x := 0; x < 4; x++ {
									want[0x5800+(3+y)*32+sx+x] = 0
								}
							}
						}
						for x := 0; x < 4; x++ {
							want[0x5800+3*32+23+x] = 0x28
						}
						want[sym("frame_count")] = byte(frame)
						want[sym("frame_count")+1] = 0
						want[sym("drop_timer")] = byte(frame % 30)
						want[sym("cur_y")] = byte(frame / 30)
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
						if !halt || keyReads != frame*5 || borders != 1 {
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
						if frame == 1 || frame == 30 || frame == 31 {
							save(fmt.Sprintf("%s/frame-%d-ram.bin", mode, frame), mem)
						}
					}
					t.Log("31 frames including input edges and gravity/full RAM PASS")
					save(mode+"/screen.bin", mem[0x4000:0x5b00])
					save(mode+"/summary.json", map[string]any{"bytes": len(a.Binary), "T_states": z.GetCycles(), "source_sha256": fmt.Sprintf("%x", sha256.Sum256(src)), "rng": rng, "type": typ, "next": queued, "minimum_sp": minSP, "full_ram": true, "termination": "bounded before frame32 game HALT"})
					t.Log(len(a.Binary), z.GetCycles(), "initial main and31frames/full RAM PASS")
				})
			}
		})
	}
}
