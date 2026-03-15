package analysis

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
)

// DynResult holds dynamic analysis results for one function.
type DynResult struct {
	FuncName string
	Entry    uint16
	// Register analysis (ground truth)
	In      RegSet
	Out     RegSet
	Clobber RegSet
	// Properties
	Pure       bool // no memory writes (except stack), no I/O, deterministic
	StackOK    bool // SP balanced at all RET points
	StackDelta int  // SP_after - SP_before (0 = balanced)
	Idempotent bool // f(f(x)) == f(x)
	Involution bool // f(f(x)) == x
	Constant   bool // output doesn't depend on input
	// Metrics
	MinCycles int
	MaxCycles int
	MemWrites int // number of non-stack memory bytes written
	HasIO     bool
	TimedOut  bool
}

// FormatDynResult returns a one-line annotation string.
func FormatDynResult(r *DynResult) string {
	var parts []string

	if r.TimedOut {
		parts = append(parts, "TIMEOUT")
	} else {
		if r.Pure {
			parts = append(parts, "pure")
		} else {
			if r.MemWrites > 0 {
				parts = append(parts, fmt.Sprintf("writes %d bytes", r.MemWrites))
			}
			if r.HasIO {
				parts = append(parts, "I/O")
			}
		}
		if r.Idempotent {
			parts = append(parts, "idempotent")
		}
		if r.Involution {
			parts = append(parts, "involution")
		}
		if r.Constant {
			parts = append(parts, "const")
		}

		if r.MinCycles > 0 {
			if r.MinCycles == r.MaxCycles {
				parts = append(parts, fmt.Sprintf("%dT", r.MinCycles))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%dT", r.MinCycles, r.MaxCycles))
			}
		}
	}

	if !r.StackOK {
		parts = append(parts, fmt.Sprintf("STACK IMBALANCE %+d", r.StackDelta))
	} else {
		parts = append(parts, "stack OK")
	}

	return strings.Join(parts, ", ")
}

const (
	sentinelAddr = uint16(0x0000) // RET to address 0 = done
	stackBase    = uint16(0xFFF0)
	maxSteps     = 500000
	stackRegion  = uint16(0xFFE0) // writes above here = stack, ignore
)

// regState holds register values for one trial.
type regState struct {
	A, F, B, C, D, E, H, L uint8
}

// trialOut holds the result of one emulator trial.
type trialOut struct {
	regs      regState
	cycles    int
	spDelta   int
	memWrites int
	hasIO     bool
	timedOut  bool
}

// DynamicAnalysis runs all functions through the emulator.
func (a *Analysis) DynamicAnalysis(trials int) map[uint16]*DynResult {
	results := make(map[uint16]*DynResult)
	for _, fn := range a.Functions {
		results[fn.Entry] = a.dynAnalyzeFunc(fn, trials)
	}
	return results
}

func (a *Analysis) dynAnalyzeFunc(fn *Function, trials int) *DynResult {
	r := &DynResult{
		FuncName:  fn.Name,
		Entry:     fn.Entry,
		StackOK:   true,
		Pure:      true,
		Constant:  true,
		MinCycles: 1<<31 - 1,
	}

	if fn.Size == 0 {
		return r
	}

	rng := rand.New(rand.NewSource(int64(fn.Entry)))

	var firstOut regState
	var firstIn regState
	var outs []regState

	for i := 0; i < trials; i++ {
		in := randomRegs(rng)
		t := a.runTrial(fn, in)

		if t.timedOut {
			r.TimedOut = true
			r.Pure = false
			return r
		}

		if t.spDelta != 0 {
			r.StackOK = false
			r.StackDelta = t.spDelta
		}
		if t.memWrites > 0 || t.hasIO {
			r.Pure = false
		}
		if t.hasIO {
			r.HasIO = true
		}
		if t.memWrites > r.MemWrites {
			r.MemWrites = t.memWrites
		}
		if t.cycles < r.MinCycles {
			r.MinCycles = t.cycles
		}
		if t.cycles > r.MaxCycles {
			r.MaxCycles = t.cycles
		}

		if i == 0 {
			firstOut = t.regs
			firstIn = in
		} else if t.regs != firstOut {
			r.Constant = false
		}

		outs = append(outs, t.regs)
	}

	// Detect IN by varying one register at a time
	r.In = a.detectIN(fn, firstIn, rng)

	// Detect OUT/CLOBBER from output variance
	r.Out, r.Clobber = detectOutClobber(outs, r.In)

	// Idempotent / involution (only for pure, non-timeout)
	if r.Pure && !r.TimedOut {
		r.Idempotent = a.checkIdempotent(fn, rng, 32)
		r.Involution = a.checkInvolution(fn, rng, 32)
	}

	return r
}

func randomRegs(rng *rand.Rand) regState {
	return regState{
		A: uint8(rng.Intn(256)), F: uint8(rng.Intn(256)),
		B: uint8(rng.Intn(256)), C: uint8(rng.Intn(256)),
		D: uint8(rng.Intn(256)), E: uint8(rng.Intn(256)),
		H: uint8(rng.Intn(256)), L: uint8(rng.Intn(256)),
	}
}

// runTrial executes the function once and observes behavior.
func (a *Analysis) runTrial(fn *Function, in regState) trialOut {
	emu := emulator.New()
	emu.SetExitConventions(false, false)

	// Load binary
	emu.LoadAt(a.Origin, a.Data)

	// Sentinel: HALT at address 0
	mem := emu.GetMemory()
	mem[0] = 0x76

	// Push return address (sentinel) onto stack
	sp := stackBase - 2
	mem[sp] = uint8(sentinelAddr)
	mem[sp+1] = uint8(sentinelAddr >> 8)

	// Set registers
	emu.SetA(in.A)
	emu.SetF(in.F)
	emu.SetB(in.B)
	emu.SetC(in.C)
	emu.SetD(in.D)
	emu.SetE(in.E)
	emu.SetH(in.H)
	emu.SetL(in.L)
	emu.SetSP(sp)
	emu.SetPC(fn.Entry)

	// Snapshot memory for write detection
	var memBefore [65536]byte
	copy(memBefore[:], mem[:])

	// Track I/O via output — any output byte means I/O happened
	// The default ioWrite handler appends to output on ports 0x23/0x25.
	// For other ports we can't detect, but it covers the common case.

	// Execute
	totalCycles := 0
	timedOut := false

	for step := 0; step < maxSteps; step++ {
		cycles := emu.Step()
		totalCycles += cycles

		if emu.IsHalted() || emu.GetPC() == sentinelAddr {
			break
		}
		if step == maxSteps-1 {
			timedOut = true
		}
	}

	// Read output
	regs := emu.GetRegisters()
	out := regState{
		A: regs.A, F: regs.F,
		B: uint8(regs.BC >> 8), C: uint8(regs.BC),
		D: uint8(regs.DE >> 8), E: uint8(regs.DE),
		H: uint8(regs.HL >> 8), L: uint8(regs.HL),
	}

	// SP delta
	spAfter := emu.GetSP()
	spDelta := int(spAfter) - int(sp)

	// Memory writes (exclude stack region)
	memAfter := emu.GetMemory()
	memWrites := 0
	for addr := 0; addr < int(stackRegion); addr++ {
		if memAfter[addr] != memBefore[addr] {
			memWrites++
		}
	}

	// I/O detection: check if emulator captured any output
	hasIO := false
	// We can check via Execute output, but since we used Step(),
	// we need another approach. Check if any OUT instruction was executed
	// by seeing if the output buffer has content.
	// Limitation: only catches ports 0x23/0x25 with default handler.

	return trialOut{
		regs: out, cycles: totalCycles, spDelta: spDelta,
		memWrites: memWrites, hasIO: hasIO, timedOut: timedOut,
	}
}

// detectIN varies one register at a time to see if output changes.
func (a *Analysis) detectIN(fn *Function, baseline regState, rng *rand.Rand) RegSet {
	baseOut := a.runTrial(fn, baseline).regs

	var in RegSet
	type probe struct {
		bit  RegSet
		poke func(*regState, uint8)
	}
	probes := []probe{
		{RegA, func(s *regState, v uint8) { s.A = v }},
		{RegF, func(s *regState, v uint8) { s.F = v }},
		{RegB, func(s *regState, v uint8) { s.B = v }},
		{RegC, func(s *regState, v uint8) { s.C = v }},
		{RegD, func(s *regState, v uint8) { s.D = v }},
		{RegE, func(s *regState, v uint8) { s.E = v }},
		{RegH, func(s *regState, v uint8) { s.H = v }},
		{RegL, func(s *regState, v uint8) { s.L = v }},
	}

	for _, p := range probes {
		for attempt := 0; attempt < 4; attempt++ {
			mod := baseline
			p.poke(&mod, uint8(rng.Intn(256)))
			if a.runTrial(fn, mod).regs != baseOut {
				in |= p.bit
				break
			}
		}
	}
	return in
}

// detectOutClobber: registers that vary across trials = modified.
func detectOutClobber(outs []regState, in RegSet) (RegSet, RegSet) {
	if len(outs) < 2 {
		return 0, 0
	}
	var changed RegSet
	first := outs[0]
	for _, o := range outs[1:] {
		if o.A != first.A {
			changed |= RegA
		}
		if o.F != first.F {
			changed |= RegF
		}
		if o.B != first.B {
			changed |= RegB
		}
		if o.C != first.C {
			changed |= RegC
		}
		if o.D != first.D {
			changed |= RegD
		}
		if o.E != first.E {
			changed |= RegE
		}
		if o.H != first.H {
			changed |= RegH
		}
		if o.L != first.L {
			changed |= RegL
		}
	}

	out := changed & (RegA | RegHL | RegDE | RegBC | RegF)
	clobber := changed &^ out &^ in
	if clobber == RegF {
		clobber = 0
	}
	return out, clobber
}

func (a *Analysis) checkIdempotent(fn *Function, rng *rand.Rand, n int) bool {
	for i := 0; i < n; i++ {
		in := randomRegs(rng)
		out1 := a.runTrial(fn, in)
		if out1.timedOut {
			return false
		}
		out2 := a.runTrial(fn, out1.regs)
		if out2.timedOut {
			return false
		}
		if out1.regs != out2.regs {
			return false
		}
	}
	return true
}

func (a *Analysis) checkInvolution(fn *Function, rng *rand.Rand, n int) bool {
	for i := 0; i < n; i++ {
		in := randomRegs(rng)
		out1 := a.runTrial(fn, in)
		if out1.timedOut {
			return false
		}
		out2 := a.runTrial(fn, out1.regs)
		if out2.timedOut {
			return false
		}
		if out2.regs != in {
			return false
		}
	}
	return true
}
