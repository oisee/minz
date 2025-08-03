package tas

import (
	"testing"
	"fmt"
)

// MockZ80 simulates Z80 for testing
type MockZ80 struct {
	pc, sp      uint16
	a, b, c, d, e, f, h, l byte
	a_, b_, c_, d_, e_, f_, h_, l_ byte
	ix, iy      uint16
	i, r        byte
	iff1, iff2  bool
	memory      [65536]byte
	cycles      uint64
	tstates     uint64
	border      byte
}

// Implement Z80Emulator interface
func (m *MockZ80) GetPC() uint16 { return m.pc }
func (m *MockZ80) SetPC(v uint16) { m.pc = v }
func (m *MockZ80) GetSP() uint16 { return m.sp }
func (m *MockZ80) SetSP(v uint16) { m.sp = v }
func (m *MockZ80) GetA() byte { return m.a }
func (m *MockZ80) SetA(v byte) { m.a = v }
func (m *MockZ80) GetB() byte { return m.b }
func (m *MockZ80) SetB(v byte) { m.b = v }
func (m *MockZ80) GetC() byte { return m.c }
func (m *MockZ80) SetC(v byte) { m.c = v }
func (m *MockZ80) GetD() byte { return m.d }
func (m *MockZ80) SetD(v byte) { m.d = v }
func (m *MockZ80) GetE() byte { return m.e }
func (m *MockZ80) SetE(v byte) { m.e = v }
func (m *MockZ80) GetF() byte { return m.f }
func (m *MockZ80) SetF(v byte) { m.f = v }
func (m *MockZ80) GetH() byte { return m.h }
func (m *MockZ80) SetH(v byte) { m.h = v }
func (m *MockZ80) GetL() byte { return m.l }
func (m *MockZ80) SetL(v byte) { m.l = v }
func (m *MockZ80) GetIX() uint16 { return m.ix }
func (m *MockZ80) SetIX(v uint16) { m.ix = v }
func (m *MockZ80) GetIY() uint16 { return m.iy }
func (m *MockZ80) SetIY(v uint16) { m.iy = v }
func (m *MockZ80) GetI() byte { return m.i }
func (m *MockZ80) SetI(v byte) { m.i = v }
func (m *MockZ80) GetR() byte { return m.r }
func (m *MockZ80) SetR(v byte) { m.r = v }
func (m *MockZ80) GetIFF1() bool { return m.iff1 }
func (m *MockZ80) SetIFF1(v bool) { m.iff1 = v }
func (m *MockZ80) GetIFF2() bool { return m.iff2 }
func (m *MockZ80) SetIFF2(v bool) { m.iff2 = v }
func (m *MockZ80) GetShadowA() byte { return m.a_ }
func (m *MockZ80) SetShadowA(v byte) { m.a_ = v }
func (m *MockZ80) GetShadowB() byte { return m.b_ }
func (m *MockZ80) SetShadowB(v byte) { m.b_ = v }
func (m *MockZ80) GetShadowC() byte { return m.c_ }
func (m *MockZ80) SetShadowC(v byte) { m.c_ = v }
func (m *MockZ80) GetShadowD() byte { return m.d_ }
func (m *MockZ80) SetShadowD(v byte) { m.d_ = v }
func (m *MockZ80) GetShadowE() byte { return m.e_ }
func (m *MockZ80) SetShadowE(v byte) { m.e_ = v }
func (m *MockZ80) GetShadowF() byte { return m.f_ }
func (m *MockZ80) SetShadowF(v byte) { m.f_ = v }
func (m *MockZ80) GetShadowH() byte { return m.h_ }
func (m *MockZ80) SetShadowH(v byte) { m.h_ = v }
func (m *MockZ80) GetShadowL() byte { return m.l_ }
func (m *MockZ80) SetShadowL(v byte) { m.l_ = v }
func (m *MockZ80) GetCycles() uint64 { return m.cycles }
func (m *MockZ80) SetCycles(v uint64) { m.cycles = v }
func (m *MockZ80) GetTStates() uint64 { return m.tstates }
func (m *MockZ80) SetTStates(v uint64) { m.tstates = v }
func (m *MockZ80) GetMemory() []byte { return m.memory[:] }
func (m *MockZ80) SetMemory(mem []byte) { copy(m.memory[:], mem) }
func (m *MockZ80) ReadByte(addr uint16) byte { return m.memory[addr] }
func (m *MockZ80) WriteByte(addr uint16, val byte) { m.memory[addr] = val }
func (m *MockZ80) GetBorder() byte { return m.border }
func (m *MockZ80) GetLastOpcode() string { return "NOP" }
func (m *MockZ80) GetRegisters() *CPURegisters {
	return &CPURegisters{
		PC: m.pc, SP: m.sp,
		A: m.a, B: m.b, C: m.c, D: m.d, E: m.e, F: m.f, H: m.h, L: m.l,
		A_: m.a_, B_: m.b_, C_: m.c_, D_: m.d_, E_: m.e_, F_: m.f_, H_: m.h_, L_: m.l_,
		IX: m.ix, IY: m.iy, I: m.i, R: m.r,
		IFF1: m.iff1, IFF2: m.iff2,
	}
}

func TestTASDebuggerBasic(t *testing.T) {
	// Create mock emulator
	emu := &MockZ80{
		pc: 0x8000,
		sp: 0xFFFE,
		a:  42,
	}
	
	// Create TAS debugger
	tas := NewTASDebugger(emu)
	tas.recording = true
	
	// Record some frames
	for i := 0; i < 100; i++ {
		emu.pc++
		emu.cycles += 4
		tas.RecordFrame()
	}
	
	// Test we recorded 100 frames
	if len(tas.stateHistory) != 100 {
		t.Errorf("Expected 100 frames, got %d", len(tas.stateHistory))
	}
	
	// Test rewind
	err := tas.Rewind(50)
	if err != nil {
		t.Errorf("Rewind failed: %v", err)
	}
	
	if tas.currentFrame != 50 {
		t.Errorf("Expected frame 50 after rewind, got %d", tas.currentFrame)
	}
	
	fmt.Println("✅ TAS basic recording and rewind works!")
}

func TestTASSaveStates(t *testing.T) {
	emu := &MockZ80{pc: 0x8000}
	tas := NewTASDebugger(emu)
	tas.recording = true
	
	// Record and save
	tas.RecordFrame()
	tas.SaveState("checkpoint1")
	
	// Change state
	emu.pc = 0x9000
	tas.RecordFrame()
	
	// Load state
	err := tas.LoadState("checkpoint1")
	if err != nil {
		t.Errorf("LoadState failed: %v", err)
	}
	
	if emu.pc != 0x8000 {
		t.Errorf("Expected PC=0x8000 after load, got 0x%04X", emu.pc)
	}
	
	fmt.Println("✅ Save states work!")
}

func TestTASSMCTracking(t *testing.T) {
	emu := &MockZ80{pc: 0x8000}
	tas := NewTASDebugger(emu)
	
	// Track SMC event
	tas.TrackSMC(0x8000, 0x8100, 0x00, 0x42)
	
	if len(tas.smcEvents) != 1 {
		t.Errorf("Expected 1 SMC event, got %d", len(tas.smcEvents))
	}
	
	event := tas.smcEvents[0]
	if event.NewValue != 0x42 {
		t.Errorf("Expected new value 0x42, got 0x%02X", event.NewValue)
	}
	
	fmt.Println("✅ SMC tracking works!")
}

func TestTASOptimizationHunt(t *testing.T) {
	emu := &MockZ80{pc: 0x8000}
	tas := NewTASDebugger(emu)
	tas.recording = true
	
	// Start hunt
	goal := OptimizationGoal{
		TargetPC:  0x9000,
		MaxCycles: 1000,
	}
	tas.StartOptimizationHunt(goal)
	
	// Simulate reaching goal
	emu.pc = 0x9000
	emu.cycles = 800
	tas.RecordFrame()
	tas.CheckOptimizationGoal()
	
	if tas.bestPath == nil {
		t.Errorf("Expected best path to be recorded")
	}
	
	fmt.Println("✅ Optimization hunting works!")
}

func TestTASTimeline(t *testing.T) {
	emu := &MockZ80{pc: 0x8000}
	tas := NewTASDebugger(emu)
	tas.recording = true
	
	// Record frames with events
	for i := 0; i < 10; i++ {
		tas.RecordFrame()
		if i == 5 {
			tas.RecordInput(0xFE, 0x1F) // Key press
		}
		if i == 7 {
			tas.TrackSMC(0x8000, 0x8100, 0x00, 0x42)
		}
	}
	
	timeline := tas.GetTimeline()
	if timeline == "No history recorded" {
		t.Errorf("Expected timeline, got empty")
	}
	
	fmt.Println("✅ Timeline generation works!")
	fmt.Println("Timeline:", timeline)
}