package mir2

import (
	"strings"
	"testing"
)

func TestNativeMemoryOrphanSpillNormalization(t *testing.T) {
	for _, defined := range []bool{false, true} {
		f := &Func{Name: "orphan", Blocks: []*Block{{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8}}}}}
		sink := []NativeMemoryAccess{}
		sb := &strings.Builder{}
		g := &z80cg{fn: f, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocMem}}}, sb: sb, nativeMemorySink: &sink, nativeOrphanLabels: map[string]string{}}
		pair := &tsmcSpillPair{reg: 1, width: 8}
		label := "_tsmc_orphan_r1_0"
		g.emitNativeTSMCSpill(pair, label, 0, "    LD (%s+1), A", label)
		if defined {
			g.emit(label + ":")
			g.emit("    LD A, 0")
		}
		err := g.resolveNativeOrphanSpills(sb.String())
		if defined {
			if err == nil {
				t.Fatal("actualcodepatchaccepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		fixOrphanedTSMCStores(sb, "orphan")
		if len(sink) != 1 || sink[0].StorageKind != "spill" || sink[0].Root.Symbol != "_spill_orphan_r1" || !strings.Contains(sb.String(), "LD (_spill_orphan_r1), A") {
			t.Fatal(sink, sb.String())
		}
		if err = CheckNativeMemoryAccessMarkers(sb.String(), sink); err != nil {
			t.Fatal(err)
		}
	}
}
func TestNativeMemoryParallelCopySpillMarkers(t *testing.T) {
	f := &Func{Name: "edge", Blocks: []*Block{{Label: "entry", Insts: []*Inst{{Op: OpConst, Dst: 1, Ty: TyU8}}}}}
	sink := []NativeMemoryAccess{}
	sb := &strings.Builder{}
	g := &z80cg{fn: f, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocMem}}}, sb: sb, nativeMemorySink: &sink}
	g.emitParallelCopy([]parallelCopy{{srcName: "A", dstName: "_spill_edge_r1", ty: TyU8}})
	if len(sink) != 1 || sink[0].StorageKind != "spill" || !sink[0].Write || sink[0].Bytes != 1 {
		t.Fatal(sink, sb.String())
	}
}
