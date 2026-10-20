package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

// Division truncates toward zero; remainder has the dividend's sign. Keep
// constants and runtime divisors, and isolated and shared modules, in the same
// table so code generation cannot accidentally depend on another function.
func TestDivModModuleVMZ80(t *testing.T) {
	for _, typ := range []struct {
		name             string
		values, divisors []int64
		bits             uint
		signed           bool
	}{
		{"i16", []int64{7, -7, 0, 1, -1, 127, -128, 255, -256, 32767, -32768}, []int64{-2, 2, -3, 3, -1, 1, -16, 16, -32768}, 16, true},
		{"i8", []int64{7, -7, 0, 1, -1, 63, -64, 127, -128}, []int64{-2, 2, -3, 3, -1, 1, -16, 16, -128}, 8, true},
		{"u8", []int64{0, 1, 7, 127, 128, 254, 255}, []int64{1, 2, 3, 16, 128, 255}, 8, false},
		{"u16", []int64{0, 1, 7, 127, 128, 255, 256, 32767, 32768, 65535}, []int64{1, 2, 3, 16, 256, 32768, 65535}, 16, false},
	} {
		for _, divisor := range typ.divisors {
			for _, layout := range []string{"shared", "combined", "div", "mod"} {
				t.Run(fmt.Sprintf("%s/%d/%s", typ.name, divisor, layout), func(t *testing.T) {
					var source strings.Builder
					ops := []string{"div", "mod"}
					if layout == "div" || layout == "mod" {
						ops = []string{layout}
					}
					for _, kind := range []string{"constant", "variable"} {
						params, rhs := "a:"+typ.name, fmt.Sprint(divisor)
						if kind == "variable" {
							params += ",b:" + typ.name
							rhs = "b"
						}
						if layout == "combined" {
							fmt.Fprintf(&source, "fun both_%s(%s)->%s {var q:%s=a / %s\nvar r:%s=a %% %s\nreturn q+r}\n", kind, params, typ.name, typ.name, rhs, typ.name, rhs)
						} else {
							for _, op := range ops {
								symbol := "/"
								if op == "mod" {
									symbol = "%"
								}
								fmt.Fprintf(&source, "fun %s_%s(%s)->%s {return a %s %s}\n", op, kind, params, typ.name, symbol, rhs)
							}
						}
					}
					h, err := nanz.Parse(source.String(), "divmod-module.nanz")
					if err != nil {
						t.Fatal(err)
					}
					raw := mir2.NewVM(hir.LowerModule(h))
					wrap := func(v int64) int64 {
						v &= (1 << typ.bits) - 1
						if typ.signed && v&(1<<(typ.bits-1)) != 0 {
							v -= 1 << typ.bits
						}
						return v
					}
					for _, a := range typ.values {
						for _, kind := range []string{"constant", "variable"} {
							names := ops
							if layout == "combined" {
								names = []string{"both"}
							}
							for _, op := range names {
								want := a / divisor
								if op == "mod" {
									want = a % divisor
								}
								if op == "both" {
									want = a/divisor + a%divisor
								}
								want = wrap(want)
								name := op + "_" + kind
								args := []int64{a}
								vals := []mir2.Value{{I: a}}
								if kind == "variable" {
									args = append(args, divisor)
									vals = append(vals, mir2.Value{I: divisor})
								}
								got, err := raw.Call(name, vals)
								if err != nil || len(got) != 1 || wrap(got[0].I) != want {
									t.Fatalf("raw %s%v=%v err=%v want %d", name, args, got, err, want)
								}
								h.Asserts = append(h.Asserts, hir.Assert{FuncName: name, Args: args, Expected: want & ((1 << typ.bits) - 1), Source: fmt.Sprintf("%s%v == %d", name, args, want)})
							}
						}
					}
					if _, err = CompileHIRSteps(h, DefaultOptions()); err != nil {
						t.Fatal(err)
					}
					t.Logf("%d cases passed raw MIR2, optimized MIR2, and Z80", len(h.Asserts))
				})
			}
		}
	}
}
