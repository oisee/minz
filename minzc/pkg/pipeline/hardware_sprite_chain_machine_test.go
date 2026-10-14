package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
)

type spriteChainMachineObserver struct {
	spectrum.CPUCore
	t                         *testing.T
	machine                   *spectrum.Machine
	draw, pushEntry           uint16
	active                    bool
	frame                     byte
	completed, irq, unsafeIRQ int
	returnPC, callSP          uint16
	irqActive                 bool
	irqPC, irqSP              uint16
	irqRegs                   rotoMachineRegisters
	onComplete                func(byte)
	pushActive                bool
	pushReturn, pushSP        uint16
	drawStart                 int64
	maxDrawT                  int64
}

func (c *spriteChainMachineObserver) Interrupt() {
	if c.IFF1() {
		if c.pushActive {
			c.unsafeIRQ++
			c.t.Fatal("IRQ accepted while PUSH kernel owns screen SP")
		}
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
func (c *spriteChainMachineObserver) DoOpcode() {
	if c.irqActive && c.PC() == c.irqPC && c.SP() == c.irqSP {
		if rotoMachineRegs(c.CPUCore) != c.irqRegs {
			c.t.Fatal("ROM IRQ register preservation")
		}
		c.irqActive = false
	}
	if c.PC() == c.pushEntry && !c.pushActive {
		if c.IFF1() {
			c.t.Fatal("PUSH entry with IRQ enabled")
		}
		c.pushActive = true
		c.pushSP = c.SP()
		c.pushReturn = uint16(c.machine.Memory.ReadByteInternal(c.SP())) | uint16(c.machine.Memory.ReadByteInternal(c.SP()+1))<<8
	}
	if c.pushActive && c.PC() == c.pushReturn && c.SP() == c.pushSP+2 {
		if c.IFF1() {
			c.t.Fatal("PUSH changed IFF")
		}
		c.pushActive = false
	}
	if c.PC() == c.draw && !c.active {
		c.active = true
		c.drawStart = c.machine.AbsoluteTStates()
		c.frame = byte(c.AF() >> 8)
		if c.frame != byte(c.completed&31) {
			c.t.Fatalf("frame order %d got %d", c.completed, c.frame)
		}
		c.callSP = c.SP()
		c.returnPC = uint16(c.machine.Memory.ReadByteInternal(c.SP())) | uint16(c.machine.Memory.ReadByteInternal(c.SP()+1))<<8
	}
	if c.active && c.PC() == c.returnPC {
		if c.SP() != c.callSP+2 {
			c.t.Fatal("draw PC/SP", c.PC(), c.SP())
		}
		elapsed := c.machine.AbsoluteTStates() - c.drawStart
		if elapsed > c.maxDrawT {
			c.maxDrawT = elapsed
		}
		c.onComplete(c.frame)
		c.completed++
		c.active = false
	}
	c.CPUCore.DoOpcode()
}
func TestHardwareSpriteChainFullMachine(t *testing.T) {
	for _, baseline := range []bool{true, false} {
		name := "dirty"
		path := "../../../reports/hardware-sprite-chain/demo/source.nanz"
		if baseline {
			name = "baseline"
			path = "../../../reports/advanced-demos/sprite-baseline.nanz"
		}
		t.Run(name, func(t *testing.T) { spriteChainMachineCheck(t, path, baseline) })
	}
}
func spriteChainMachineCheck(t *testing.T, sourcePath string, baseline bool) {
	src, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	rom, err := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if err != nil {
		t.Fatal(err)
	}
	// Independent image model; it does not inspect generated code or call an
	// address helper from the Nanz program.
	rows := []string{"00183c7e7e3c1800", "183c7effff7e3c18", "3c7effa5a5ff7e3c", "7effa55a5aa5ff7e", "7effa55a5aa5ff7e", "3c7effa5a5ff7e3c", "183c7effff7e3c18", "00183c7e7e3c1800"}
	// Independent legal displayed-frame hashes. No RenderFullScreen call: the
	// actual image below was fetched by the ULA while the CPU executed writes.
	legal := map[[32]byte]bool{}
	for frame := 0; frame < 32; frame++ {
		pixels := make([]byte, 256*192*4)
		for i := 3; i < len(pixels); i += 4 {
			pixels[i] = 255
		}
		for row, encoded := range rows {
			data, _ := hex.DecodeString(encoded)
			for col, b := range data {
				for bit := 0; bit < 8; bit++ {
					if b&(128>>bit) != 0 {
						x := (4+(frame&15)+col)*8 + bit
						y := 56 + (frame>>4)*8 + row
						offset := (y*256 + x) * 4
						pixels[offset], pixels[offset+1] = 255, 255
					}
				}
			}
		}
		legal[sha256.Sum256(pixels)] = true
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		for _, via := range []string{"compiled", "push"} {
			t.Run(mode+"/"+via, func(t *testing.T) {
				h, err := nanz.ParseWithOpts(string(src), "sprite-chain-machine", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
				if err != nil {
					t.Fatal(err)
				}
				steps, err := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
				if err != nil {
					t.Fatal(err)
				}
				assembly := "ORG 0x8000\n" + steps.Assembly
				a, err := z80asm.NewAssembler().AssembleString(assembly)
				if err != nil || len(a.Errors) > 0 {
					t.Fatal(err, a.Errors)
				}
				m, err := spectrum.New48K(rom)
				if err != nil {
					t.Fatal(err)
				}
				for i, b := range a.Binary {
					m.Memory.Write(0x8000+uint16(i), b, false)
				}
				if via == "push" {
					m.Memory.Write(uint16(a.Symbols["primitive"]), 1, false)
				}
				m.CPU.SetPC(uint16(a.Symbols["main"]))
				m.CPU.SetSP(0xeff0)
				m.CPU.SetIY(0x5c3a)
				m.CPU.SetIM(1)
				m.CPU.SetIFF1(false)
				m.CPU.SetIFF2(false)
				c := &spriteChainMachineObserver{CPUCore: m.CPU, t: t, machine: m, draw: uint16(a.Symbols["draw_frame"]), pushEntry: uint16(a.Symbols["actor_push"])}
				m.CPU = c
				save := func(name string, data []byte) {
					if d := os.Getenv("MINZ_HW_CHAIN_MACHINE_EXPORT"); d != "" {
						p := filepath.Join(d, t.Name()[len("TestHardwareSpriteChainFullMachine/"):], name)
						if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(p, data, 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
				c.onComplete = func(frame byte) {
					want := make([]byte, 6912)
					for i := 6144; i < len(want); i++ {
						want[i] = 0x46
					}
					x, y := 4+int(frame&15), 56+int(frame>>4)*8
					for row, s := range rows {
						data, _ := hex.DecodeString(s)
						scan := y + row
						addr := (scan&0xc0)*32 + (scan&7)*256 + (scan&0x38)*4 + x
						copy(want[addr:addr+8], data)
					}
					got := make([]byte, 6912)
					for i := range got {
						got[i] = m.Memory.ReadByteInternal(0x4000 + uint16(i))
					}
					if !bytes.Equal(got, want) {
						for i := range got {
							if got[i] != want[i] {
								t.Fatalf("frame %d screen[%04x]=%02x want %02x", frame, 0x4000+i, got[i], want[i])
							}
						}
					}
					save(fmt.Sprintf("completed-%02d.scr", c.completed), got)
				}
				displayed, broken := 0, 0
				for f := 0; f < 200 && c.completed < 64; f++ {
					m.RunFrame()
					if c.completed == 0 {
						continue
					} // Initialization is not an animation frame.
					pixels := make([]byte, 256*192*4)
					fb := m.ULA.Framebuffer()
					for y := 0; y < 192; y++ {
						start := ((y+m.Mode.BorderTop)*m.Mode.TotalPixelWidth + m.Mode.BorderLeft) * 4
						copy(pixels[y*256*4:(y+1)*256*4], fb[start:start+256*4])
					}
					displayed++
					if !legal[sha256.Sum256(pixels)] {
						broken++
					}
				}
				if baseline {
					if broken == 0 {
						t.Fatal("temporal oracle failed to detect baseline flicker")
					}
				} else {
					if broken != 0 {
						t.Fatalf("%d/%d displayed frames contain a partial actor", broken, displayed)
					}
					if c.maxDrawT >= int64(m.Mode.TStatesPerFrame()) {
						t.Fatalf("draw needs %d T, exceeds frame", c.maxDrawT)
					}
				}
				if c.completed != 64 || c.irq == 0 || c.unsafeIRQ != 0 {
					t.Fatalf("completed=%d IRQ=%d unsafe=%d", c.completed, c.irq, c.unsafeIRQ)
				}
				result, _ := json.MarshalIndent(map[string]any{"PASS": true, "mode": mode, "via": via, "completed": c.completed, "ROM_IRQ": c.irq, "unsafe_IRQ": c.unsafeIRQ, "video_frames": m.FrameCount(), "binary_bytes": len(a.Binary), "max_draw_T": c.maxDrawT, "displayed_frames": displayed, "partial_frames": broken}, "", "  ")
				save("result.json", result)
				save("program.a80", []byte(assembly))
				save("program.bin", a.Binary)
				if d := os.Getenv("MINZ_HW_CHAIN_MACHINE_EXPORT"); d != "" {
					if err := formats.SaveSNA(filepath.Join(d, t.Name()[len("TestHardwareSpriteChainFullMachine/"):], "running.sna"), m); err != nil {
						t.Fatal(err)
					}
				}
				t.Logf("%s %s: 64 screens, %d IRQ, %d video frames, max draw %d T, partial display %d/%d, PC/SP preserved", mode, via, c.irq, m.FrameCount(), c.maxDrawT, broken, displayed)
			})
		}
	}
}
