package pipeline

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestShowcaseBufStreamNativeCapacityModel(t *testing.T) {
	const path = "examples/nanz/30_stream_test.nanz"
	source, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	for _, capacity := range []int64{0, 1, 3, 63, 64} {
		t.Run(fmt.Sprintf("cap%d", capacity), func(t *testing.T) {
			h, err := nanz.Parse(string(source), "30_stream_test.nanz")
			if err != nil {
				t.Fatal(err)
			}
			buffer := &hir.AddrOfExpr{Sym: "buf", Ty: mir2.TyPtr}
			stream := &hir.AddrOfExpr{Sym: "stream", Ty: mir2.TyPtr}
			body := []hir.Stmt{&hir.ExprStmt{Expr: &hir.CallExpr{Fn: "bufstream_init", Args: []hir.Expr{stream, buffer, hir.U16(capacity)}, Ty: mir2.TyVoid}}}
			// More writes than capacity proves dropped bytes cannot advance state or
			// overwrite guard/tail. Boundary bytes include 0xff and 0x00.
			for i := 0; i < 70; i++ {
				body = append(body, &hir.ExprStmt{Expr: &hir.CallExpr{Fn: "bufstream_write_u8", Args: []hir.Expr{stream, hir.U8(int64(uint8(i * 37)))}, Ty: mir2.TyVoid}})
			}
			body = append(body, &hir.ReturnStmt{})
			h.Funcs = append(h.Funcs, &hir.Func{Name: "oracle_stream", RetTy: mir2.TyVoid, Body: &hir.Block{Body: body}})
			// Initial bytes are explicit workload state shared by both machines.
			for i := range h.Globals {
				if h.Globals[i].Name == "buf" {
					h.Globals[i].Init = bytes.Repeat([]byte{0xa5}, 64)
				}
				if h.Globals[i].Name == "stream" {
					h.Globals[i].Init = bytes.Repeat([]byte{0xc3}, 6)
				}
			}
			steps, err := CompileHIRSteps(h)
			if err != nil {
				t.Fatal(err)
			}
			vm := mir2.NewVM(steps.MIR2Module)
			if _, err := vm.Call("oracle_stream", nil); err != nil {
				t.Fatal(err)
			}
			memory, symbols := nativeOracleRun(t, steps, []string{"oracle_stream"})
			expected := bytes.Repeat([]byte{0xa5}, 64)
			for i := int64(0); i < capacity; i++ {
				expected[i] = byte(i * 37)
			}
			addr := symbols["buf"]
			actualVM := vm.ReadHeap(vm.GlobalAddr("buf"), 64)
			actualZ80 := memory[addr : addr+64]
			if !bytes.Equal(actualVM, expected) || !bytes.Equal(actualZ80, expected) {
				t.Fatalf("full buffer VM=%x Z80=%x want=%x", actualVM, actualZ80, expected)
			}
			check := func(name string, data []byte, pointer uint16) {
				if binary.LittleEndian.Uint16(data) != pointer || binary.LittleEndian.Uint16(data[2:]) != uint16(capacity) || binary.LittleEndian.Uint16(data[4:]) != uint16(capacity) {
					t.Fatalf("%s descriptor=%x expected pointer=%04x expected pos/cap=%d", name, data, pointer, capacity)
				}
			}
			check("VM", vm.ReadHeap(vm.GlobalAddr("stream"), 6), uint16(vm.GlobalAddr("buf")))
			start := symbols["stream"]
			check("Z80", memory[start:start+6], symbols["buf"])
		})
	}
	t.Logf("ORACLE %s 5 capacities × 70 writes; full 64-byte buffer + 6-byte descriptor; VM/native independent model PASS", path)
}
