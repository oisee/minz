package main

import (
	"encoding/binary"
	"testing"

	"github.com/minz/minzc/pkg/abap"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

// MZV replaces the SY accessors with host functions. SY-TABIX must use the
// same four-byte backing as compiled LOOP AT/READ TABLE/APPEND, including the
// restoration of an outer loop's row number after a nested loop.
func TestABAPInternalTableTabixHost(t *testing.T) {
	m, err := abap.Compile(`REPORT ztabix.
TYPES: BEGIN OF ty_row,
         v TYPE i,
       END OF ty_row.
DATA row TYPE ty_row.
DATA inner TYPE ty_row.
DATA rows TYPE STANDARD TABLE OF ty_row INITIAL SIZE 3.
DATA x TYPE i.
APPEND row TO rows.
APPEND row TO rows.
x = sy-tabix.
LOOP AT rows INTO row.
  LOOP AT rows INTO inner.
  ENDLOOP.
  x = x * 10 + sy-tabix.
ENDLOOP.
READ TABLE rows INTO row INDEX 2.
x = x * 10 + sy-tabix.`, "ztabix")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
		v := mir2.NewVM(module)
		registerABAPHosts(v, true, false)
		if _, err := v.Call("main", nil); err != nil {
			t.Fatal(err)
		}
		if got := int32(binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr("x"), 4))); got != 2122 {
			t.Fatalf("x=%d want 2122", got)
		}
		if got := int32(binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr("_abap_tabix_value"), 4))); got != 2 {
			t.Fatalf("SY-TABIX backing %d want 2", got)
		}
	}
}
