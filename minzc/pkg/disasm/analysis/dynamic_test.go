package analysis

import "testing"

// dynStub places a hand-assembled routine at $8000 and describes it as a single
// function, so dynAnalyzeFunc can be exercised without a real binary. $8000
// keeps the code clear of the sentinel the harness writes at address 0.
func dynStub(code []byte) (*Analysis, *Function) {
	const origin = uint16(0x8000)
	a := &Analysis{
		Data:      code,
		Origin:    origin,
		Functions: map[uint16]*Function{},
	}
	fn := &Function{
		Entry: origin,
		End:   origin + uint16(len(code)) - 1,
		Name:  "stub",
		Size:  len(code),
	}
	a.Functions[origin] = fn
	return a, fn
}

// A routine that returns normally is stack-balanced. The harness pushes a
// 2-byte sentinel return address, so measuring SP against the pre-entry value
// without accounting for the RET that pops it reported +2 for every such
// function — i.e. every correct function looked broken.
func TestDynamic_StackBalanced(t *testing.T) {
	a, fn := dynStub([]byte{0x78, 0xC9}) // LD A,B / RET
	r := a.dynAnalyzeFunc(fn, 16)

	if r.TimedOut {
		t.Fatal("LD A,B / RET timed out")
	}
	if !r.StackOK || r.StackDelta != 0 {
		t.Errorf("want balanced stack, got StackOK=%v delta=%+d", r.StackOK, r.StackDelta)
	}
}

// ...and a routine that really does leave SP shifted is still caught. This one
// returns via JP (HL) after bumping SP, so it exits cleanly but one word light.
func TestDynamic_StackImbalanceDetected(t *testing.T) {
	a, fn := dynStub([]byte{0xE1, 0x33, 0xE9}) // POP HL / INC SP / JP (HL)
	r := a.dynAnalyzeFunc(fn, 16)

	if r.TimedOut {
		t.Fatal("POP HL / INC SP / JP (HL) timed out")
	}
	if r.StackOK {
		t.Error("want imbalance reported, got StackOK=true")
	}
	if r.StackDelta != 1 {
		t.Errorf("want delta +1, got %+d", r.StackDelta)
	}
}

// Port traffic makes a function impure. The old code declared a hasIO variable,
// never assigned it, and reported OUT-ing functions as pure.
func TestDynamic_IODetected(t *testing.T) {
	a, fn := dynStub([]byte{0xD3, 0xFE, 0xC9}) // OUT ($FE),A / RET
	r := a.dynAnalyzeFunc(fn, 16)

	if !r.HasIO {
		t.Error("want HasIO=true for OUT ($FE),A")
	}
	if r.Pure {
		t.Error("a function that writes a port is not pure")
	}
}

func TestDynamic_PureRegisterOnly(t *testing.T) {
	a, fn := dynStub([]byte{0x78, 0xC9}) // LD A,B / RET
	r := a.dynAnalyzeFunc(fn, 16)

	if !r.Pure {
		t.Errorf("LD A,B / RET should be pure (writes %d bytes, io=%v)", r.MemWrites, r.HasIO)
	}
	if r.MemWrites != 0 {
		t.Errorf("want no memory writes, got %d", r.MemWrites)
	}
}

// NEG is an involution on A but not idempotent. Comparing whole register states
// against the input — flags included — reported neither.
func TestDynamic_Involution(t *testing.T) {
	a, fn := dynStub([]byte{0xED, 0x44, 0xC9}) // NEG / RET
	r := a.dynAnalyzeFunc(fn, 16)

	if !r.Involution {
		t.Errorf("NEG should be an involution (out=%016b)", r.Out)
	}
	if r.Idempotent {
		t.Error("NEG is not idempotent: NEG(NEG(x)) != NEG(x) except at 0")
	}
}

// AND $0F is the mirror case: idempotent, not an involution.
func TestDynamic_Idempotent(t *testing.T) {
	a, fn := dynStub([]byte{0xE6, 0x0F, 0xC9}) // AND $0F / RET
	r := a.dynAnalyzeFunc(fn, 16)

	if !r.Idempotent {
		t.Errorf("AND $0F should be idempotent (out=%016b)", r.Out)
	}
	if r.Involution {
		t.Error("AND $0F is not an involution")
	}
}

// A fixed return value is constant even though nothing varies across trials —
// the case output-variance alone cannot see.
func TestDynamic_Constant(t *testing.T) {
	a, fn := dynStub([]byte{0x3E, 0x2A, 0xC9}) // LD A,$2A / RET
	r := a.dynAnalyzeFunc(fn, 16)

	if !r.Constant {
		t.Error("LD A,$2A / RET returns the same value regardless of input")
	}
}

// A bare RET returns nothing, so the algebraic predicates are vacuous rather
// than true. They used to both come out true for RST vectors and stubs.
func TestDynamic_NoOutputHasNoProperties(t *testing.T) {
	a, fn := dynStub([]byte{0xC9}) // RET
	r := a.dynAnalyzeFunc(fn, 16)

	if r.Out != 0 {
		t.Fatalf("bare RET should have no OUT registers, got %016b", r.Out)
	}
	if r.Idempotent || r.Involution {
		t.Errorf("want no properties on a function with no output, got idempotent=%v involution=%v",
			r.Idempotent, r.Involution)
	}
	if r.Constant {
		t.Error("a function that writes nothing has no output to be constant in")
	}
}
