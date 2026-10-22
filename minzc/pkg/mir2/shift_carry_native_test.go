package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// Predicate producers precede a destructive shift into the source's home.
// Reading a deferred BIT from that home would test the relocated/new value.
// Refused forms must retain executable AND/zero producers for ordinary CMP.
func TestShiftCarryEmissionNative(t *testing.T) {
	for _, width := range []int{8, 16} {
		for _, right := range []bool{false, true} {
			for _, deferred := range []bool{false, true} {
				for _, kind := range []string{"and-ne", "and-eq", "bitget", "count-two", "wrong-bit", "intervening-add", "indexed-result", "spilled-result"} {
					t.Run(fmt.Sprintf("u%d/right%v/deferred%v/%s", width, right, deferred, kind), func(t *testing.T) {
						ty, cls, srcHome, maskHome, constantHome, resultHome := mir2.TyU8, mir2.ClassGeneral, "A", "B", "C", "A"
						if width == 16 {
							ty, cls, srcHome, maskHome, constantHome, resultHome = mir2.TyU16, mir2.ClassPointer, "HL", "BC", "DE", "HL"
						}
						if kind == "indexed-result" {
							resultHome = "IYH"
							if width == 16 {
								resultHome = "IY"
							}
						}
						if kind == "spilled-result" {
							resultHome = "$F000"
						}
						bit, count := width-1, 1
						if right {
							bit = 0
						}
						if kind == "wrong-bit" {
							bit = 1
						}
						if kind == "count-two" {
							count = 2
						}
						m := &mir2.Module{}
						f := m.AddFunc("carry_probe")
						f.Contract.Returns = []mir2.Return{{Ty: ty, Class: cls}}
						if width == 8 {
							f.Contract.Returns[0].Class = mir2.ClassAcc
						}
						b := mir2.NewBuilder(f)
						b.SwitchToNewBlock("entry")
						x := b.Param("x", ty, cls)
						mask := b.Const(1<<bit, ty, cls)
						test := b.And(x, mask, ty, cls)
						if kind == "bitget" {
							b.Cur.Insts = b.Cur.Insts[:len(b.Cur.Insts)-1]
							test = b.BitGet(x, bit, ty, mir2.ClassGeneral)
							maskHome = "B"
						}
						zero := b.Const(0, ty, cls)
						k := b.Const(int64(count), ty, cls)
						shift := b.Shl(x, k, ty, cls)
						if right {
							b.Cur.Insts[len(b.Cur.Insts)-1].Op = mir2.OpShr
						}
						locs := map[mir2.Reg]mir2.PhysLoc{}
						put := func(r mir2.Reg, name string) { locs[r] = mir2.PhysLoc{Kind: mir2.LocReg, Name: name} }
						put(x, srcHome)
						put(mask, constantHome)
						put(test, maskHome)
						put(zero, constantHome)
						put(k, constantHome)
						put(shift, resultHome)
						if kind == "spilled-result" {
							locs[shift] = mir2.PhysLoc{Kind: mir2.LocMem, Name: resultHome, Offset: 0xf000}
						}
						if kind == "intervening-add" {
							r := b.Add(zero, mask, ty, cls)
							home := "D"
							if width == 16 {
								home = "IX"
							}
							put(r, home)
						}
						cond := mir2.CmpNe
						if kind == "and-eq" {
							cond = mir2.CmpEq
						}
						flag := b.Cmp(cond, test, zero, mir2.ClassFlag, false)
						locs[flag] = mir2.PhysLoc{Kind: mir2.LocFlag, Name: "F"}
						if deferred {
							insts := b.Cur.Insts
							cmp := insts[len(insts)-1]
							insts = insts[:len(insts)-1]
							at := 0
							for n, i := range insts {
								if i.Dst == shift {
									at = n
									break
								}
							}
							insts = append(insts, nil)
							copy(insts[at+1:], insts[at:len(insts)-1])
							insts[at] = cmp
							b.Cur.Insts = insts
						}
						b.BrIf(flag, "yes", nil, "no", nil)
						b.SwitchToNewBlock("yes")
						poly := b.Const(7, ty, cls)
						out := b.Xor(shift, poly, ty, cls)
						put(poly, constantHome)
						put(out, srcHome)
						b.Ret(out)
						b.SwitchToNewBlock("no")
						b.Ret(shift)
						asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
						fused := kind == "and-ne" || kind == "and-eq" || kind == "bitget"
						if fused && (strings.Contains(asm, "    BIT ") || strings.Contains(asm, "    AND ") || strings.Contains(asm, "    CP ")) {
							t.Fatalf("unfused:\n%s", asm)
						}
						if !fused && !strings.Contains(asm, "    CP ") && !strings.Contains(asm, "    BIT ") && !strings.Contains(asm, "    AND ") {
							t.Fatalf("missing fallback predicate:\n%s", asm)
						}
						vm := mir2.NewVM(m)
						inputs := make([]int, 256)
						for v := range inputs {
							inputs[v] = v
							if width == 16 {
								inputs[v] = ((v*73+37)&255)<<8 | v
							}
						}
						if width == 16 {
							inputs = append(inputs, 0, 1, 0x80, 0x100, 0x7fff, 0x8000, 0x8001, 0xff00, 0xfffe, 0xffff)
						}
						for _, xval := range inputs {
							want := (xval << count) & ((1 << width) - 1)
							if right {
								want = xval >> count
							}
							yes := xval&(1<<bit) != 0
							if kind == "and-eq" {
								yes = !yes
							}
							if yes {
								want ^= 7
							}
							got, e := vm.Call("carry_probe", []mir2.Value{{I: int64(xval)}})
							if e != nil || len(got) != 1 || got[0].I != int64(want) {
								t.Fatalf("x%d VM%v err%v want%d", xval, got, e, want)
							}
							init := fmt.Sprintf("LD %s,%d\n", srcHome, xval)
							regs := fix1Execute(t, "ORG 0x8000\nLD SP,0xFF00\n"+init+"CALL carry_probe\nDI\nHALT\n"+asm)
							result := int(regs.A)
							if width == 16 {
								result = int(regs.HL)
							}
							if result != want || regs.SP != 0xff00 {
								t.Fatalf("x%d got%d SP%x want%d\n%s", xval, result, regs.SP, want, asm)
							}
						}
					})
				}
			}
		}
	}
}
