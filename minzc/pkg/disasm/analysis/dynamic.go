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
	// Metrics.
	//
	// MinCycles/MaxCycles are the observed T-state cost between entry and
	// return. They are currently always 0: RemogattoZ80.Step() reports no
	// cycles because the remogatto CPU's Tstates counter is never advanced by
	// this package's memory accessor, so there is nothing to measure. The
	// annotation omits the figure rather than printing a zero. Static
	// per-instruction costs remain available through `mzd --cycles`.
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
	stackBase   = uint16(0xFFF0)
	maxSteps    = 500000
	stackRegion = uint16(0xFFE0) // writes above here = stack, ignore

	// trampolineAddr is where the register-priming prologue is assembled. It
	// sits just below the stack; a program image that reaches this high will
	// overlap it, but such an image leaves no room for the harness anyway.
	trampolineAddr = uint16(0xFF80)
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
		MinCycles: 1<<31 - 1,
	}

	if fn.Size == 0 {
		return r
	}

	rng := rand.New(rand.NewSource(int64(fn.Entry)))

	var firstIn regState
	var ins []regState
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
			firstIn = in
		}

		ins = append(ins, in)
		outs = append(outs, t.regs)
	}

	// Detect IN by varying one register at a time
	r.In = a.detectIN(fn, firstIn, rng)

	// Detect OUT/CLOBBER from output variance
	r.Out, r.Clobber = detectOutClobber(ins, outs, r.In)

	r.Constant = detectConstant(ins, outs)

	// Idempotent / involution, compared only over the registers the function
	// actually returns. Comparing whole register states made these meaningless:
	// clobbered registers carry unrelated input values, so real idempotent
	// functions read as not idempotent — and functions with no output at all
	// (RST vectors, stubs) satisfied both predicates vacuously.
	if r.Pure && !r.TimedOut && r.Out != 0 {
		r.Idempotent = a.checkIdempotent(fn, rng, 32, r.Out)
		r.Involution = a.checkInvolution(fn, rng, 32, r.Out)
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
//
// Execution uses RemogattoZ80, the FUSE-verified core behind mze. The other
// emulator in this package implements about fifty opcodes and silently treats
// everything else as NOP, which made every observation here meaningless: a
// function whose body the emulator did not understand appeared to do nothing,
// and so looked pure, cheap and property-rich.
//
// That core exposes no register setters, so inputs are supplied the same way
// the pipeline's Z80 asserts supply them — by assembling a short prologue that
// loads the registers, calls the function, and returns to a known address:
//
//	LD HL,AF / PUSH HL / POP AF   ; F cannot be loaded directly
//	LD BC,nn / LD DE,nn / LD HL,nn
//	CALL entry
//	retAddr:                       ; execution stops here
//
// SP is primed to stackBase before the prologue runs, and PUSH/POP balance, so
// at retAddr a well-behaved function leaves SP exactly at stackBase. The
// function's own cost is the cycle count between entry and retAddr, excluding
// the prologue.
func (a *Analysis) runTrial(fn *Function, in regState) trialOut {
	emu := emulator.NewRemogattoZ80()

	if err := emu.LoadMemory(a.Origin, a.Data); err != nil {
		return trialOut{timedOut: true}
	}

	// Assemble the prologue.
	tramp := []byte{
		0x21, in.F, in.A, // LD HL, A<<8|F
		0xE5,             // PUSH HL
		0xF1,             // POP AF
		0x01, in.C, in.B, // LD BC, nn
		0x11, in.E, in.D, // LD DE, nn
		0x21, in.L, in.H, // LD HL, nn
		0xCD, uint8(fn.Entry), uint8(fn.Entry >> 8), // CALL entry
	}
	retAddr := trampolineAddr + uint16(len(tramp))
	for i, b := range tramp {
		emu.SetMemory(trampolineAddr+uint16(i), b)
	}
	emu.SetMemory(retAddr, 0x76) // HALT, in case execution is resumed there

	// Observe port traffic. Any IN or OUT on any port means the function is not
	// pure; the previous handlers recorded only the two console ports, so this
	// was undetectable and hasIO stayed false forever.
	sawIO := false
	emu.SetIOHandlers(
		func(port uint16) byte { sawIO = true; return 0xFF },
		func(port uint16, value byte) { sawIO = true },
	)

	emu.SetSP(stackBase)
	emu.SetPC(trampolineAddr)

	// Snapshot memory once the program and prologue are in place, so neither
	// counts as a write by the function.
	var memBefore [65536]byte
	copy(memBefore[:], emu.Memory())

	entered := false
	cyclesAtEntry := 0
	totalCycles := 0
	funcCycles := 0
	spAtReturn := stackBase
	timedOut := true

	for step := 0; step < maxSteps; step++ {
		totalCycles += emu.Step()
		pc := emu.GetPC()

		if !entered && pc == fn.Entry {
			entered = true
			cyclesAtEntry = totalCycles
		}
		if pc == retAddr {
			funcCycles = totalCycles - cyclesAtEntry
			spAtReturn = emu.GetSP()
			timedOut = false
			break
		}
		if emu.IsHalted() {
			spAtReturn = emu.GetSP()
			timedOut = pc != retAddr
			break
		}
	}

	if timedOut {
		return trialOut{timedOut: true}
	}

	regs := emu.GetRegisters()
	out := regState{
		A: regs.A, F: regs.F,
		B: uint8(regs.BC >> 8), C: uint8(regs.BC),
		D: uint8(regs.DE >> 8), E: uint8(regs.DE),
		H: uint8(regs.HL >> 8), L: uint8(regs.HL),
	}

	// A balanced function returns with SP back at stackBase.
	spDelta := int(spAtReturn) - int(stackBase)

	// Memory writes, ignoring the stack region.
	memAfter := emu.Memory()
	memWrites := 0
	for addr := 0; addr < int(stackRegion); addr++ {
		if memAfter[addr] != memBefore[addr] {
			memWrites++
		}
	}

	return trialOut{
		regs: out, cycles: funcCycles, spDelta: spDelta,
		memWrites: memWrites, hasIO: sawIO, timedOut: false,
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

// detectOutClobber reports which registers the function modifies.
//
// Modification is measured per trial as output != input. Variance across trials
// cannot be used for this: a register the function merely passes through still
// varies, because the random input varied, so a bare RET came out claiming all
// eight registers as its output.
func detectOutClobber(ins, outs []regState, in RegSet) (RegSet, RegSet) {
	if len(outs) < 2 || len(ins) != len(outs) {
		return 0, 0
	}
	var changed RegSet
	for i := range outs {
		changed |= diffMask(ins[i], outs[i])
	}

	out := changed & (RegA | RegHL | RegDE | RegBC | RegF)
	clobber := changed &^ out &^ in
	if clobber == RegF {
		clobber = 0
	}
	return out, clobber
}

// checkIdempotent reports whether f(f(x)) == f(x) on the registers in out.
func (a *Analysis) checkIdempotent(fn *Function, rng *rand.Rand, n int, out RegSet) bool {
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
		if !eqOn(out2.regs, out1.regs, out) {
			return false
		}
	}
	return true
}

// checkInvolution reports whether f(f(x)) == x on the registers in out.
//
// Flags are excluded: the comparison is against the original input, and F on
// exit reflects the function's last operation rather than the F it was handed.
// Including it would reject every real involution, NEG among them.
func (a *Analysis) checkInvolution(fn *Function, rng *rand.Rand, n int, out RegSet) bool {
	out &^= RegF
	if out == 0 {
		return false
	}
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
		if !eqOn(out2.regs, in, out) {
			return false
		}
	}
	return true
}

// diffMask returns the set of registers whose values differ between two states.
func diffMask(x, y regState) RegSet {
	var d RegSet
	if x.A != y.A {
		d |= RegA
	}
	if x.F != y.F {
		d |= RegF
	}
	if x.B != y.B {
		d |= RegB
	}
	if x.C != y.C {
		d |= RegC
	}
	if x.D != y.D {
		d |= RegD
	}
	if x.E != y.E {
		d |= RegE
	}
	if x.H != y.H {
		d |= RegH
	}
	if x.L != y.L {
		d |= RegL
	}
	return d
}

// eqOn reports whether two states agree on every register in mask.
func eqOn(x, y regState, mask RegSet) bool {
	return diffMask(x, y)&mask == 0
}

// detectConstant reports whether the output is independent of the input.
//
// Output variance cannot answer this on its own: a constant function varies in
// nothing, so detectOutClobber finds no OUT registers for it. Nor can whole-state
// comparison — a function returning a fixed value in A still leaves the caller's
// B, C, D... in place, and those differ from trial to trial because the inputs
// did. So look instead at which registers the function *writes*: a register is
// written if some trial left it holding something other than what went in. The
// function is constant when every written register holds the same value in every
// trial.
func detectConstant(ins, outs []regState) bool {
	if len(outs) < 2 || len(ins) != len(outs) {
		return false
	}
	var written RegSet
	for i := range outs {
		written |= diffMask(ins[i], outs[i])
	}
	if written == 0 {
		return false // writes nothing, so there is no output to be constant in
	}
	for _, o := range outs[1:] {
		if !eqOn(o, outs[0], written) {
			return false
		}
	}
	return true
}
