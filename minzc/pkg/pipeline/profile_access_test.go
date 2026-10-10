package pipeline

import (
	"strings"
	"testing"
)

func TestMeasuredSumRefusesZeroTripExtraRead(t *testing.T) {
	factory, _ := measuredSumFixture(t)
	h, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	// The return remains correct (zero), but the binary reads input even though
	// the zero-trip source loop has no data accesses. Final-memory equality alone
	// would accept this binary.
	s.Assembly = "sum_array:\n LD HL, 16384\n LD C, (HL)\n LD A, 0\n RET\n"
	v := measuredLoopVariant{module: h, steps: s}
	var digest string
	_, err = measureLoopExecution(v, "sum_array", []int64{0x4000, 0}, map[uint16]byte{}, 0, false, &digest)
	if err == nil || !strings.Contains(err.Error(), "unexpected data access at 4000") {
		t.Fatalf("accepted zero-trip input read: %v", err)
	}
}
