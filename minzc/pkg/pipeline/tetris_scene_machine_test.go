package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/z80asm"
)

func scenePreparationSource(t *testing.T, source, variant string) string {
	t.Helper()
	var descriptor string
	switch variant {
	case "loop":
		descriptor = "prepare \"loop\""
	case "baked8":
		descriptor = "prepare \"baked\" 8"
	case "baked16":
		descriptor = "prepare \"baked\" 16"
	case "baked32":
		descriptor = "prepare \"baked\" 32"
	case "baked64":
		descriptor = "prepare \"baked\" 64"
	default:
		t.Fatal("unknown scene prepare candidate", variant)
	}
	source = regexp.MustCompile(`(?m)^[ \t]*prepare[^\n]*\n`).ReplaceAllString(source, "")
	opening := regexp.MustCompile(`@scene\("[A-Za-z_][A-Za-z_0-9]*"\)[ \t]*\{`)
	if len(opening.FindAllStringIndex(source, -1)) != 1 {
		t.Fatal("candidate source needs exactly one scene")
	}
	return opening.ReplaceAllStringFunc(source, func(s string) string { return s + "\n" + descriptor + "\n" })
}

type tetrisSceneObserver struct {
	spectrum.CPUCore
	t                                      *testing.T
	m                                      *spectrum.Machine
	done, irqActive                        bool
	irqPC, irqSP                           uint16
	irqRegs                                rotoMachineRegisters
	irq, transient                         int
	unchanged                              map[uint16]byte
	start, publishStart, elapsed, publishT int64
	publish                                uint16
	minSP                                  uint16
	returnSP                               uint16
	returnPC                               uint16
	phases                                 map[uint16]string
	activePhase                            string
	phaseReturn, phaseSP                   uint16
	phaseStart                             int64
	phaseMax                               map[string]int64
	phaseLast                              map[string]int64
}

func (c *tetrisSceneObserver) Interrupt() {
	if c.IFF1() {
		c.irq++
		c.irqActive = true
		c.irqPC, c.irqSP = c.PC(), c.SP()
		if c.Halted() {
			c.irqPC++
		}
		c.irqRegs = rotoMachineRegs(c.CPUCore)
	}
	c.CPUCore.Interrupt()
}
func (c *tetrisSceneObserver) DoOpcode() {
	if c.activePhase != "" && c.PC() == c.phaseReturn && c.SP() == c.phaseSP+2 {
		elapsed := c.m.AbsoluteTStates() - c.phaseStart
		c.phaseLast[c.activePhase] = elapsed
		if elapsed > c.phaseMax[c.activePhase] {
			c.phaseMax[c.activePhase] = elapsed
		}
		c.activePhase = ""
	}
	if name := c.phases[c.PC()]; name != "" && c.activePhase == "" {
		c.activePhase = name
		c.phaseStart = c.m.AbsoluteTStates()
		c.phaseSP = c.SP()
		c.phaseReturn = uint16(c.m.Memory.ReadByteInternal(c.SP())) | uint16(c.m.Memory.ReadByteInternal(c.SP()+1))<<8
	}

	if !c.done && c.SP() < c.minSP {
		c.minSP = c.SP()
	}
	if c.irqActive && c.PC() == c.irqPC && c.SP() == c.irqSP {
		if rotoMachineRegs(c.CPUCore) != c.irqRegs {
			c.t.Fatal("IM2 register preservation")
		}
		c.irqActive = false
	}
	if !c.done && c.PC() == c.publish && c.publish != 0 {
		c.publishStart = c.m.AbsoluteTStates()
	}
	returnPC := c.returnPC
	if returnPC == 0 {
		returnPC = 0x7000
	}
	if !c.done && c.PC() == returnPC {
		expectedSP := c.returnSP
		if expectedSP == 0 {
			expectedSP = 0xeff2
		}
		if c.SP() != expectedSP {
			c.t.Fatal("renderer return SP", c.SP())
		}
		if !c.IFF1() || !c.IFF2() {
			c.t.Fatal("renderer did not preserve enabled INT")
		}
		c.done = true
		c.elapsed = c.m.AbsoluteTStates() - c.start
		c.publishT = 0
		if c.publishStart != 0 {
			c.publishT = c.m.AbsoluteTStates() - c.publishStart
		}
		c.SetIFF1(false)
		c.SetIFF2(false)
	}
	c.CPUCore.DoOpcode()
	if !c.done {
		for a, want := range c.unchanged {
			if c.m.Memory.ReadByteInternal(a) != want {
				c.transient++
			}
		}
	}
}

// This tests the actual playable Spectrum source, rather than CP/M Tetris or
// the earlier tetris.nanz. The oracle uses O/I/T geometry and physical ULA
// addresses. It inspects fetched pixels without RenderFullScreen.
func TestTetrisSceneTemporalMachine(t *testing.T) {
	rom, e := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if e != nil {
		t.Fatal(e)
	}
	boot, e := os.ReadFile("../../../scripts/zx_tetris_entry.a80")
	if e != nil {
		t.Fatal(e)
	}
	colors := []byte{0, 5, 6, 3, 4, 2, 0x46, 1}
	geometry := [][4][2]int{{{0, 0}, {1, 0}, {2, 0}, {3, 0}}, {{0, 0}, {1, 0}, {0, 1}, {1, 1}}, {{1, 0}, {0, 1}, {1, 1}, {2, 1}}}
	board := make([]byte, 200)
	for x := 0; x < 10; x++ {
		board[190+x] = byte(x%7 + 1)
	}
	// Pixel RAM is independently initialized exactly for owned cells and walls.
	initial := make([]byte, 6912)
	solid := func(cx, cy int) {
		for line := 0; line < 8; line++ {
			y := cy*8 + line
			a := (y&0xc0)*32 + (y&7)*256 + (y&0x38)*4 + cx
			initial[a] = 255
		}
	}
	for y := 2; y < 22; y++ {
		for x := 11; x < 21; x++ {
			solid(x, y)
		}
		for _, x := range []int{10, 21} {
			solid(x, y)
			initial[6144+y*32+x] = 7
		}
	}
	for x := 10; x < 22; x++ {
		solid(x, 22)
		initial[6144+22*32+x] = 7
	}
	for y := 3; y < 7; y++ {
		for _, base := range []int{5, 23} {
			for x := 0; x < 4; x++ {
				solid(base+x, y)
			}
		}
	}
	image := func(x, y, next, hold int) []byte {
		want := append([]byte(nil), initial...)
		for i, v := range board {
			want[6144+(2+i/10)*32+11+i%10] = colors[v]
		}
		for _, p := range geometry[1] {
			want[6144+(2+y+p[1])*32+11+x+p[0]] = 6
		}
		for _, p := range []struct{ t, base int }{{next, 23}, {hold, 5}} {
			if p.t < 3 {
				for _, d := range geometry[p.t] {
					want[6144+(3+d[1])*32+p.base+d[0]] = colors[p.t+1]
				}
			}
		}
		return want
	}
	pixels := func(screen []byte) []byte {
		rgba := make([]byte, 256*192*4)
		for y := 0; y < 192; y++ {
			for x := 0; x < 256; x++ {
				address := (y&0xc0)*32 + (y&7)*256 + (y&0x38)*4 + x/8
				attr := screen[6144+(y/8)*32+x/8]
				ink := attr & 7
				if screen[address]&(128>>uint(x&7)) == 0 {
					ink = (attr >> 3) & 7
				}
				level := byte(0xcd)
				if attr&64 != 0 {
					level = 255
				}
				offset := (y*256 + x) * 4
				if ink&2 != 0 {
					rgba[offset] = level
				}
				if ink&4 != 0 {
					rgba[offset+1] = level
				}
				if ink&1 != 0 {
					rgba[offset+2] = level
				}
				rgba[offset+3] = 255
			}
		}
		return rgba
	}
	for _, baseline := range []bool{true, false} {
		name, path := "scene", "../../../reports/advanced-demos/tetris/shadow-candidate.nanz"
		if baseline {
			name, path = "baseline", "../../../reports/advanced-demos/tetris/baseline.nanz"
		}
		src, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				source := string(src)
				prepareVariant := "source"
				if !baseline && os.Getenv("MINZ_TETRIS_SCENE_PREPARE") != "" {
					prepareVariant = os.Getenv("MINZ_TETRIS_SCENE_PREPARE")
					source = scenePreparationSource(t, source, prepareVariant)
				}

				entry := "render"
				if baseline {
					source += "\nfun frame_render() { zx_halt()\nrender() }\n"
					entry = "frame_render"
				}
				h, e := nanz.ParseWithOpts(source, "tetris-scene.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
				if e != nil {
					t.Fatal(e)
				}
				steps, e := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
				if e != nil {
					t.Fatal(e)
				}
				assembly := string(boot) + "\n" + steps.Assembly
				a, e := z80asm.NewAssembler().AssembleString(assembly)
				if e != nil {
					t.Fatal(e)
				}
				if len(a.Errors) > 0 {
					t.Fatal(a.Errors)
				}
				for x := 0; x < 10; x++ {
					board[140+x] = 0
				}
				m, e := spectrum.New48K(rom)
				if e != nil {
					t.Fatal(e)
				}
				for i, b := range a.Binary {
					m.Memory.Write(0x6000+uint16(i), b, false)
				}
				for i, b := range initial {
					m.Memory.Write(0x4000+uint16(i), b, false)
				}
				// Exact launcher IM2 table, bypassing slow startup only for render fixtures.
				for addr := 0xfe00; addr <= 0xff00; addr++ {
					m.Memory.Write(uint16(addr), 0xfd, false)
				}
				irq := a.Symbols["demo_irq"]
				m.Memory.Write(0xfdfd, 0xc3, false)
				m.Memory.Write(0xfdfe, byte(irq), false)
				m.Memory.Write(0xfdff, byte(irq>>8), false)
				symbol := func(n string) uint16 {
					v, ok := a.Symbols[n]
					if !ok {
						t.Fatal("missing symbol", n)
					}
					return uint16(v)
				}
				set := func(n string, v byte) { m.Memory.Write(symbol(n), v, false) }
				for i, b := range board {
					m.Memory.Write(symbol("board")+uint16(i), b, false)
				}
				c := &tetrisSceneObserver{CPUCore: m.CPU, t: t, m: m}
				if !baseline {
					c.publish = symbol("tetris_publish")
					c.phases = map[uint16]string{symbol("compose"): "compose", symbol("tetris_prepare"): "prepare"}
					c.phaseMax = map[string]int64{}
				}
				m.CPU = c
				save := func(n string, b []byte) {
					if d := os.Getenv("MINZ_TETRIS_SCENE_EXPORT"); d != "" {
						p := filepath.Join(d, name, mode, n)
						if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
							t.Fatal(e)
						}
						if e := os.WriteFile(p, b, 0644); e != nil {
							t.Fatal(e)
						}
					}
				}
				previous := append([]byte(nil), initial...)
				partial, displayed, transient := 0, 0, 0
				var maxPublish, maxRender int64
				var samples []map[string]any
				poses := [][4]int{{3, 0, 0, 255}, {3, 0, 0, 255}, {4, 0, 1, 255}, {4, 1, 1, 255}, {4, 1, 2, 1}, {4, 1, 2, 1}, {4, 1, 2, 1}, {3, 1, 2, 1}, {3, 2, 2, 1}, {3, 2, 2, 1}, {5, 2, 2, 0}, {5, 2, 2, 0}, {5, 16, 2, 0}, {5, 0, 2, 0}}
				for sample, pose := range poses {
					x, y, next, hold := pose[0], pose[1], pose[2], pose[3]
					if sample == 6 || sample == 9 || sample == 12 {
						for cx := 0; cx < 10; cx++ {
							value := byte(0)
							if sample == 6 && cx%2 == 0 {
								value = byte(cx%7 + 1)
							}
							if sample == 12 && (cx == 5 || cx == 6) {
								value = 6
							}
							board[140+cx] = value
							m.Memory.Write(symbol("board")+uint16(140+cx), value, false)
						}
						if !baseline {
							set("board_dirty", 1)
						}
					}
					want := image(x, y, next, hold)
					set("cur_type", 1)
					set("cur_rot", 0)
					set("cur_x", byte(x))
					set("cur_y", byte(y))
					set("next_type", byte(next))
					set("hold_type", byte(hold))
					unchanged := map[uint16]byte{}
					// Current cells which are also visible in the previous image may never be
					// erased even briefly. Preview changes are tested by full fetched pixels.
					for _, p := range geometry[1] {
						offset := 6144 + (2+y+p[1])*32 + 11 + x + p[0]
						if previous[offset] == want[offset] {
							unchanged[0x4000+uint16(offset)] = want[offset]
						}
					}
					ts := c.Tstates()
					c.Reset()
					c.SetTstates(ts)
					c.SetPC(symbol(entry))
					c.SetSP(0xfce0)
					c.SetI(0xfe)
					c.SetIM(2)
					c.SetIFF1(true)
					c.SetIFF2(true)
					m.Memory.Write(0xfce0, 0, false)
					m.Memory.Write(0xfce1, 0x5e, false)
					m.Memory.Write(0x5e00, 0x76, false)
					c.done, c.irqActive = false, false
					c.unchanged = unchanged
					c.transient = 0
					c.publishStart = 0
					c.start = m.AbsoluteTStates()
					c.phaseLast = map[string]int64{}
					sampleFrames := m.FrameCount()
					c.minSP = 0xfce0
					c.returnSP = 0xfce2
					c.returnPC = 0x5e00
					irqBefore := c.irq
					beforeRAM := make([]byte, 65536)
					for address := range beforeRAM {
						beforeRAM[address] = m.Memory.ReadByteInternal(uint16(address))
					}
					allowed := map[int]bool{}
					if !baseline {
						for n, width := range map[string]int{"tetris_values": 232, "tetris_shown": 232, "tetris_dirty_cells": 232, "tetris_dirty_values": 232, "tetris_dirty_count": 1, "board_dirty": 1, "drawn_valid": 1, "drawn_type": 1, "drawn_rot": 1, "drawn_x": 1, "drawn_y": 1, "drawn_ghost_y": 1, "drawn_next": 1, "drawn_hold": 1, "render_changed": 1} {
							for i := 0; i < width; i++ {
								allowed[int(symbol(n))+i] = true
							}
						}
					}
					for _, line := range strings.Split(assembly, "\n") {
						fields := strings.Fields(line)
						if len(fields) < 3 || !strings.HasPrefix(fields[0], "_spill_") {
							continue
						}
						start := int(symbol(strings.TrimSuffix(fields[0], ":")))
						width := 1
						if fields[1] == "DW" {
							width = 2
						} else if fields[1] != "DB" {
							t.Fatal("spill declaration", line)
						}
						count := len(strings.Split(strings.Join(fields[2:], ""), ","))
						for i := 0; i < width*count; i++ {
							allowed[start+i] = true
						}
					}

					legal := map[[32]byte]bool{sha256.Sum256(pixels(previous)): true, sha256.Sum256(pixels(want)): true}
					for frame := 0; frame < 20; frame++ {
						m.RunFrame()
						fb := m.ULA.Framebuffer()
						got := make([]byte, 256*192*4)
						for py := 0; py < 192; py++ {
							start := ((py+m.Mode.BorderTop)*m.Mode.TotalPixelWidth + m.Mode.BorderLeft) * 4
							copy(got[py*256*4:(py+1)*256*4], fb[start:start+256*4])
						}
						displayed++
						if !legal[sha256.Sum256(got)] {
							partial++
							save(fmt.Sprintf("partial-%02d-%02d.rgba", sample, frame), got)
						}
						if c.done {
							break
						}
					}
					if !c.done {
						t.Fatal("renderer did not return")
					}
					got := make([]byte, 6912)
					for i := range got {
						got[i] = m.Memory.ReadByteInternal(0x4000 + uint16(i))
					}
					for address, old := range beforeRAM {
						expected := old
						if address == 0x5c78 {
							expected = old + byte(c.irq-irqBefore)
						}
						if address >= 0x4000 && address < 0x5b00 {
							expected = want[address-0x4000]
						} else if allowed[address] || (address >= int(c.minSP) && address < 0xfce0) {
							continue
						}
						if value := m.Memory.ReadByteInternal(uint16(address)); value != expected {
							t.Fatalf("sample %d RAM[%04x]=%02x want %02x", sample, address, value, expected)
						}
					}
					if !bytes.Equal(got, want) {
						for i := range got {
							if got[i] != want[i] {
								t.Fatalf("sample %d screen[%04x]=%02x want %02x", sample, 0x4000+i, got[i], want[i])
							}
						}
					}
					for i, b := range board {
						if m.Memory.ReadByteInternal(symbol("board")+uint16(i)) != b {
							t.Fatal("renderer changed board")
						}
					}
					for n, v := range map[string]byte{"cur_type": 1, "cur_rot": 0, "cur_x": byte(x), "cur_y": byte(y), "next_type": byte(next), "hold_type": byte(hold)} {
						if m.Memory.ReadByteInternal(symbol(n)) != v {
							t.Fatal("renderer changed gameplay", n)
						}
					}
					if !baseline {
						gy := y
						canPlace := func(row int) bool {
							for _, p := range geometry[1] {
								cx, cy := x+p[0], row+p[1]
								if cx >= 10 || cy >= 20 || board[cy*10+cx] != 0 {
									return false
								}
							}
							return true
						}
						for canPlace(gy + 1) {
							gy++
						}
						if got := m.Memory.ReadByteInternal(symbol("drawn_ghost_y")); int(got) != gy {
							t.Fatalf("sample %d cached landing %d want %d", sample, got, gy)
						}
					}
					if !baseline && bytes.Equal(previous, want) && c.phaseLast["prepare"] != 0 {
						t.Fatal("unchanged frame unnecessarily prepared")
					}
					samples = append(samples, map[string]any{"sample": sample, "pose": []int{x, y, next, hold}, "render_wait_T": c.elapsed, "publish_T": c.publishT, "phases_T": c.phaseLast, "video_frames": m.FrameCount() - sampleFrames})
					transient += c.transient
					if c.publishT > maxPublish {
						maxPublish = c.publishT
					}
					if c.elapsed > maxRender {
						maxRender = c.elapsed
					}
					save(fmt.Sprintf("completed-%02d.scr", sample), got)
					previous = want
				}
				if baseline {
					if transient == 0 || partial == 0 {
						t.Fatal("negative control missed erase/flicker", transient, partial)
					}
				} else {
					if transient != 0 || partial != 0 {
						t.Fatalf("transient erase %d; partial fetched frames %d/%d", transient, partial, displayed)
					}
					if maxPublish >= int64(m.Mode.TStatesPerFrame()) {
						t.Fatal("publish exceeds frame", maxPublish)
					}
				}
				if c.irq == 0 {
					t.Fatal("no IM2 interrupts")
				}
				result, _ := json.MarshalIndent(map[string]any{"PASS": true, "baseline": baseline, "mode": mode, "samples": len(poses), "partial_frames": partial, "displayed_frames": displayed, "transient_erase_observations": transient, "max_render_including_wait_T": maxRender, "max_publish_T": maxPublish, "binary_bytes": len(a.Binary), "IM2_IRQ": c.irq, "phase_max_T": c.phaseMax, "per_sample": samples, "prepare_variant": prepareVariant}, "", "  ")
				save("result.json", result)
				save("source.nanz", []byte(source))
				save("program.a80", []byte(assembly))
				save("program.bin", a.Binary)
				t.Logf("%s: partial=%d/%d transient=%d publish=%d T renderer+wait=%d T bytes=%d IRQ=%d", name, partial, displayed, transient, maxPublish, maxRender, len(a.Binary), c.irq)
				t.Logf("phase timings: %v; samples: %v", c.phaseMax, samples)
			})
		}
	}
}
