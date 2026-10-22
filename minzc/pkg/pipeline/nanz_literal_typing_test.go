package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Independent mathematical-domain model: use the peer domain if possible,
// otherwise choose the smallest machine domain containing peer and literal.
func p28Domain(bits int, signed bool, k int64) (int, bool) {
	lo, hi := int64(0), int64(1)<<bits-1
	if signed {
		lo, hi = -(int64(1) << (bits - 1)), int64(1)<<(bits-1)-1
	}
	if k < lo {
		lo = k
	}
	if k > hi {
		hi = k
	}
	for _, w := range []int{8, 16, 24, 32} {
		if lo < 0 || signed {
			if lo >= -(int64(1)<<(w-1)) && hi < int64(1)<<(w-1) {
				return w, true
			}
		} else if hi < int64(1)<<w {
			return w, false
		}
	}
	panic("unrepresentable domain")
}

func p28Reference(a, b int64, op string, bits int, signed bool) int64 {
	var v int64
	switch op {
	case "<":
		if a < b {
			return 1
		}
		return 0
	case "<=":
		if a <= b {
			return 1
		}
		return 0
	case ">":
		if a > b {
			return 1
		}
		return 0
	case ">=":
		if a >= b {
			return 1
		}
		return 0
	case "==":
		if a == b {
			return 1
		}
		return 0
	case "!=":
		if a != b {
			return 1
		}
		return 0
	case "+":
		v = a + b
	case "-":
		v = a - b
	case "*":
		v = a * b
	case "/":
		v = a / b
	case "%":
		v = a % b
	case "&":
		v = a & b
	case "|":
		v = a | b
	case "xor":
		v = a ^ b
	case "<<":
		v = a << uint(b)
	case ">>":
		if signed {
			v = a >> uint(b)
		} else {
			v = int64(uint64(a) >> uint(b))
		}
	}
	// Arithmetic wraps at the selected width, then the explicit u16 result
	// conversion makes comparison of VM/emulator return bit patterns unambiguous.
	v &= int64(1)<<bits - 1
	if signed && v&(int64(1)<<(bits-1)) != 0 {
		v -= int64(1) << bits
	}
	return int64(uint16(v))
}

func TestNanzLiteralTypingPropertyVMZ80(t *testing.T) {
	for _, tc := range []struct {
		ty     string
		bits   int
		signed bool
		lo, hi int64
	}{
		{"i8", 8, true, -128, 127}, {"u8", 8, false, 0, 255},
		{"i16", 16, true, -32768, 32767}, {"u16", 16, false, 0, 65535},
	} {
		for _, op := range []string{"+", "-", "*", "/", "%", "&", "|", "xor", "<<", ">>", "<", "<=", ">", ">=", "==", "!="} {
			t.Run(tc.ty+"/"+op, func(t *testing.T) {
				var src strings.Builder
				type shape struct {
					name            string
					bits            int
					signed, compare bool
				}
				var shapes []shape
				var asserts []hir.Assert
				constants := []int64{tc.lo - 1, tc.lo, -1, 0, 1, 127, 128, 255, 256, tc.hi, tc.hi + 1}
				seen := map[int64]bool{}
				for _, k := range constants {
					if seen[k] {
						continue
					}
					seen[k] = true
					bits, signed := p28Domain(tc.bits, tc.signed, k)
					for side := 0; side < 2; side++ {
						if bits <= 16 && side == 1 && (op == "<<" || op == ">>") && (k < 0 || k >= int64(bits)) {
							continue
						}
						if side == 1 && (op == "/" || op == "%") && k == 0 {
							continue
						}
						name := fmt.Sprintf("p%d", len(shapes))
						l, r := "x", fmt.Sprint(k)
						if side == 0 {
							l, r = r, l
						}
						compare := strings.Contains(" < <= > >= == != ", " "+op+" ")
						ret, expr := "u16", fmt.Sprintf("((%s %s %s) as u16)", l, op, r)
						if compare {
							ret, expr = "bool", fmt.Sprintf("%s %s %s", l, op, r)
						}
						function := fmt.Sprintf("fun %s(x:%s)->%s{return %s}\n", name, tc.ty, ret, expr)
						if bits > 16 {
							if _, e := nanz.Parse(function, "wide.nanz"); e == nil || !strings.Contains(e.Error(), "explicit wide cast") {
								t.Fatalf("expected wide-cast diagnostic for %s: %v", function, e)
							}
							continue
						}
						src.WriteString(function)
						shapes = append(shapes, shape{name, bits, signed, compare})
						for _, x := range []int64{tc.lo, tc.lo + 1, -2, -1, 0, 1, 127, 128, 255, 256, tc.hi - 1, tc.hi} {
							if x < tc.lo || x > tc.hi {
								continue
							}
							a, b := x, k
							if side == 0 {
								a, b = k, x
							}
							// Only C-defined shift counts; other arithmetic has Nanz wrap semantics.
							if (op == "<<" || op == ">>") && (b < 0 || b >= int64(bits)) {
								continue
							}
							if op == "/" || op == "%" {
								if b == 0 || signed && a == -(int64(1)<<(bits-1)) && b == -1 {
									continue
								}
							}
							want := p28Reference(a, b, op, bits, signed)
							for _, engine := range []string{"mir2", "z80"} {
								asserts = append(asserts, hir.Assert{FuncName: name, Args: []int64{x}, Expected: want, Via: engine, Source: fmt.Sprintf("%s %s literal=%d side=%d x=%d", tc.ty, op, k, side, x)})
							}
						}
					}
				}
				parse := func() *hir.Module {
					h, e := nanz.Parse(src.String(), "p28.nanz")
					if e != nil {
						t.Fatal(e)
					}
					h.Sandboxes = []hir.Sandbox{{Name: "p28", Asserts: asserts}}
					return h
				}
				h := parse()
				// Check source expression types before optimizers can fold the evidence.
				for i, f := range h.Funcs {
					e := f.Body.Body[0].(*hir.ReturnStmt).Val
					if !shapes[i].compare {
						e = e.(*hir.CastExpr).X
						if e.ExprTy().Width() != shapes[i].bits || mir2.IsSigned(e.ExprTy()) != shapes[i].signed {
							t.Fatalf("%s: type %s want %d signed=%v", f.Name, e.ExprTy(), shapes[i].bits, shapes[i].signed)
						}
					}
				}
				raw := hir.LowerModule(h)
				if shapes[0].compare {
					for i, f := range raw.Funcs {
						found := false
						for _, b := range f.Blocks {
							for _, in := range b.Insts {
								if in.Op == mir2.OpCmp {
									found = true
									want := map[string]mir2.CmpCond{"<": mir2.CmpUlt, "<=": mir2.CmpUle, ">": mir2.CmpUgt, ">=": mir2.CmpUge, "==": mir2.CmpEq, "!=": mir2.CmpNe}[op]
									if shapes[i].signed {
										want = map[string]mir2.CmpCond{"<": mir2.CmpLt, "<=": mir2.CmpLe, ">": mir2.CmpGt, ">=": mir2.CmpGe, "==": mir2.CmpEq, "!=": mir2.CmpNe}[op]
									}
									if in.Cond != want || shapes[i].signed && (in.SrcTy == nil || in.SrcTy.Width() != shapes[i].bits) {
										t.Fatalf("%s predicate=%v source=%v want %v width=%d", f.Name, in.Cond, in.SrcTy, want, shapes[i].bits)
									}
								}
							}
						}
						if !found {
							t.Fatalf("%s: comparison missing", f.Name)
						}
					}
				}
				if !shapes[0].compare {
					for i, f := range raw.Funcs {
						expected := map[string]mir2.Op{"+": mir2.OpAdd, "-": mir2.OpSub, "*": mir2.OpMul, "/": mir2.OpDiv, "%": mir2.OpMod, "&": mir2.OpAnd, "|": mir2.OpOr, "xor": mir2.OpXor, "<<": mir2.OpShl, ">>": mir2.OpShr}[op]
						if shapes[i].signed && op == "/" {
							expected = mir2.OpSDiv
						}
						if shapes[i].signed && op == "%" {
							expected = mir2.OpSMod
						}
						if op == ">>" && shapes[i].signed {
							expected = mir2.OpSar
						}
						found := false
						for _, b := range f.Blocks {
							for _, in := range b.Insts {
								if in.Op == expected {
									found = true
									if in.Ty.Width() != shapes[i].bits || mir2.IsSigned(in.Ty) != shapes[i].signed {
										t.Fatalf("%s: MIR %v type %s", f.Name, in.Op, in.Ty)
									}
								}
							}
						}
						if !found {
							t.Fatalf("%s: missing MIR %v", f.Name, expected)
						}
					}
				}
				if e := RunAssertsMIR2(h, raw); e != nil {
					t.Fatalf("raw VM: %v", e)
				}
				h = parse()
				steps, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
				if e != nil {
					t.Fatal(e)
				}
				if e := RunAssertsMIR2(h, steps.MIR2Module); e != nil {
					t.Fatal(e)
				}
				p28RunNative(t, h, steps)
			})
		}
	}
}

func TestNanzLiteralTypingContextsVMZ80(t *testing.T) {
	t.Run("forward_initializer", func(t *testing.T) {
		text := `fun inferred(x:i8)->bool{let y=1+widen(x)
return y<0}
fun nested(x:i8)->bool{let y=(1+widen(x))+1
return y<0}
fun explicit(x:i8)->bool{let y:u8=1+widen(x)
return y<0}
fun widen(x:i8)->i16{return x as i16}`
		parse := func() *hir.Module {
			h, e := nanz.Parse(text, "forward-initializer.nanz")
			if e != nil {
				t.Fatal(e)
			}
			var asserts []hir.Assert
			for _, x := range []int64{-128, -3, -2, -1, 0, 1, 126, 127} {
				for _, name := range []string{"inferred", "nested", "explicit"} {
					want := int64(0)
					if name == "inferred" && x+1 < 0 || name == "nested" && x+2 < 0 {
						want = 1
					}
					for _, engine := range []string{"mir2", "z80"} {
						asserts = append(asserts, hir.Assert{FuncName: name, Args: []int64{x}, Expected: want, Via: engine, Source: fmt.Sprintf("%s at %d", name, x)})
					}
				}
			}
			h.Sandboxes = []hir.Sandbox{{Name: "forward", Asserts: asserts}}
			return h
		}
		h := parse()
		if e := RunAssertsMIR2(h, hir.LowerModule(h)); e != nil {
			t.Fatal(e)
		}
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		if e := RunAssertsMIR2(h, steps.MIR2Module); e != nil {
			t.Fatal(e)
		}
		p28RunNative(t, h, steps)
	})
	t.Run("signed_division", func(t *testing.T) {
		text := "fun sample(x:i16)->i16{return (x/256)%16}"
		parse := func() *hir.Module {
			h, e := nanz.Parse(text, "signed-division.nanz")
			if e != nil {
				t.Fatal(e)
			}
			var asserts []hir.Assert
			for _, x := range []int64{-32768, -32767, -4097, -257, -256, -255, -1, 0, 1, 255, 256, 257, 32767} {
				for _, engine := range []string{"mir2", "z80"} {
					asserts = append(asserts, hir.Assert{FuncName: "sample", Args: []int64{x}, Expected: (x / 256) % 16, Via: engine, Source: fmt.Sprintf("signed division chain at %d", x)})
				}
			}
			h.Sandboxes = []hir.Sandbox{{Name: "division", Asserts: asserts}}
			return h
		}
		h := parse()
		if e := RunAssertsMIR2(h, hir.LowerModule(h)); e != nil {
			t.Fatal(e)
		}
		steps, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		if e := RunAssertsMIR2(h, steps.MIR2Module); e != nil {
			t.Fatal(e)
		}
		p28RunNative(t, h, steps)
	})
	t.Run("diagnostics", func(t *testing.T) {
		for _, src := range []string{
			"fun f(x:u32)->u32{return x + -1}",
			"fun f(x:i32)->bool{return 4294967295 < x}",
			"fun f()->u32{return 4294967296}",
			"fun f()->i32{return -2147483649}",
			"fun f()->bool{return 4294967295 < word()} fun word()->i32{return 0}",
		} {
			if _, e := nanz.Parse(src, "domain.nanz"); e == nil {
				t.Errorf("expected domain error: %s", src)
			}
		}
		for _, src := range []string{
			"fun f(x:u32)->u32{return x + (-1 as u32)}",
			"fun f(x:i32)->bool{return (4294967295 as i32) < x}",
		} {
			if _, e := nanz.Parse(src, "cast.nanz"); e != nil {
				t.Errorf("explicit cast: %v", e)
			}
		}
	})
	for _, tc := range []struct {
		ty     string
		bits   int
		signed bool
		lo, hi int64
	}{
		{"i8", 8, true, -128, 127}, {"u8", 8, false, 0, 255},
		{"i16", 16, true, -32768, 32767}, {"u16", 16, false, 0, 65535},
	} {
		t.Run(tc.ty, func(t *testing.T) {
			var src strings.Builder
			fmt.Fprintf(&src, "fun identity(x:%s)->%s{return x}\n", tc.ty, tc.ty)
			var asserts []hir.Assert
			for i, k := range []int64{tc.lo - 1, tc.lo, 0, 1, tc.hi, tc.hi + 1} {
				forms := []string{
					fmt.Sprintf("let y:%s=%d\nreturn y", tc.ty, k),
					fmt.Sprintf("var y:%s=0\ny=%d\nreturn y", tc.ty, k),
					fmt.Sprintf("return identity(%d)", k),
					fmt.Sprintf("return %d", k),
				}
				for j, body := range forms {
					name := fmt.Sprintf("ctx%d_%d", i, j)
					fmt.Fprintf(&src, "fun %s()->%s{%s}\n", name, tc.ty, body)
					v := k & (int64(1)<<tc.bits - 1)
					if tc.signed && v&(int64(1)<<(tc.bits-1)) != 0 {
						v -= int64(1) << tc.bits
					}
					for _, engine := range []string{"mir2", "z80"} {
						asserts = append(asserts, hir.Assert{FuncName: name, Expected: v, Via: engine})
					}
				}
			}
			parse := func() *hir.Module {
				h, e := nanz.Parse(src.String(), "contexts.nanz")
				if e != nil {
					t.Fatal(e)
				}
				h.Sandboxes = []hir.Sandbox{{Name: "context", Asserts: asserts}}
				return h
			}
			h := parse()
			if e := RunAssertsMIR2(h, hir.LowerModule(h)); e != nil {
				t.Fatal(e)
			}
			h = parse()
			steps, e := CompileHIRSteps(h, Options{ContractOpt: true, AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
			}
			if e := RunAssertsMIR2(h, steps.MIR2Module); e != nil {
				t.Fatal(e)
			}
			p28RunNative(t, h, steps)
		})
	}
}

// Assemble the module and all input trampolines once. Each trampoline uses the
// production assertion bootstrap (including ABI placement and spill setup).
func p28RunNative(t *testing.T, h *hir.Module, steps Steps) {
	t.Helper()
	var cases []hir.Assert
	for _, sb := range h.Sandboxes {
		for _, a := range sb.Asserts {
			if a.Via == "z80" {
				cases = append(cases, a)
			}
		}
	}
	var asm strings.Builder
	asm.WriteString("ORG 0x8000\n")
	for i, a := range cases {
		fmt.Fprintf(&asm, "p28_assert_%d:\n", i)
		boot := buildAssertBootstrap(0x8000, a, steps.MIR2Module.FuncByName(a.FuncName), steps.Allocation)
		asm.WriteString(boot[strings.IndexByte(boot, '\n')+1:])
	}
	asm.WriteString(steps.Assembly)
	program, e := z80asm.NewAssembler().AssembleString(asm.String())
	if e != nil {
		t.Fatal(e)
	}
	if len(program.Errors) > 0 {
		t.Fatal(program.Errors)
	}
	if len(program.Binary) > 0x7000 {
		t.Fatal("test module overlaps assertion stack")
	}
	z := emulator.NewRemogattoZ80()
	if e := z.LoadMemory(0x8000, program.Binary); e != nil {
		t.Fatal(e)
	}
	funcs := map[string]*hir.Func{}
	for _, f := range h.Funcs {
		funcs[f.Name] = f
	}
	for i, a := range cases {
		z.Unhalt()
		z.SetPC(uint16(program.Symbols[fmt.Sprintf("p28_assert_%d", i)]))
		if e := z.Run(); e != nil {
			t.Fatalf("%s: %v", a.Source, e)
		}
		if e := checkAssertZ80Result(z, a, steps.MIR2Module.FuncByName(a.FuncName), funcs); e != nil {
			t.Fatal(e)
		}
	}
}
