package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
	"github.com/minz/minzc/pkg/z80asm"
)

type typedSpriteSceneObserver struct {
	spectrum.CPUCore
	t                                                                  *testing.T
	m                                                                  *spectrum.Machine
	prepare, draw, frameAddress                                        uint16
	preparing, drawing, irqActive                                      bool
	prepareReturn, prepareSP, drawReturn, drawSP                       uint16
	irqPC, irqSP                                                       uint16
	irqRegs                                                            rotoMachineRegisters
	prepareIFF                                                         bool
	beforeScreen                                                       []byte
	prepareStart, drawStart, lastPrepare, maxPrepare, maxDraw, maxBusy int64
	completed, irq                                                     int
	onDraw                                                             func(byte)
}

func (c *typedSpriteSceneObserver) Interrupt() {
	if c.IFF1() {
		if c.IY() != 0x5c3a {
			c.t.Fatal("ROM IRQ needs system-variable IY")
		}
		if c.drawing {
			c.t.Fatal("IRQ accepted during publication")
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
func (c *typedSpriteSceneObserver) screen() []byte {
	b := make([]byte, 6912)
	for i := range b {
		b[i] = c.m.Memory.ReadByteInternal(0x4000 + uint16(i))
	}
	return b
}
func (c *typedSpriteSceneObserver) returnAddress() uint16 {
	return uint16(c.m.Memory.ReadByteInternal(c.SP())) | uint16(c.m.Memory.ReadByteInternal(c.SP()+1))<<8
}
func (c *typedSpriteSceneObserver) DoOpcode() {
	if c.irqActive && c.PC() == c.irqPC && c.SP() == c.irqSP {
		if rotoMachineRegs(c.CPUCore) != c.irqRegs {
			c.t.Fatal("ROM IRQ register preservation")
		}
		c.irqActive = false
	}
	if c.PC() == c.prepare && !c.preparing {
		c.preparing = true
		c.prepareReturn, c.prepareSP = c.returnAddress(), c.SP()
		c.prepareStart = c.m.AbsoluteTStates()
		c.prepareIFF = c.IFF1()
		c.beforeScreen = c.screen()
	}
	if c.preparing && c.PC() == c.prepareReturn {
		if c.SP() != c.prepareSP+2 || c.IFF1() != c.prepareIFF {
			c.t.Fatal("prepare PC/SP/IFF")
		}
		if !bytes.Equal(c.beforeScreen, c.screen()) {
			c.t.Fatal("preparation wrote screen memory")
		}
		c.lastPrepare = c.m.AbsoluteTStates() - c.prepareStart
		if c.lastPrepare > c.maxPrepare {
			c.maxPrepare = c.lastPrepare
		}
		c.preparing = false
	}
	if c.PC() == c.draw && !c.drawing {
		if c.IFF1() {
			c.t.Fatal("publication starts with INT enabled")
		}
		c.drawing = true
		c.drawReturn, c.drawSP = c.returnAddress(), c.SP()
		c.drawStart = c.m.AbsoluteTStates()
		if got := c.m.Memory.ReadByteInternal(c.frameAddress); got != byte(c.completed&31) {
			c.t.Fatalf("scene order: got %d want %d", got, c.completed&31)
		}
	}
	if c.drawing && c.PC() == c.drawReturn {
		if c.SP() != c.drawSP+2 || c.IFF1() {
			c.t.Fatal("publication PC/SP/IFF")
		}
		elapsed := c.m.AbsoluteTStates() - c.drawStart
		if elapsed > c.maxDraw {
			c.maxDraw = elapsed
		}
		if busy := c.lastPrepare + elapsed; busy > c.maxBusy {
			c.maxBusy = busy
		}
		c.onDraw(byte(c.completed & 31))
		c.completed++
		c.drawing = false
	}
	c.CPUCore.DoOpcode()
}

// Expected pixels come from the archived byte pattern, not the new descriptor
// or generated address helpers. Inspect actual ULA fetches, not RenderFullScreen.
func TestZXTypedSpriteSceneMachine(t *testing.T) {
	runZXTypedSpriteSceneMachine(t, "typed-sprite-scene", "MINZ_TYPED_SPRITE_EXPORT", func(frame, row int) int { return 0 })
}

func runZXTypedSpriteSceneMachine(t *testing.T, scene, exportEnv string, rowShift func(frame, row int) int, pixelUnits ...bool) {
	src, err := os.ReadFile("../../../reports/advanced-demos/" + scene + "/source.nanz")
	if err != nil {
		t.Fatal(err)
	}
	runZXTypedSpriteSceneSourceMachine(t, src, exportEnv, rowShift, pixelUnits...)
}

func runZXTypedSpriteSceneSourceMachine(t *testing.T, src []byte, exportEnv string, rowShift func(frame, row int) int, pixelUnits ...bool) {
	t.Helper()
	rom, err := os.ReadFile("../../cmd/mzx/roms/48.rom")
	if err != nil {
		t.Fatal(err)
	}
	rows := []string{"00183c7e7e3c1800", "183c7effff7e3c18", "3c7effa5a5ff7e3c", "7effa55a5aa5ff7e", "7effa55a5aa5ff7e", "3c7effa5a5ff7e3c", "183c7effff7e3c18", "00183c7e7e3c1800"}
	pixelShift := len(pixelUnits) > 0 && pixelUnits[0]
	baseX := func(frame, row int) int {
		if pixelShift {
			return 32 + (frame & 15) + rowShift(frame, row)
		}
		return (4 + (frame & 15) + rowShift(frame, row)) * 8
	}
	expectedScreen := func(frame int) []byte {
		want := make([]byte, 6912)
		for i := 6144; i < len(want); i++ {
			want[i] = 0x46
		}
		for row, encoded := range rows {
			data, _ := hex.DecodeString(encoded)
			y := 56 + (frame>>4)*8 + row
			for col, b := range data {
				for bit := 0; bit < 8; bit++ {
					if b&(128>>bit) == 0 {
						continue
					}
					x := baseX(frame, row) + col*8 + bit
					address := (y/64)*2048 + (y%8)*256 + ((y/8)%8)*32 + x/8
					want[address] |= 128 >> uint(x%8)
				}
			}
		}
		return want
	}
	legal := map[[32]byte]bool{}
	var poseHashes [32][32]byte
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
						x := baseX(frame, row) + col*8 + bit
						y := 56 + (frame>>4)*8 + row
						offset := (y*256 + x) * 4
						pixels[offset], pixels[offset+1] = 255, 255
					}
				}
			}
		}
		hash := sha256.Sum256(pixels)
		legal[hash] = true
		poseHashes[frame] = hash
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, err := nanz.ParseWithOpts(string(src), "typed-sprite-scene", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			steps, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if err != nil {
				t.Fatal(err)
			}
			rawHIR, err := nanz.ParseWithOpts(string(src), "typed-sprite-scene-raw", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			for _, module := range []*mir2.Module{hir.LowerModule(rawHIR), steps.MIR2Module} {
				vm := mir2.NewVM(module)
				vm.EnsureHeap(65536)
				initial := make([]byte, 6912)
				for i := 6144; i < len(initial); i++ {
					initial[i] = 0x46
				}
				vm.WriteHeapSlice(0x4000, initial)
				rejected, err := vm.Call("draw_frame", nil)
				if err != nil || len(rejected) != 1 || rejected[0].I != 0 {
					t.Fatal("unprepared VM publication", rejected, err)
				}
				if !bytes.Equal(initial, vm.ReadHeap(0x4000, 6912)) {
					t.Fatal("unprepared VM publication wrote screen")
				}

				for frame := 0; frame < 64; frame++ {
					pose := frame & 31
					vm.WriteHeapSlice(vm.GlobalAddr("frame"), []byte{byte(pose)})
					before := append([]byte(nil), vm.ReadHeap(0x4000, 6912)...)
					prepared, err := vm.Call("prepare_frame", []mir2.Value{{I: int64(pose)}})
					if err != nil || len(prepared) != 1 || prepared[0].I != 1 {
						t.Fatal("VM preparation", pose, prepared, err)
					}
					if !bytes.Equal(before, vm.ReadHeap(0x4000, 6912)) {
						t.Fatal("VM preparation wrote screen", pose)
					}
					drawn, err := vm.Call("draw_frame", nil)
					if err != nil || len(drawn) != 1 || drawn[0].I != 1 {
						t.Fatal("VM publication", pose, drawn, err)
					}
					if !bytes.Equal(vm.ReadHeap(0x4000, 6912), expectedScreen(pose)) {
						t.Fatal("VM scene bitmap/attributes", pose)
					}
				}
			}
			t.Log("128 successive raw/final VM scenes: exact bitmap/attributes and preparation without screen writes")
			assembly := "ORG 0x8000\n" + steps.Assembly
			a, err := z80asm.NewAssembler().AssembleString(assembly)
			if err != nil || len(a.Errors) != 0 {
				t.Fatal(err, a.Errors)
			}
			m, err := spectrum.New48K(rom)
			if err != nil {
				t.Fatal(err)
			}
			for i, b := range a.Binary {
				m.Memory.Write(0x8000+uint16(i), b, false)
			}
			m.CPU.SetPC(uint16(a.Symbols["main"]))
			m.CPU.SetSP(0xeff0)
			m.CPU.SetIY(0x5c3a)
			m.CPU.SetIM(1)
			m.CPU.SetIFF1(false)
			m.CPU.SetIFF2(false)
			for _, name := range []string{"main", "prepare_frame", "draw_frame", "frame"} {
				if _, ok := a.Symbols[name]; !ok {
					t.Fatal("missing symbol", name)
				}
			}
			c := &typedSpriteSceneObserver{CPUCore: m.CPU, t: t, m: m, prepare: uint16(a.Symbols["prepare_frame"]), draw: uint16(a.Symbols["draw_frame"]), frameAddress: uint16(a.Symbols["frame"])}
			// Probe the refused call before main starts: no prepared operands,
			// no screen writes, and the caller's disabled INT state is preserved.
			before := c.screen()
			m.Memory.Write(0xeff0, 0, false)
			m.Memory.Write(0xeff1, 0x70, false)
			m.CPU.SetPC(uint16(a.Symbols["draw_frame"]))
			refused := false
			for i := 0; i < 1000; i++ {
				if m.CPU.PC() == 0x7000 && m.CPU.SP() == 0xeff2 {
					refused = true
					break
				}
				m.CPU.DoOpcode()
			}
			if !refused || m.CPU.IFF1() || byte(m.CPU.AF()>>8) != 0 || !bytes.Equal(before, c.screen()) {
				t.Fatal("unprepared native publication PC/SP/IFF/screen")
			}
			m.CPU.SetPC(uint16(a.Symbols["main"]))
			m.CPU.SetSP(0xeff0)
			m.CPU = c
			c.onDraw = func(frame byte) {
				if !bytes.Equal(c.screen(), expectedScreen(int(frame))) {
					for at, got := range c.screen() {
						if want := expectedScreen(int(frame))[at]; got != want {
							t.Fatalf("completed scene %d bitmap/attributes at %04x: got %02x want %02x", frame, 0x4000+at, got, want)
						}
					}
				}
			}

			displayed := 0
			seen := map[[32]byte]bool{}
			for frame := 0; frame < 200 && c.completed < 64; frame++ {
				m.RunFrame()
				if c.completed == 0 {
					continue
				}
				pixels := make([]byte, 256*192*4)
				fb := m.ULA.Framebuffer()
				for y := 0; y < 192; y++ {
					start := ((y+m.Mode.BorderTop)*m.Mode.TotalPixelWidth + m.Mode.BorderLeft) * 4
					copy(pixels[y*256*4:(y+1)*256*4], fb[start:start+256*4])
				}
				hash := sha256.Sum256(pixels)
				if !legal[hash] {
					t.Fatalf("partial ULA scene at video frame %d", m.FrameCount())
				}
				if hash != poseHashes[(c.completed-1)&31] {
					t.Fatal("ULA pose order does not match completed scene", c.completed)
				}
				seen[hash] = true
				displayed++
			}
			if c.completed != 64 || displayed != 64 || len(seen) != 32 || c.irq == 0 {
				t.Fatalf("completed=%d displayed=%d unique=%d IRQ=%d", c.completed, displayed, len(seen), c.irq)
			}
			if c.maxBusy >= int64(m.Mode.TStatesPerFrame()) {
				t.Fatalf("prepare+publish needs %d T", c.maxBusy)
			}
			if root := os.Getenv(exportEnv); root != "" {
				dir := filepath.Join(root, mode)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				result, _ := json.MarshalIndent(map[string]any{"PASS": true, "binary_bytes": len(a.Binary), "completed": c.completed, "displayed": displayed, "unique_poses": len(seen), "ROM_IRQ": c.irq, "max_prepare_T": c.maxPrepare, "max_publish_T": c.maxDraw, "max_busy_T": c.maxBusy, "raw_final_VM": true, "VM_frames_per_module": 64, "preparation_preserves_screen": true, "PC_SP_IFF": true}, "", "  ")
				for name, data := range map[string][]byte{"source.nanz": src, "program.a80": []byte(assembly), "program.bin": a.Binary, "result.json": result, "screen.scr": c.screen()} {
					if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
				if err := formats.SaveSNA(filepath.Join(dir, "running.sna"), m); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("%s: 64 native/ULA scenes; prepare<=%d T, publish<=%d T, busy<=%d T, module=%d bytes", mode, c.maxPrepare, c.maxDraw, c.maxBusy, len(a.Binary))
		})
	}
}
