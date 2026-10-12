package mir2

import (
	"strings"
	"testing"
)

func TestNativeCallSnapshotOptionalEmission(t *testing.T) {
	emit := func(capture bool) (string, []NativeCallFrameSnapshot) {
		var b strings.Builder
		records := []NativeCallFrameSnapshot{}
		g := &z80cg{sb: &b, fn: &Func{Name: "f", Blocks: []*Block{{Label: "entry"}}}}
		if capture {
			g.nativeCallFrameSink = &records
		}
		g.pickupCallResults([]parallelCopy{{srcName: "A", dstName: "C", ty: TyU8}}, []string{"AF", "BC", "DE", "HL"}, false)
		return b.String(), records
	}
	plain, _ := emit(false)
	captured, records := emit(true)
	if len(records) != 1 || len(records[0].Instructions) != 19 {
		t.Fatal("missing fallback snapshot")
	}
	if e := CheckNativeCallFrameMarkers(captured, records); e != nil {
		t.Fatal(e)
	}
	lines := strings.Split(captured, "\n")
	for j, l := range lines {
		lines[j] = strings.SplitN(l, " ; @minz-call-frame:", 2)[0]
	}
	if strings.Join(lines, "\n") != plain {
		t.Fatal("optional capture changed instruction text")
	}
	for _, s := range []string{strings.Replace(captured, "@minz-call-frame:", "@lost:", 1), captured + "; @minz-call-frame:orphan\n", captured + "; @minz-call-frame:" + records[0].Instructions[0].ID + "\n"} {
		if e := CheckNativeCallFrameMarkers(s, records); e == nil {
			t.Fatal("lost/duplicate/orphan accepted")
		}
	}
}
