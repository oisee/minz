package pipeline

import (
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"sort"
	"testing"
)

func TestMemoryHostReachableAliasAndUnreachableNextFunction(t *testing.T) {
	// Real EQU alias shares code with a reachable target, while the next distinct
	// function must remain nonexecutable. DB and operand interiors remain denied.
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x400\nreachable:\n LD A,7\n RET\nalias EQU reachable\nunreachable:\n LD A,99\n RET\ndata:\n DB 42\n")
	if err != nil || len(a.Errors) > 0 {
		t.Fatal(err, a.Errors)
	}
	if a.Symbols["REACHABLE"] != a.Symbols["ALIAS"] {
		t.Fatal("fixture lacks actual EQU alias")
	}
	ranges := []memoryHostFunctionRange{{"reachable", a.Symbols["REACHABLE"]}, {"alias", a.Symbols["ALIAS"]}, {"unreachable", a.Symbols["UNREACHABLE"]}}
	sort.SliceStable(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	for _, reach := range []map[string]bool{{"reachable": true}, {"alias": true}} {
		m := &memoryHostZ80Memory{lengths: map[uint16]int{}, active: true}
		copy(m.data[0x400:], a.Binary)
		for _, line := range a.Listing {
			if len(line.Bytes) == 0 || line.SourceLine == "DB 42" || !memoryHostReachableInstruction(ranges, reach, line.Address) {
				continue
			}
			m.lengths[uint16(line.Address)] = len(line.Bytes)
			for p := line.Address; p < line.Address+len(line.Bytes); p++ {
				m.executable[p] = true
			}
		}
		m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
		m.cpu.SetPC(0x400)
		if _, e := memoryHostNativeStep(m); e != nil || m.cpu.A != 7 {
			t.Fatal("reachable EQU body denied", e, m.cpu.A)
		}
		for _, pc := range []uint16{0x401, uint16(a.Symbols["UNREACHABLE"]), uint16(a.Symbols["DATA"])} {
			m.cpu.SetPC(pc)
			if _, e := memoryHostNativeStep(m); e == nil {
				t.Fatalf("noninstruction/unreachable/data accepted at %04x", pc)
			}
		}
	}
}
