package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"os"
	"reflect"
	"strings"
	"testing"
)

type packedWordRunner struct {
	bin    *z80asm.Result
	memory *memoryHostZ80Memory
}

func newPackedWordRunner(a *z80asm.Result) *packedWordRunner {
	m := &memoryHostZ80Memory{lengths: map[uint16]int{}, active: true}
	copy(m.data[1024:], a.Binary)
	for _, l := range a.Listing {
		if len(l.Bytes) > 0 {
			m.lengths[uint16(l.Address)] = len(l.Bytes)
			for i := range l.Bytes {
				m.executable[l.Address+i] = true
			}
		}
	}
	return &packedWordRunner{a, m}
}
func runPackedWord(t *testing.T, runner *packedWordRunner, ptr uint16, lo, hi byte, denySecond bool) (uint16, int, []memoryHostNativeAccess, error) {
	t.Helper()
	m := runner.memory
	m.events = nil
	next := ptr + 1
	m.data[ptr] = lo
	m.data[next] = hi
	m.readable[ptr] = true
	m.readable[next] = !denySecond
	m.region[ptr] = "input"
	m.region[next] = "input"
	m.stack[0xeffe] = true
	m.stack[0xefff] = true
	m.stackInitialized[0xeffe] = true
	m.stackInitialized[0xefff] = true
	m.callFrames = []memoryHostCallFrame{{SP: 0xeffe, Return: 0xf100}}
	m.readable[0xeffe] = true
	m.readable[0xefff] = true
	m.data[0xeffe] = 0
	m.data[0xefff] = 0xf1
	if m.cpu == nil {
		m.cpu = z80.NewZ80(m, memoryHostNoPorts{})
	}
	m.cpu.Tstates = 0
	m.cpu.SetPC(1024)
	m.cpu.SetSP(0xeffe)
	m.cpu.H = byte(ptr >> 8)
	m.cpu.L = byte(ptr)
	cycles := 0
	for i := 0; i < 100; i++ {
		n, err := memoryHostNativeStep(m)
		cycles += n
		if err != nil {
			return m.cpu.HL(), cycles, m.events, err
		}
		if m.cpu.PC() == 0xf100 {
			if m.cpu.SP() != 0xf000 {
				t.Fatal("SP", m.cpu.SP())
			}
			return m.cpu.HL(), cycles, m.events, nil
		}
	}
	t.Fatal("nontermination")
	return 0, 0, nil, nil
}

func TestFAT12BytePackingRetainsNativeAccessesAndWrap(t *testing.T) {
	src, err := os.ReadFile("../../../stdlib/fs/fat12.minz")
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.Parse(string(src), "fat12.minz")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	begin := strings.Index(s.Assembly, "ld_word:\n")
	end := strings.Index(s.Assembly[begin:], "\n    RET\n") + begin + len("\n    RET\n")
	selected := s.Assembly[begin:end]
	if !strings.Contains(selected, "pack disjoint byte lanes") {
		t.Fatal("actual unchanged helper did not pack", selected)
	}
	// Recorded original unfused helper from origin/main. Keep an independent
	// native oracle even when MIR packing removes the old intermediate words.
	baseline := `ld_word:
    LD C, (HL)
    LD B, 0
    LD DE, 1
    ADD HL, DE
    LD D, (HL)
    LD L, D
    LD H, 0
    LD H, L
    LD L, 0
    LD A, H
    OR B
    LD H, A
    LD A, L
    OR C
    LD L, A
    RET
`
	selectedBin, err := z80asm.NewAssembler().AssembleString("ORG 1024\n" + selected)
	if err != nil || len(selectedBin.Errors) > 0 {
		t.Fatal(err, selectedBin.Errors)
	}
	baselineBin, err := z80asm.NewAssembler().AssembleString("ORG 1024\n" + baseline)
	if err != nil || len(baselineBin.Errors) > 0 {
		t.Fatal(err, baselineBin.Errors)
	}
	selectedRunner, baselineRunner := newPackedWordRunner(selectedBin), newPackedWordRunner(baselineBin)
	raw := mir2.NewVM(s.MIR2Module)
	raw.EnsureHeap(65536)
	cases := 0
	var baseT, selectedT int
	for _, ptr := range []uint16{0x8000, 0xffff} {
		for word := 0; word < 65536; word++ {
			lo, hi := byte(word), byte(word>>8)
			want := uint16(word)
			raw.WriteHeap(int64(ptr), lo)
			raw.WriteHeap(int64(uint16(ptr+1)), hi)
			values, err := raw.Call("ld_word", []mir2.Value{{I: int64(ptr)}})
			if err != nil || len(values) != 1 || values[0].I != int64(word) {
				t.Fatal("MIR word oracle", values, err)
			}
			got, cycles, events, err := runPackedWord(t, selectedRunner, ptr, lo, hi, false)
			if err != nil || got != want {
				t.Fatalf("packed ptr%04x word%04x got%04x err%v", ptr, word, got, err)
			}
			wantEvents := []memoryHostNativeAccess{{"read", ptr, lo}, {"read", ptr + 1, hi}}
			if !reflect.DeepEqual(events, wantEvents) {
				t.Fatal("ordered access oracle", events, wantEvents)
			}
			selectedT = cycles
			// Both native paths cover the complete word domain.
			{
				b, c, e, err := runPackedWord(t, baselineRunner, ptr, lo, hi, false)
				if err != nil || b != want || !reflect.DeepEqual(e, wantEvents) {
					t.Fatal("baseline oracle", b, e, err)
				}
				baseT = c
			}
			cases++
		}
	}
	for _, ptr := range []uint16{0x8000, 0xffff} {
		for _, runner := range []*packedWordRunner{baselineRunner, selectedRunner} {
			_, _, events, err := runPackedWord(t, runner, ptr, 0x34, 0x12, true)
			if err == nil || !reflect.DeepEqual(events, []memoryHostNativeAccess{{"read", ptr, 0x34}}) {
				t.Fatal("partial refusal prefix", events, err)
			}
		}
	}
	a, _ := z80asm.NewAssembler().AssembleString(selected)
	b, _ := z80asm.NewAssembler().AssembleString(baseline)
	t.Logf("unchanged fat12 ld_word %d exhaustive selected VM/native cases incl ffff wrap; ordered reads and denied-second prefix PASS; helper %d→%d T/%d→%d B", cases, baseT, selectedT, len(b.Binary), len(a.Binary))
	if baseT-selectedT != 45 || len(b.Binary)-len(a.Binary) != 12 {
		t.Fatal("unexpected measured delta", baseT, selectedT)
	}
	if dir := os.Getenv("MINZ_BYTE_PACK_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0755)
		for n, d := range map[string][]byte{"source.minz": src, "baseline.a80": []byte(baseline), "selected.a80": []byte(selected), "selected.bin": a.Binary, "baseline.bin": b.Binary, "final.mir": []byte(s.MIR2Module.Dump())} {
			if err := os.WriteFile(dir+"/"+n, d, 0644); err != nil {
				t.Fatal(err)
			}
		}
		os.WriteFile(dir+"/metrics.txt", []byte(fmt.Sprintf("%d cases %d→%d T %d→%d B\n", cases, baseT, selectedT, len(b.Binary), len(a.Binary))), 0644)
	}
}
