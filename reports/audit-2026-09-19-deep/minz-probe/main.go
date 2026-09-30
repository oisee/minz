package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/vir"
	"os"
	"path/filepath"
	"unsafe"
)

func main() {
	dir, err := os.MkdirTemp("", "minz-deep-probe-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	header := func(magic string, count uint32, metrics byte) []byte {
		b := []byte(magic)
		b = binary.LittleEndian.AppendUint32(b, 1)
		b = binary.LittleEndian.AppendUint32(b, count)
		return append(b, 4, metrics, 0, 0)
	}
	v2 := func(count uint64) []byte {
		b := []byte{'Z', '8', '0', 'T', 2, 0, 0, 0, 6, 3, 4}
		return binary.LittleEndian.AppendUint64(b, count)
	}
	cases := map[string][]byte{
		"ENRT_missing_flags_metrics": append(header("ENRT", 1, 12), 2, 8, 0, 0, 2),
		"ENRT_valid_infeasible":      append(header("ENRT", 1, 12), 255),
		"Z80T_count_mismatch":        append(v2(2), 255),
		"Z80T_invalid_location":      append(v2(1), 2, 8, 0, 255, 254),
	}
	out := map[string]any{"entry_bytes": unsafe.Sizeof(vir.EnrichedEntry{})}
	for name, b := range cases {
		p := filepath.Join(dir, name)
		if e := os.WriteFile(p, b, 0600); e != nil {
			panic(e)
		}
		t, e := vir.LoadEnrichedBinary(p)
		r := map[string]any{"accepted": e == nil}
		if e != nil {
			r["error"] = e.Error()
		} else {
			r["records"] = len(t.Entries)
			if len(t.Entries) > 0 {
				r["first"] = t.Entries[0]
			}
		}
		out[name] = r
	}
	base := vir.EnrichedShape{NVregs: 2, Widths: []int{8, 8}, LocSetIndex: []int{0, 0}, Interference: 2}
	i, e := vir.EnrichedIndexOf(base, 4)
	out["invalid_mask_index"] = map[string]any{"index": i, "error": fmt.Sprint(e)}
	good := base
	good.Interference = 0
	good.LocSetIndex = []int{0, 1}
	j, e := vir.EnrichedIndexOf(good, 4)
	out["valid_shape_index"] = map[string]any{"index": j, "error": fmt.Sprint(e)}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if e := enc.Encode(out); e != nil {
		panic(e)
	}
}
