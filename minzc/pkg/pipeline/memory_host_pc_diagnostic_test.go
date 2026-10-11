package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

type pcOwner struct {
	Name    string
	Start   int
	End     int
	Adapter bool
}
type pcCount struct {
	PC          uint16
	Owner       string
	Instruction []byte
	Steps       int64
	TStates     int64
}
type pcLedger struct {
	Listing      []z80asm.ListingLine
	Symbols      map[string]int
	Owners       []pcOwner
	Aliases      map[string]int
	Calls        []map[uint16]*pcCount
	Instructions map[uint16][]byte
}

func pcPhysicalOwners(listing []z80asm.ListingLine, symbols map[string]int, names []string, hosts map[string]string) ([]pcOwner, map[string]int, error) {
	known := map[string]bool{}
	for _, n := range names {
		known[n] = true
	}
	owners := []pcOwner{}
	aliases := map[string]int{}
	seen := map[int]string{}
	for _, line := range listing {
		text := strings.TrimSpace(line.SourceLine)
		if fields := strings.Fields(text); len(fields) >= 3 && strings.EqualFold(fields[1], "EQU") && known[fields[0]] {
			address, ok := symbols[fields[0]]
			if !ok {
				return nil, nil, fmt.Errorf("alias symbol missing %s", fields[0])
			}
			aliases[fields[0]] = address
		}
		if !strings.HasSuffix(text, ":") {
			continue
		}
		name := strings.TrimSuffix(text, ":")
		if !known[name] {
			continue
		}
		address, ok := symbols[name]
		if !ok || address != line.Address {
			return nil, nil, fmt.Errorf("physical entry missing %s", name)
		}
		if old, ok := seen[address]; ok {
			return nil, nil, fmt.Errorf("duplicate physical entries %s/%s", old, name)
		}
		seen[address] = name
		_, adapter := hosts[name]
		owners = append(owners, pcOwner{Name: name, Start: address, Adapter: adapter})
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].Start < owners[j].Start })
	instructions := map[int]bool{}
	codeEnd := 0
	for _, line := range listing {
		if pcListingInstruction(line) {
			instructions[line.Address] = true
			if end := line.Address + len(line.Bytes); end > codeEnd {
				codeEnd = end
			}
		}
	}
	for i := range owners {
		if !instructions[owners[i].Start] {
			return nil, nil, fmt.Errorf("entry has no instruction %s", owners[i].Name)
		}
		owners[i].End = codeEnd
		if i+1 < len(owners) {
			owners[i].End = owners[i+1].Start
		}
	}
	return owners, aliases, nil
}
func pcListingInstruction(line z80asm.ListingLine) bool {
	if len(line.Bytes) == 0 {
		return false
	}
	text := strings.TrimSpace(strings.SplitN(line.SourceLine, ";", 2)[0])
	f := strings.Fields(text)
	if len(f) > 0 && strings.HasSuffix(f[0], ":") {
		f = f[1:]
	}
	if len(f) == 0 {
		return false
	}
	switch strings.ToUpper(f[0]) {
	case "DB", "DW", "DS", "DEFB", "DEFW", "DEFS":
		return false
	}
	return true
}
func pcCheckedAdd(a, b int64) int64 {
	if b < 0 || a < 0 || a > math.MaxInt64-b {
		panic("ledger overflow")
	}
	return a + b
}
func (p *pcLedger) setup(a *z80asm.Result, names []string, hosts map[string]string) error {
	var e error
	p.Owners, p.Aliases, e = pcPhysicalOwners(a.Listing, a.Symbols, names, hosts)
	p.Listing = a.Listing
	p.Symbols = a.Symbols
	p.Instructions = map[uint16][]byte{}
	for _, line := range a.Listing {
		if pcListingInstruction(line) {
			if line.Address < a.Origin || line.Address+len(line.Bytes) > a.Origin+len(a.Binary) {
				return fmt.Errorf("listing outside binary")
			}
			if !bytes.Equal(line.Bytes, a.Binary[line.Address-a.Origin:line.Address-a.Origin+len(line.Bytes)]) {
				return fmt.Errorf("listing binary mismatch")
			}
			if _, exists := p.Instructions[uint16(line.Address)]; exists {
				return fmt.Errorf("duplicate instruction PC")
			}
			p.Instructions[uint16(line.Address)] = append([]byte{}, line.Bytes...)
		}
	}
	return e
}
func (p *pcLedger) observe(call int, pc uint16, cycles int, instruction []byte) {
	if call < 0 || cycles <= 0 || !bytes.Equal(p.Instructions[pc], instruction) || len(instruction) == 0 {
		panic("unbound instruction/cycles")
	}
	for len(p.Calls) <= call {
		p.Calls = append(p.Calls, map[uint16]*pcCount{})
	}
	owner := "unowned"
	for _, o := range p.Owners {
		if o.Start > int(pc) {
			break
		}
		if int(pc) >= o.End {
			continue
		}
		owner = o.Name
		if o.Adapter {
			owner = "adapter:" + owner
		}
	}
	row := p.Calls[call][pc]
	if row == nil {
		row = &pcCount{PC: pc, Owner: owner, Instruction: append([]byte{}, instruction...)}
		p.Calls[call][pc] = row
	}
	if !bytes.Equal(row.Instruction, instruction) {
		panic("instruction bytes changed")
	}
	row.Steps = pcCheckedAdd(row.Steps, 1)
	row.TStates = pcCheckedAdd(row.TStates, int64(cycles))
}
func (p *pcLedger) verify(e *MemoryHostTargetEvidence) error {
	if len(p.Calls) != len(e.Invocations) {
		return fmt.Errorf("invocation cardinality")
	}
	var steps, total int64
	for i, call := range p.Calls {
		var ns, nt int64
		for _, row := range call {
			ns = pcCheckedAdd(ns, row.Steps)
			nt = pcCheckedAdd(nt, row.TStates)
		}
		if ns != e.Invocations[i].Steps || nt != e.InvocationTStates[i] {
			return fmt.Errorf("call %d ledger totals", i)
		}
		steps = pcCheckedAdd(steps, ns)
		total = pcCheckedAdd(total, nt)
	}
	if steps != e.ExecutedSteps || total != e.TotalTStates {
		return fmt.Errorf("whole ledger totals")
	}
	return nil
}

func TestMemoryHostPCDiagnosticOwnerPartition(t *testing.T) {
	listing := []z80asm.ListingLine{{Address: 90, Bytes: []byte{0}, SourceLine: "NOP"}, {Address: 100, SourceLine: "main:"}, {Address: 100, Bytes: []byte{1}, SourceLine: "NOP"}, {Address: 110, SourceLine: "peek:"}, {Address: 110, Bytes: []byte{2}, SourceLine: "NOP"}, {Address: 120, SourceLine: "node_tag EQU peek"}}
	owners, aliases, e := pcPhysicalOwners(listing, map[string]int{"main": 100, "peek": 110, "node_tag": 110}, []string{"main", "peek", "node_tag"}, map[string]string{"peek": "read"})
	if e != nil || len(owners) != 2 || !owners[1].Adapter || aliases["node_tag"] != 110 {
		t.Fatal(owners, aliases, e)
	}
	if _, _, e = pcPhysicalOwners(append(listing, z80asm.ListingLine{Address: 110, SourceLine: "other:"}), map[string]int{"main": 100, "peek": 110, "node_tag": 110, "other": 110}, []string{"main", "peek", "node_tag", "other"}, nil); e == nil {
		t.Fatal("duplicate emitting entry accepted")
	}
	p := &pcLedger{Owners: owners, Instructions: map[uint16][]byte{90: {0}, 100: {1}, 110: {2}}}
	p.observe(0, 90, 4, []byte{0})
	p.observe(0, 100, 7, []byte{1})
	p.observe(0, 110, 10, []byte{2})
	if p.Calls[0][90].Owner != "unowned" || p.Calls[0][110].Owner != "adapter:peek" {
		t.Fatal(p.Calls)
	}
}

func TestMemoryHostPCDiagnosticOriginalBoth30(t *testing.T) {
	dir := os.Getenv("MINZ_MEMORY_HOST_PC_ARTIFACTS")
	if dir == "" {
		t.Skip("isolated explicit artifact measurement only")
	}
	t.Cleanup(func() { memoryHostPCLayoutDiagnostic = nil; memoryHostPCStepDiagnostic = nil })
	_, w := memoryHostParserFixture(t)
	sourceBytes, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	parse := func() (*hir.Module, error) {
		m, e := nanz.Parse(string(sourceBytes), "../examples/nanz/self_lanz_parser.nanz")
		if e == nil {
			m.Target = hir.TargetCPM
		}
		return m, e
	}
	for _, mask := range []int{0, 3} {
		ledger := &pcLedger{}
		memoryHostPCLayoutDiagnostic = ledger.setup
		memoryHostPCStepDiagnostic = ledger.observe
		factory := func() (*hir.Module, error) {
			m, e := parse()
			if e != nil {
				return nil, e
			}
			original, e := parse()
			if e != nil {
				return nil, e
			}
			if e := constructMemoryHostArenaAppend(m, w, mask&1 != 0, mask&2 != 0); e != nil {
				return nil, e
			}
			if e := hir.ValidateMemoryHostArenaAppendChoices(original, m, w, hir.MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}); e != nil {
				return nil, e
			}
			return m, nil
		}
		evidence, e := RunZ80MemoryHostBaseline(factory, w)
		memoryHostPCLayoutDiagnostic = nil
		memoryHostPCStepDiagnostic = nil
		if e != nil {
			t.Fatal(e)
		}
		if e = ledger.verify(evidence.Target); e != nil {
			t.Fatal(e)
		}
		name := map[int]string{0: "original", 3: "both"}[mask]
		out := filepath.Join(dir, name)
		if e = os.MkdirAll(out, 0755); e != nil {
			t.Fatal(e)
		}
		for n, v := range map[string]any{"pc-ledger.json": ledger, "factory-baseline.json": evidence} {
			b, e := json.MarshalIndent(v, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(out, n), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
		if e = os.WriteFile(filepath.Join(out, "target.bin"), evidence.Target.Binary, 0644); e != nil {
			t.Fatal(e)
		}
		reference, err := os.ReadFile(filepath.Join(os.Getenv("MINZ_MEMORY_HOST_PC_REFERENCE"), name, "factory-baseline.json"))
		if err != nil {
			t.Fatal(err)
		}
		var previous MemoryHostBaselineEvidence
		if err = json.Unmarshal(reference, &previous); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(previous.Target.Binary, evidence.Target.Binary) || previous.Target.Assembly != evidence.Target.Assembly || previous.Target.AdapterSHA256 != evidence.Target.AdapterSHA256 || !reflect.DeepEqual(previous.Target.InvocationTStates, evidence.Target.InvocationTStates) || previous.Target.TotalTStates != evidence.Target.TotalTStates || previous.Target.ExecutedSteps != evidence.Target.ExecutedSteps || arenaSameObservations(previous.Target.Invocations, evidence.Target.Invocations) != nil {
			t.Fatal("saved binary/cost/state drift")
		}
		for i := range previous.Target.Invocations {
			if previous.Target.Invocations[i].Steps != evidence.Target.Invocations[i].Steps {
				t.Fatal("per-call steps drift")
			}
		}
		for _, same := range []bool{previous.Target.SidecarSHA256 == evidence.Target.SidecarSHA256, previous.Target.CallFramesSHA256 == evidence.Target.CallFramesSHA256, previous.Target.AllocationSHA256 == evidence.Target.AllocationSHA256, previous.Target.MIRSHA256 == evidence.Target.MIRSHA256} {
			if !same {
				t.Fatalf("artifact binding drift: sidecar %s/%s frame %s/%s allocation %s/%s MIR %s/%s", previous.Target.SidecarSHA256, evidence.Target.SidecarSHA256, previous.Target.CallFramesSHA256, evidence.Target.CallFramesSHA256, previous.Target.AllocationSHA256, evidence.Target.AllocationSHA256, previous.Target.MIRSHA256, evidence.Target.MIRSHA256)
			}
		}

		t.Logf("%s %dT steps%d BIN%x", name, evidence.Target.TotalTStates, evidence.Target.ExecutedSteps, sha256.Sum256(evidence.Target.Binary))
	}
}
