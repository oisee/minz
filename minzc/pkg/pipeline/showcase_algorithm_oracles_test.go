package pipeline

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"os"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pascal"
)

func TestShowcasePopcountAllBytesNativeModel(t *testing.T) {
	const path = "examples/nanz/04_lut_popcount.nanz"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.Parse(string(src), "04_lut_popcount.nanz")
	if err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 256; x++ {
		h.Asserts = append(h.Asserts, hir.Assert{FuncName: "popcount", Args: []int64{int64(x)}, Expected: int64(bits.OnesCount8(uint8(x)))})
	}
	if _, err := CompileHIRSteps(h); err != nil {
		t.Fatal(err)
	}
	t.Logf("ORACLE %s all256 popcount inputs VM+Z80 independent bits.OnesCount8 PASS", path)
}

func TestShowcaseStateMachineNativeTransitionModel(t *testing.T) {
	const path = "examples/nanz/12_state_machine.nanz"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.Parse(string(src), "12_state_machine.nanz")
	if err != nil {
		t.Fatal(err)
	}
	h.Asserts = nil
	table := [4][4]int64{{0, 1, 2, 3}, {0, 1, 2, 3}, {0, 0, 0, 3}, {3, 3, 3, 3}}
	for state := int64(0); state < 4; state++ {
		for _, event := range []int64{0, 1, 2, 3, 4, 127, 255} {
			want := table[state][0]
			if state == 1 && event > 3 {
				want = 1
			}
			if event < 4 {
				want = table[state][event]
			}
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: "next_state", Args: []int64{state, event}, Expected: want})
		}
		alive := int64(1)
		if state == 3 {
			alive = 0
		}
		h.Asserts = append(h.Asserts, hir.Assert{FuncName: "is_alive", Args: []int64{state}, Expected: alive}, hir.Assert{FuncName: "state_speed", Args: []int64{state}, Expected: []int64{0, 2, 4, 0}[state]})
	}
	if _, err := CompileHIRSteps(h); err != nil {
		t.Fatal(err)
	}
	t.Logf("ORACLE %s 36 VM+Z80 state/event/property calls independent transition table PASS", path)
}

func TestShowcaseLizpCryptoNativeByteModels(t *testing.T) {
	const path = "examples/lizp/crypto.lizp"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := lizp.Compile(string(src), "crypto.lizp")
	if err != nil {
		t.Fatal(err)
	}
	h.Asserts = nil
	values := []int64{0, 1, 25, 26, 64, 65, 90, 91, 127, 128, 254, 255}
	for _, a := range values {
		for _, b := range []int64{0, 1, 25, 26, 255} {
			for _, f := range []string{"xor_encrypt", "xor_decrypt"} {
				h.Asserts = append(h.Asserts, hir.Assert{FuncName: f, Args: []int64{a, b}, Expected: a ^ b})
			}
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: "hash_combine", Args: []int64{a, b}, Expected: ((a * 6) & 255) ^ b})
			want := a
			if a >= 65 && a <= 90 {
				want = 65 + (((a - 65 + b) & 255) % 26)
			}
			h.Asserts = append(h.Asserts, hir.Assert{FuncName: "caesar_encrypt", Args: []int64{a, b}, Expected: want})
		}
	}
	for _, p := range [][3]int64{{0, 0, 0}, {255, 255, 255}, {128, 127, 1}, {1, 2, 3}, {10, 20, 30}} {
		h.Asserts = append(h.Asserts, hir.Assert{FuncName: "checksum_xor", Args: p[:], Expected: p[0] ^ p[1] ^ p[2]}, hir.Assert{FuncName: "checksum_add", Args: p[:], Expected: (p[0] + p[1] + p[2]) & 255})
	}
	if _, err := CompileHIRSteps(h); err != nil {
		t.Fatal(err)
	}
	t.Logf("ORACLE %s 250 VM+Z80 byte crypto/checksum calls; unsigned wrap model PASS", path)
}

func TestShowcasePascalSieveNativeFullMemoryModel(t *testing.T) {
	const path = "examples/pascal/sieve.pas"
	src, err := os.ReadFile("../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := pascal.Compile(string(src), "sieve.pas")
	if err != nil {
		t.Fatal(err)
	}
	main := h.FuncByName("main")
	if main == nil {
		t.Fatal("missing source main")
	}
	// The compute workload is the original main prefix before CP/M WriteLn.
	// No computational statement is altered; console output needs BDOS separately.
	body := append([]hir.Stmt(nil), main.Body.Body...)
	end := len(body)
	for i, s := range body {
		if x, ok := s.(*hir.ExprStmt); ok {
			if c, ok := x.Expr.(*hir.CallExpr); ok && (c.Fn == "WriteI16" || c.Fn == "WriteU16" || c.Fn == "WriteU8" || c.Fn == "WriteCrLf" || c.Fn == "WriteStr") {
				end = i
				break
			}
		}
	}
	if end == len(body) {
		t.Fatal("expected explicit console boundary")
	}
	h.Funcs = append(h.Funcs, &hir.Func{Name: "oracle_sieve", RetTy: mir2.TyVoid, Body: &hir.Block{Body: append(body[:end], &hir.ReturnStmt{})}})
	s, err := CompileHIRSteps(h)
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(s.MIR2Module)
	if _, err := vm.Call("oracle_sieve", nil); err != nil {
		t.Fatal(err)
	}
	memory, symbols := nativeOracleRun(t, s, []string{"oracle_sieve"})
	// Flags index i represents odd number2i+3. Derive primality independently
	// by trial division instead of translating the sieve algorithm.
	want := make([]byte, 101)
	count := 0
	for i := range want {
		number := 2*i + 3
		prime := true
		for d := 2; d*d <= number; d++ {
			if number%d == 0 {
				prime = false
				break
			}
		}
		if prime {
			want[i] = 1
			count++
		}
	}
	gotVM := vm.ReadHeap(vm.GlobalAddr("FLAGS"), 101)
	addr := symbols["FLAGS"]
	gotZ80 := memory[addr : addr+101]
	if !bytes.Equal(gotVM, want) || !bytes.Equal(gotZ80, want) {
		t.Fatalf("FLAGS VM=%v Z80=%v want=%v", gotVM, gotZ80, want)
	}
	zaddr := symbols["COUNT"]
	zcount := binary.LittleEndian.Uint16(memory[zaddr : zaddr+2])
	vmcount := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("COUNT"), 2))
	if int(zcount) != count || int(vmcount) != count {
		t.Fatalf("count VM=%d Z80=%d want=%d", vmcount, zcount, count)
	}
	t.Logf("ORACLE %s 101 flags and prime count=%d VM+Z80 independent trial division PASS", path, count)
}
