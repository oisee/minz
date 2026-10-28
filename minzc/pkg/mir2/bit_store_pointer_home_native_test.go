package mir2_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// bitStoreHomeCase is one load-modify-store bit update whose pointer lives in a
// fixed physical home. emitBitMemOp can only address (HL) at offset 0/1 and
// (IX+d)/(IY+d) with d in 0..127; every other home must keep the ordinary
// load / Or|And / store sequence the fusion scan suppressed.
type bitStoreHomeCase struct {
	home     string // pointer home: HL, DE, BC, IX, IY or spill
	word     bool   // u16 cell (false: u8)
	bit      int    // bit index within the whole cell
	set      bool   // SET (Or) or RES (And ^mask)
	explicit bool   // OpBitSet/OpBitReset instead of Or/And with a mask constant
}

// byteHomes returns byte-register homes that do not alias the pointer home.
func byteHomes(home string) []string {
	taken := map[string]bool{}
	for _, r := range []string{"A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL"} {
		taken[r] = false
	}
	switch home {
	case "HL":
		taken["H"], taken["L"] = true, true
	case "DE":
		taken["D"], taken["E"] = true, true
	case "BC":
		taken["B"], taken["C"] = true, true
	case "IX":
		taken["IXH"], taken["IXL"] = true, true
	case "IY":
		taken["IYH"], taken["IYL"] = true, true
	}
	var free []string
	for _, r := range []string{"A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL"} {
		if !taken[r] {
			free = append(free, r)
		}
	}
	return free
}

// pairHomes returns 16-bit homes that do not alias the pointer home.
func pairHomes(home string) []string {
	all := []string{"BC", "DE", "HL", "IX", "IY"}
	var free []string
	for _, p := range all {
		if p != home {
			free = append(free, p)
		}
	}
	return free
}

// buildBitStoreModule emits: cur = Load(ptr); next = cur OP mask; Store(ptr, next)
// and returns the reloaded cell so the emulator result observes memory.
func buildBitStoreModule(tc bitStoreHomeCase) (*mir2.Module, *mir2.AllocResult, mir2.Reg) {
	ty := mir2.TyU8
	if tc.word {
		ty = mir2.TyU16
	}
	m := &mir2.Module{Name: "bitstore"}
	f := m.AddFunc("bitstore")
	f.Contract.Returns = []mir2.Return{{Ty: ty, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	ptr := b.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
	cur := b.Load(ptr, ty, mir2.ClassGeneral)
	var next mir2.Reg
	if tc.explicit {
		if tc.set {
			next = b.BitSet(cur, tc.bit, ty, mir2.ClassGeneral)
		} else {
			next = b.BitReset(cur, tc.bit, ty, mir2.ClassGeneral)
		}
	} else {
		mask := uint64(1) << tc.bit
		if !tc.set {
			mask = ^mask
		}
		if tc.word {
			mask &= 0xFFFF
		} else {
			mask &= 0xFF
		}
		k := b.Const(int64(mask), ty, mir2.ClassGeneral)
		if tc.set {
			next = b.Or(cur, k, ty, mir2.ClassGeneral)
		} else {
			next = b.And(cur, k, ty, mir2.ClassGeneral)
		}
	}
	b.Store(ptr, next, ty)
	out := b.Load(ptr, ty, mir2.ClassAcc)
	b.Ret(out)

	locs := map[mir2.Reg]mir2.PhysLoc{}
	switch tc.home {
	case "spill":
		locs[ptr] = mir2.PhysLoc{Kind: mir2.LocMem, Name: "$F000", Offset: 0xf000}
	default:
		kind := mir2.LocReg
		if tc.home == "IX" || tc.home == "IY" {
			kind = mir2.LocIXY
		}
		locs[ptr] = mir2.PhysLoc{Kind: kind, Name: tc.home}
	}
	if tc.word {
		homes := pairHomes(tc.home)
		locs[cur] = mir2.PhysLoc{Kind: mir2.LocReg, Name: homes[0]}
		locs[next] = mir2.PhysLoc{Kind: mir2.LocReg, Name: homes[1]}
		locs[out] = mir2.PhysLoc{Kind: mir2.LocReg, Name: homes[2]}
	} else {
		homes := byteHomes(tc.home)
		locs[cur] = mir2.PhysLoc{Kind: mir2.LocReg, Name: homes[0]}
		locs[next] = mir2.PhysLoc{Kind: mir2.LocReg, Name: homes[1]}
		locs[out] = mir2.PhysLoc{Kind: mir2.LocReg, Name: "A"}
	}
	return m, &mir2.AllocResult{Locs: locs}, ptr
}

// TestBitStorePointerHomeNative drives the fused SET/RES store from every
// pointer home. BC and DE cannot encode SET/RES n,(rr); the fusion decision
// must be taken before the load and the Or|And producer are suppressed.
func TestBitStorePointerHomeNative(t *testing.T) {
	cases := []bitStoreHomeCase{}
	for _, home := range []string{"HL", "DE", "BC", "IX", "IY", "spill"} {
		for _, word := range []bool{false, true} {
			bits := []int{0, 2}
			if word {
				// Bit 9 addresses the high byte (byteOffset 1).
				bits = append(bits, 9)
			}
			for _, bit := range bits {
				for _, set := range []bool{true, false} {
					for _, explicit := range []bool{false, true} {
						cases = append(cases, bitStoreHomeCase{home, word, bit, set, explicit})
					}
				}
			}
		}
	}
	for _, tc := range cases {
		name := fmt.Sprintf("%s_w%v_b%v_%v_exp%v", tc.home, tc.word, tc.bit, setClear(tc.set), tc.explicit)
		t.Run(name, func(t *testing.T) {
			m, alloc, ptr := buildBitStoreModule(tc)
			asm := mustZ80Asm(mir2.Z80Codegen(m, alloc))
			// Two seeds so that for every bit index at least one of set/clear
			// actually flips the bit in at least one seed.
			for _, seed := range []uint32{0xA5C3, 0x5A3C} {
				init := fmt.Sprintf("LD HL,%d\nLD (0x4000),HL\n", seed&0xFFFF)
				if tc.home == "spill" {
					init += fmt.Sprintf("LD HL,0x4000\nLD (_spill_bitstore_r%d),HL\n", ptr)
				} else {
					init += fmt.Sprintf("LD %s,0x4000\n", tc.home)
				}
				regs := fix1Execute(t, "ORG 0x8000\nLD SP,0xFF00\n"+init+"CALL bitstore\nDI\nHALT\n"+asm)
				want := seed
				if tc.set {
					want |= 1 << tc.bit
				} else {
					want &= ^(uint32(1) << tc.bit)
				}
				if tc.word {
					if got := uint32(regs.HL) & 0xFFFF; got != want&0xFFFF {
						t.Fatalf("seed %04X cell got %04X want %04X\n%s", seed, got, want&0xFFFF, asm)
					}
				} else if got := uint32(regs.A) & 0xFF; got != want&0xFF {
					t.Fatalf("seed %04X cell got %02X want %02X\n%s", seed, got, want&0xFF, asm)
				}
			}
		})
	}
}

func setClear(set bool) string {
	if set {
		return "set"
	}
	return "clear"
}

// TestBitStoreAddrOfDirectPathNative: a global's address held in an index
// register is addressable for SET/RES, and the same store also qualifies for
// the u16 direct-address path (LD (sym),HL), which runs earlier in the store
// emitter. That path stores the value register as-is, so it must not be taken
// while the load and the Or|And producer are suppressed by the fusion.
func TestBitStoreAddrOfDirectPathNative(t *testing.T) {
	// HL is not a candidate here: the stored value is allocated to HL, and a
	// pointer and its value cannot share a home. DE and BC cannot encode the
	// memory operand, so the fusion never suppresses their producers.
	for _, home := range []string{"IX", "IY", "DE", "BC"} {
		for _, set := range []bool{true, false} {
			name := fmt.Sprintf("%s_%v", home, setClear(set))
			t.Run(name, func(t *testing.T) {
				kind := mir2.LocReg
				if home == "IX" || home == "IY" {
					kind = mir2.LocIXY
				}
				m := &mir2.Module{Name: "bitglobal"}
				m.Globals = append(m.Globals, mir2.Global{Name: "wbuf", Ty: &mir2.ArrayTy{Elem: mir2.TyU16, Len: 4, Layout: mir2.LayoutAoS}, Init: []byte{0x01, 0x01, 0, 0, 0, 0, 0, 0}})
				f := m.AddFunc("bitglobal")
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				ptr := b.AddrOf("wbuf", mir2.ClassPointer)
				cur := b.Load(ptr, mir2.TyU16, mir2.ClassPair)
				maskValue := int64(1) << 9
				if !set {
					maskValue = ^(1 << 9) & 0xFFFF
				}
				mask := b.Const(maskValue, mir2.TyU16, mir2.ClassPair)
				var next mir2.Reg
				if set {
					next = b.Or(cur, mask, mir2.TyU16, mir2.ClassPair)
				} else {
					next = b.And(cur, mask, mir2.TyU16, mir2.ClassPair)
				}
				b.Store(ptr, next, mir2.TyU16)
				out := b.Load(ptr, mir2.TyU16, mir2.ClassPair)
				b.Ret(out)
				free := pairHomes(home)
				locs := map[mir2.Reg]mir2.PhysLoc{
					ptr:  {Kind: kind, Name: home},
					cur:  {Kind: mir2.LocReg, Name: free[0]},
					mask: {Kind: mir2.LocReg, Name: free[1]},
					next: {Kind: mir2.LocReg, Name: "HL"},
					out:  {Kind: mir2.LocReg, Name: free[2]},
				}
				asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
				regs := fix1Execute(t, "ORG 0x8000\nLD SP,0xFF00\nLD HL,0\nCALL bitglobal\nDI\nHALT\n"+asm)
				want := uint16(0x0301)
				if !set {
					want = uint16(0x0101)
				}
				if regs.HL != want {
					t.Fatalf("cell got %04X want %04X\n%s", regs.HL, want, asm)
				}
			})
		}
	}
}
