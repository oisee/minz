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

type rotoMachineRegisters struct{ AF, BC, DE, HL, IX, IY, AFAlt, BCAlt, DEAlt, HLAlt uint16 }

func rotoMachineRegs(c spectrum.CPUCore) rotoMachineRegisters {
	return rotoMachineRegisters{c.AF(), c.BC(), c.DE(), c.HL(), c.IX(), c.IY(), c.AF_(), c.BC_(), c.DE_(), c.HL_()}
}

// Observes ordinary CPU execution; it neither intercepts ROM services nor
// changes CPU/memory state. Accepted IRQs execute the embedded 48K ROM.
type rotoMachineObserver struct {
	spectrum.CPUCore
	t                                               *testing.T
	machine                                         *spectrum.Machine
	draw                                            uint16
	drawing                                         bool
	argument                                        byte
	drawSP, drawReturn                              uint16
	completed, acceptedIRQ, drawingIRQ, returnedIRQ int
	irqActive                                       bool
	irqSP, irqPC                                    uint16
	irqRegs                                         rotoMachineRegisters
	onComplete                                      func(byte, uint16, int64)
}

func (c *rotoMachineObserver) Interrupt() {
	if c.IFF1() {
		if c.irqActive {
			c.t.Fatal("nested ROM IRQ before return")
		}
		c.acceptedIRQ++
		if c.drawing {
			c.drawingIRQ++
		}
		c.irqActive = true
		c.irqSP, c.irqPC = c.SP(), c.PC()
		if c.Halted() {
			c.irqPC++
		}
		c.irqRegs = rotoMachineRegs(c.CPUCore)
	}
	c.CPUCore.Interrupt()
}
func (c *rotoMachineObserver) DoOpcode() {
	if c.irqActive && c.PC() == c.irqPC && c.SP() == c.irqSP {
		if got := rotoMachineRegs(c.CPUCore); got != c.irqRegs {
			c.t.Fatalf("ROM IRQ failed to preserve main/alternate registers: got %+v want %+v", got, c.irqRegs)
		}
		if !c.IFF1() {
			c.t.Fatal("ROM IRQ returned with interrupts disabled")
		}
		c.returnedIRQ++
		c.irqActive = false
	}
	if c.PC() == c.draw && !c.drawing {
		c.drawing = true
		c.argument = byte(c.AF() >> 8)
		if c.argument != byte(c.completed&31) {
			c.t.Fatalf("main frame order: call %d argument %d", c.completed, c.argument)
		}
		c.drawSP = c.SP()
		c.drawReturn = uint16(c.machine.Memory.ReadByteInternal(c.drawSP)) | uint16(c.machine.Memory.ReadByteInternal(c.drawSP+1))<<8
	}
	if c.drawing && c.PC() == c.drawReturn {
		if c.SP() != c.drawSP+2 {
			c.t.Fatalf("draw_frame return PC %04x SP %04x want %04x", c.PC(), c.SP(), c.drawSP+2)
		}
		c.onComplete(c.argument, c.SP(), c.machine.AbsoluteTStates())
		c.completed++
		c.drawing = false
	}
	c.CPUCore.DoOpcode()
}

// Runs the generated main, rather than calling draw_frame from the host.
// This is a 48K Spectrum emulator proof with actual ROM IRQ handling and ULA
// contention, not a physical-hardware measurement or a tear-free guarantee.
func TestZXHalfCellRotozoomerFullMachine(t *testing.T) {
	src, err := os.ReadFile("../../../reports/zx-rotozoomer-kernel/source.nanz")
	if err != nil {
		t.Fatal(err)
	}
	rom, err := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if err != nil {
		t.Fatal(err)
	}
	if len(rom) != 16384 {
		t.Fatalf("48K ROM length %d", len(rom))
	}
	var model struct {
		A       []int  `json:"axis_a"`
		B       []int  `json:"axis_b"`
		Texture []byte `json:"texture"`
	}
	data, err := os.ReadFile("../../../reports/zx-rotozoomer-32x48/model.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	if len(model.A) != 32 || len(model.B) != 32 || len(model.Texture) != 256 {
		t.Fatal("independent model dimensions")
	}
	save := func(mode, name string, data []byte) {
		if root := os.Getenv("MINZ_ZX_ROTO_MACHINE_EXPORT"); root != "" {
			path := filepath.Join(root, mode, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, err := nanz.Parse(string(src), "halfcell-machine")
			if err != nil {
				t.Fatal(err)
			}
			steps, err := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if err != nil {
				t.Fatal(err)
			}
			asm := "ORG 0x8000\n" + steps.Assembly
			result, err := z80asm.NewAssembler().AssembleString(asm)
			if err != nil || len(result.Errors) > 0 {
				t.Fatal(err, result.Errors)
			}
			main, mainOK := result.Symbols["main"]
			draw, drawOK := result.Symbols["draw_frame"]
			if !mainOK || !drawOK {
				t.Fatal("missing main/draw_frame entry")
			}
			machine, err := spectrum.New48K(rom)
			if err != nil {
				t.Fatal(err)
			}
			for i, b := range result.Binary {
				machine.Memory.Write(0x8000+uint16(i), b, false)
			}
			machine.CPU.SetPC(uint16(main))
			machine.CPU.SetSP(0xeff0)
			machine.CPU.SetIY(0x5c3a) // Standard ROM system-variable base.
			machine.CPU.SetIM(1)
			machine.CPU.SetIFF1(false)
			machine.CPU.SetIFF2(false)
			observer := &rotoMachineObserver{CPUCore: machine.CPU, t: t, machine: machine, draw: uint16(draw)}
			machine.CPU = observer
			type completion struct {
				Frame byte   `json:"frame"`
				T     int64  `json:"absolute_t_states"`
				SP    uint16 `json:"return_sp"`
			}
			completions := []completion{}
			observer.onComplete = func(frame byte, sp uint16, absoluteT int64) {
				// Both iterations must return to the same live main stack frame.
				if sp != 0xefee {
					t.Fatalf("main call stack drift: SP %04x want efee", sp)
				}
				want := make([]byte, 6912)
				for i := 0; i < 6144; i++ {
					if (0x4000+i)&0x400 != 0 {
						want[i] = 255
					}
				}
				a, b := model.A[frame], model.B[frame]
				ha, hb := a/2, b/2 // Nanz signed truncation toward zero.
				sample := func(u, v int) byte { return model.Texture[(int(uint16(v)>>8)&15)*16+(int(uint16(u)>>8)&15)] }
				for y := 0; y < 24; y++ {
					for x := 0; x < 32; x++ {
						u := 32768 - a*16 - b*12 + x*a + y*hb*2
						v := 32768 + b*16 - a*12 - x*b + y*ha*2
						want[6144+y*32+x] = 64 | sample(u, v)<<3 | sample(u+hb, v+ha)
					}
				}
				actual := make([]byte, 6912)
				for i := range actual {
					actual[i] = machine.Memory.ReadByteInternal(0x4000 + uint16(i))
				}
				if !bytes.Equal(actual, want) {
					for i := range actual {
						if actual[i] != want[i] {
							t.Fatalf("completed frame %d address %04x got %02x want %02x", frame, 0x4000+i, actual[i], want[i])
						}
					}
				}
				completions = append(completions, completion{frame, absoluteT, sp})
				save(mode, fmt.Sprintf("completed-%02d-frame-%02d.scr", observer.completed, frame), actual)
			}
			for videoFrame := 0; videoFrame < 400 && observer.completed < 64; videoFrame++ {
				machine.RunFrame()
			}
			if observer.completed != 64 {
				t.Fatalf("only %d completed draws in %d video frames; PC=%04x SP=%04x", observer.completed, machine.FrameCount(), observer.PC(), observer.SP())
			}
			if observer.acceptedIRQ == 0 || observer.drawingIRQ == 0 || observer.returnedIRQ == 0 {
				t.Fatalf("missing actual ROM IRQ execution: accepted %d during draw %d returned %d", observer.acceptedIRQ, observer.drawingIRQ, observer.returnedIRQ)
			}
			// ROM FRAMES is updated by the real interrupt handler at $0038.
			frames := uint32(machine.Memory.ReadByteInternal(0x5c78)) | uint32(machine.Memory.ReadByteInternal(0x5c79))<<8 | uint32(machine.Memory.ReadByteInternal(0x5c7a))<<16
			if frames != uint32(observer.acceptedIRQ) {
				t.Fatalf("ROM FRAMES=%d accepted IRQ=%d", frames, observer.acceptedIRQ)
			}
			romSHA := sha256.Sum256(rom)
			binarySHA := sha256.Sum256(result.Binary)
			summary := map[string]any{"PASS": true, "completed_draws": observer.completed, "video_frames": machine.FrameCount(), "accepted_rom_irq": observer.acceptedIRQ, "irq_during_draw": observer.drawingIRQ, "returned_rom_irq": observer.returnedIRQ, "rom_sha256": hex.EncodeToString(romSHA[:]), "binary_sha256": hex.EncodeToString(binarySHA[:]), "binary_bytes": len(result.Binary), "completions": completions, "scope": "actual generated main; 48K ROM; IM1 IRQ; ULA contention; 64 complete screens; two frame-order cycles; draw PC/SP; ROM IRQ main and alternate register preservation"}
			encoded, err := json.MarshalIndent(summary, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			save(mode, "result.json", encoded)
			save(mode, "program.bin", result.Binary)
			save(mode, "program.a80", []byte(asm))
			if root := os.Getenv("MINZ_ZX_ROTO_MACHINE_EXPORT"); root != "" {
				if err := formats.SaveSNA(filepath.Join(root, mode, "running.sna"), machine); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("%s: 64 full main draws, %d video frames, %d ROM IRQ (%d during draw), all screens/PC/SP/registers PASS", mode, machine.FrameCount(), observer.acceptedIRQ, observer.drawingIRQ)
		})
	}
}
