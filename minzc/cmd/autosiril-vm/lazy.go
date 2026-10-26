package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

type lazyConfig struct {
	Source uint32 `json:"source"`
	Mode   string `json:"mode"`
}
type lazyInput struct {
	Notes   []virtualNote `json:"notes"`
	Configs []lazyConfig  `json:"configs"`
}

func lazyModule(engine, music, metaPath string, optimized bool) (*mir2.Module, error) {
	source, err := os.ReadFile(engine)
	if err != nil {
		return nil, err
	}
	h, err := nanz.ParseWithOpts(string(source), engine, nanz.ParseOpts{BaseDir: filepath.Dir(engine)})
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(music)
	if err != nil {
		return nil, err
	}
	c, err := lanz.Compile(string(data), "autosiril_music")
	if err != nil {
		return nil, err
	}
	data, err = os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var meta manifest
	if err = json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	sink := h.FuncByName("emit_flat")
	if sink == nil || len(sink.Params) != 2 || sink.Params[0].Ty != mir2.TyU16 || sink.Params[1].Ty != mir2.TyU16 || sink.RetTy != mir2.TyVoid {
		return nil, fmt.Errorf("invalid lazy sink ABI")
	}
	sink.IsExtern = false
	sink.IsIO = true
	sink.Body = hir.Blk(&hir.ExprStmt{Expr: &hir.CallExpr{Fn: "@host.lazy.emit_flat", Args: []hir.Expr{hir.Var(sink.Params[0].Name, mir2.TyU16), hir.Var(sink.Params[1].Name, mir2.TyU16)}, Ty: mir2.TyVoid}})
	if err = linkMusic(h, c, meta); err != nil {
		return nil, err
	}
	h.Target = hir.TargetMZV
	h.NonNullObjects = true
	m := hir.LowerModule(h)
	if optimized {
		optimize(m)
	}
	if err = mir2.Verify(m); err != nil {
		return nil, err
	}
	return m, nil
}

func runLazy(input, engine, music, metaPath, out, backend string, budget, limit uint, optimized bool) error {
	if music == "" || metaPath == "" || out == "" {
		return fmt.Errorf("lazy stage needs -music, -manifest and -out")
	}
	if backend != "flat" && backend != "banked" && backend != "churn" {
		return fmt.Errorf("unknown arena backend %q", backend)
	}
	if budget == 0 || budget > 256<<20 || limit > 4294967295 {
		return fmt.Errorf("invalid lazy budget/limit")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	var in lazyInput
	if err = json.Unmarshal(data, &in); err != nil {
		return err
	}
	if len(in.Notes) > 8184 {
		return fmt.Errorf("compact note capacity 8184 exceeded: %d", len(in.Notes))
	}
	sources := map[uint32]bool{}
	for i, n := range in.Notes {
		sources[n.Source] = true
		if n.Start > 65535 || n.Off > 65535 || n.Source > 255 || n.Pitch > 127 {
			return fmt.Errorf("note%d exceeds compact field range", i)
		}
	}
	for _, c := range in.Configs {
		if !sources[c.Source] || c.Source > 255 || (c.Mode != "p" && c.Mode != "m" && c.Mode != "d") {
			return fmt.Errorf("invalid lazy config: %+v", c)
		}
	}
	m, err := lazyModule(engine, music, metaPath, optimized)
	if err != nil {
		return err
	}
	vm := mir2.NewVM(m)
	vm.MaxMemory = int(budget)
	vm.MaxSteps = 2000000000
	reserved := vm.AllocHeap(nil).I
	if reserved > int64(budget) {
		return fmt.Errorf("stage reservation%d exceeds budget%d", reserved, budget)
	}
	// This profile retains a near pointer ABI for the generator frame. All note
	// arena objects are accessed by handles; no high backing address is borrowed.
	if reserved >= 65536 {
		return fmt.Errorf("near frame region begins above 64 KiB: %d", reserved)
	}
	put16 := func(name string, value uint16) {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], value)
		vm.WriteHeapSlice(vm.GlobalAddr(name), b[:])
	}
	get16 := func(name string) uint16 { return binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr(name), 2)) }
	get32 := func(name string) uint32 { return binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr(name), 4)) }
	put16("note_count", uint16(len(in.Notes)))
	put16("mapped_bank", 65535)
	if backend != "flat" {
		vm.WriteHeap(vm.GlobalAddr("banked"), 1)
	}
	if backend == "churn" {
		vm.WriteHeap(vm.GlobalAddr("churn"), 1)
	}
	for i, n := range in.Notes {
		// Objects never straddle a physical page; each page has four spare bytes.
		at := vm.GlobalAddr("note_bytes") + int64((i/682)*4096+(i%682)*6)
		var record [6]byte
		binary.LittleEndian.PutUint16(record[0:2], uint16(n.Start))
		binary.LittleEndian.PutUint16(record[2:4], uint16(n.Off))
		record[4] = n.Pitch
		record[5] = byte(n.Source)
		vm.WriteHeapSlice(at, record[:])
	}
	if _, err = vm.Call("validate_input", nil); err != nil {
		return err
	}
	if f := get16("fault"); f != 0 {
		return fmt.Errorf("lazy input fault%d", f)
	}
	flattened := make([][]flatCell, len(in.Configs))
	for i, c := range in.Configs {
		flattened[i] = []flatCell{}
		// The sole host callback is an output sink. Every row/selection decision is
		// Nanz + actual exported Crystal MIR. Captured output is verification I/O,
		// outside the measured VM stage memory (native output will stream).
		vm.Hosts["@host.lazy.emit_flat"] = func(args []mir2.Value) ([]mir2.Value, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("invalid sink arity")
			}
			packed := uint16(args[1].I)
			row := uint32(args[0].I)
			pitch := byte(packed)
			kind := "s"
			if packed>>8 == 4 {
				kind = "r"
			} else if packed>>8 != 1 {
				return nil, fmt.Errorf("invalid flattened kind")
			}
			cells := flattened[i]
			if len(cells) == 0 || cells[len(cells)-1].Row != row {
				cells = append(cells, flatCell{Row: row, Notes: [][]any{}})
			}
			cells[len(cells)-1].Notes = append(cells[len(cells)-1].Notes, []any{pitch, kind})
			flattened[i] = cells
			return nil, nil
		}
		_, err = vm.Call("consume", []mir2.Value{{I: int64(c.Source)}, {I: int64(c.Mode[0])}, {I: int64(limit)}})
		if err != nil {
			return err
		}
		if f := get16("fault"); f != 0 {
			return fmt.Errorf("lazy stream fault%d", f)
		}
	}
	heap := vm.AllocHeap(nil).I
	if heap >= 65536 {
		return fmt.Errorf("generator frame escaped near region: %d", heap)
	}
	receipt := map[string]any{"protocol": "autosiril-lazy-flat-v1", "flattened": flattened, "backend": backend, "optimized": optimized, "notes": len(in.Notes), "note_capacity": 8184, "note_payload_bytes": len(in.Notes) * 6, "arena_reserved_bytes": 49152, "global_vm_reservation_bytes": reserved, "vm_heap_bytes": heap, "vm_stage_budget_bytes": budget, "active_capacity": 512, "peak_active": get16("peak_active"), "rows_visited": get32("rows_visited"), "yielded": get32("yielded"), "bank_switches": get32("bank_switches"), "payload_reads": get32("payload_reads"), "vm_steps": vm.StepsExecuted(), "limit": limit, "scope": "paired-notes to flattened entries; VM data/frame only; excludes native code, stack, MIDI parser, later stages and host verification I/O"}
	data, err = json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, data, 0644)
}
