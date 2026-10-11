package mir2

import (
	"fmt"
	"strings"
)

// NativeCallFrameSnapshot describes only compiler-generated result pickup.
// Instruction markers bind the finite sequence; they grant no numeric stack
// authority outside its independently checked frame geometry.
type NativeCallFrameSnapshot struct {
	ID            string                       `json:"id"`
	Function      string                       `json:"function"`
	Block         string                       `json:"block"`
	CallerSaved   []string                     `json:"caller_saved"`
	SnapshotPairs []string                     `json:"snapshot_pairs"`
	Results       []NativeCallFrameResult      `json:"results"`
	Instructions  []NativeCallFrameInstruction `json:"instructions"`
}
type NativeCallFrameResult struct {
	Source, Destination string
	Width               int
}
type NativeCallFrameInstruction struct {
	ID       string `json:"id"`
	Assembly string `json:"assembly"`
}

func (g *z80cg) beginNativeCallFrame(saved, pairs []string, results []parallelCopy) {
	if g.nativeCallFrameSink == nil {
		return
	}
	if g.nativeCallFrameActive != nil {
		g.err = fmt.Errorf("nested emission call snapshot")
		return
	}
	id := fmt.Sprintf("f%d_b%d_cf%d", g.nativeFunctionIndex, g.nativeBlockIndex, len(*g.nativeCallFrameSink))
	d := NativeCallFrameSnapshot{ID: id, Function: g.fn.Name, Block: g.fn.Blocks[g.nativeBlockIndex].Label, CallerSaved: append([]string{}, saved...), SnapshotPairs: append([]string{}, pairs...)}
	for _, r := range results {
		d.Results = append(d.Results, NativeCallFrameResult{r.srcName, r.dstName, r.ty.Width()})
	}
	*g.nativeCallFrameSink = append(*g.nativeCallFrameSink, d)
	g.nativeCallFrameActive = &(*g.nativeCallFrameSink)[len(*g.nativeCallFrameSink)-1]
}
func (g *z80cg) endNativeCallFrame() { g.nativeCallFrameActive = nil }
func (g *z80cg) annotateNativeCallFrame(s string) string {
	d := g.nativeCallFrameActive
	if d == nil || !strings.HasPrefix(s, "    ") || strings.HasPrefix(strings.TrimSpace(s), ";") {
		return s
	}
	id := fmt.Sprintf("%s_s%d", d.ID, len(d.Instructions))
	d.Instructions = append(d.Instructions, NativeCallFrameInstruction{ID: id, Assembly: strings.TrimSpace(s)})
	return s + " ; @minz-call-frame:" + id
}
func CheckNativeCallFrameMarkers(assembly string, records []NativeCallFrameSnapshot) error {
	count := 0
	for _, d := range records {
		for _, i := range d.Instructions {
			count++
			if i.ID == "" || strings.Count(assembly, "@minz-call-frame:"+i.ID+"\n") != 1 {
				return fmt.Errorf("missing/duplicate call-frame marker %q", i.ID)
			}
		}
	}
	if strings.Count(assembly, "@minz-call-frame:") != count {
		return fmt.Errorf("orphan call-frame marker")
	}
	return nil
}
