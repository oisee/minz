package main

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// Unit controls use explicit scalar selector fixtures. verify_lazy.py uses the
// actual native Crystal exporter product and the fresh Ruby oracle instead.
func lazyFixture(t *testing.T, optimized bool) (*mir2.VM, func(string) uint32) {
	t.Helper()
	dir := t.TempDir()
	music := filepath.Join(dir, "music.lanz")
	meta := filepath.Join(dir, "manifest.json")
	source := `(fun action ((mask u8)) u8
(if (== mask (cast 0 u8)) (block (return (cast 0 u8))))
(if (== mask (cast 2 u8)) (block (return (cast 0 u8))))
(if (== mask (cast 4 u8)) (block (return (cast 4 u8))))
(return (cast 1 u8)))
(fun prefer ((current u8) (candidate u8) (mode u8)) u8
(if (== mode (cast 109 u8)) (block
 (if (> candidate current) (block (return candidate)))
 (return current)))
(if (< candidate current) (block (return candidate)))
(return current))`
	if err := os.WriteFile(music, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(manifest{Functions: []binding{{Source: "crystal_cell_action", Symbol: "action"}, {Source: "crystal_prefer", Symbol: "prefer"}}})
	if err := os.WriteFile(meta, data, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := lazyModule("../../../research/autosiril-vm/lazy_flatten.nanz", music, meta, optimized)
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(m)
	vm.MaxMemory = 65536
	vm.MaxSteps = 10000000
	get := func(name string) uint32 {
		w := 4
		if name == "fault" || name == "mapped_bank" || name == "peak_active" {
			w = 2
		}
		b := vm.ReadHeap(vm.GlobalAddr(name), w)
		if w == 2 {
			return uint32(binary.LittleEndian.Uint16(b))
		}
		return binary.LittleEndian.Uint32(b)
	}
	return vm, get
}
func setLazy16(vm *mir2.VM, name string, v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	vm.WriteHeapSlice(vm.GlobalAddr(name), b[:])
}
func putLazyNote(vm *mir2.VM, index int, start, off uint16, pitch, source byte) {
	at := vm.GlobalAddr("note_bytes") + int64((index/682)*4096+(index%682)*6)
	var b [6]byte
	binary.LittleEndian.PutUint16(b[:2], start)
	binary.LittleEndian.PutUint16(b[2:4], off)
	b[4] = pitch
	b[5] = source
	vm.WriteHeapSlice(at, b[:])
}

func TestLazyArenaControls(t *testing.T) {
	for _, optimized := range []bool{false, true} {
		t.Run(map[bool]string{false: "raw", true: "optimized"}[optimized], func(t *testing.T) {
			vm, get := lazyFixture(t, optimized)
			if _, err := vm.Call("factory_probe", nil); err != nil {
				t.Fatal(err)
			}
			if get("rows_visited") != 0 || get("yielded") != 0 || get("payload_reads") != 0 {
				t.Fatal("factory ran producer body")
			}
			setLazy16(vm, "note_count", 683)
			setLazy16(vm, "mapped_bank", 65535)
			for i := 0; i < 683; i++ {
				putLazyNote(vm, i, 0, 0, 60, 0)
			}
			putLazyNote(vm, 682, 513, 1027, 91, 1)
			vm.WriteHeap(vm.GlobalAddr("banked"), 1)
			vm.WriteHeap(vm.GlobalAddr("churn"), 1)
			for _, tc := range []struct{ id, field, want int64 }{{682, 4, 60}, {683, 0, 1}, {683, 1, 2}, {683, 2, 3}, {683, 3, 4}, {683, 4, 91}, {683, 5, 1}} {
				out, err := vm.Call("read_byte", []mir2.Value{{I: tc.id}, {I: tc.field}})
				if err != nil || len(out) != 1 || out[0].I != tc.want {
					t.Fatalf("bank boundary: %v %v", out, err)
				}
			}
			if get("bank_switches") == 0 {
				t.Fatal("churn did not remap")
			}
			if _, err := vm.Call("stale_probe", nil); err != nil {
				t.Fatal(err)
			}
			if get("fault") != 102 {
				t.Fatal("stale lease accepted")
			}
		})
	}
}

func TestLazyDemandOrderAndExhaustion(t *testing.T) {
	for _, optimized := range []bool{false, true} {
		vm, get := lazyFixture(t, optimized)
		setLazy16(vm, "note_count", 2)
		putLazyNote(vm, 0, 0, 60000, 60, 0)
		putLazyNote(vm, 1, 50000, 50001, 64, 0)
		var values []uint16
		vm.Hosts["@host.lazy.emit_flat"] = func(args []mir2.Value) ([]mir2.Value, error) {
			values = append(values, uint16(args[1].I))
			return nil, nil
		}
		if _, err := vm.Call("consume", []mir2.Value{{I: 0}, {I: 112}, {I: 1}}); err != nil {
			t.Fatal(err)
		}
		if len(values) != 1 || values[0] != 316 || get("rows_visited") != 1 || get("yielded") != 1 {
			t.Fatalf("read beyond demand: %v rows%d", values, get("rows_visited"))
		}
		// A fresh consumer can acquire the scratch region after abandonment.
		values = nil
		if _, err := vm.Call("consume", []mir2.Value{{I: 0}, {I: 112}, {I: 100}}); err != nil {
			t.Fatal(err)
		}
		if get("fault") != 0 || len(values) != 5 {
			t.Fatalf("resume/new stream: %v fault%d", values, get("fault"))
		}
		done, err := vm.Call("exhaustion_probe", []mir2.Value{{I: 0}, {I: 112}})
		if err != nil || len(done) != 1 || done[0].I != 0 {
			t.Fatalf("exhaustion changed output/effects: %v %v", done, err)
		}
		// Invalid onset ordering must be rejected, never silently sorted/reordered.
		putLazyNote(vm, 0, 10, 11, 60, 0)
		putLazyNote(vm, 1, 0, 1, 64, 0)
		if _, err := vm.Call("validate_input", nil); err != nil {
			t.Fatal(err)
		}
		if get("fault") != 106 {
			t.Fatal("source disorder accepted")
		}
	}
}

func TestLazyCapacityErrors(t *testing.T) {
	vm, get := lazyFixture(t, false)
	setLazy16(vm, "note_count", 1)
	putLazyNote(vm, 0, 0, 1, 60, 0)
	out, err := vm.Call("exclusive_probe", nil)
	if err != nil || len(out) != 1 || out[0].I != 107 {
		t.Fatalf("concurrent scratch ownership accepted: %v %v", out, err)
	}
	vm, get = lazyFixture(t, false)
	setLazy16(vm, "note_count", 8185)
	if _, err := vm.Call("validate_input", nil); err != nil {
		t.Fatal(err)
	}
	if get("fault") != 104 {
		t.Fatal("note capacity accepted")
	}
	vm, get = lazyFixture(t, false)
	setLazy16(vm, "note_count", 513)
	for i := 0; i < 513; i++ {
		putLazyNote(vm, i, 0, 1, 60, 0)
	}
	if _, err := vm.Call("consume", []mir2.Value{{I: 0}, {I: 112}, {I: 1}}); err != nil {
		t.Fatal(err)
	}
	if get("fault") != 108 {
		t.Fatal("active capacity accepted")
	}
}
