package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/z80asm"
)

// Force a tuple result into a saved byte register and an unsaved IY half.
// Both pickup orders conflict with the caller saves, and every ordinary
// scratch pair is saved, so pickup must read the stack snapshot through IX.
func TestCallResultInIYHalfWithSavedPairs(t *testing.T) {
	for _, half := range []string{"IYH", "IYL"} {
		t.Run(half, func(t *testing.T) {
			var sb strings.Builder
			ar := &AllocResult{Locs: map[Reg]PhysLoc{
				1: {Kind: LocReg, Name: "B"},
				2: {Kind: LocReg, Name: half},
			}}
			g := &z80cg{ar: ar, sb: &sb}
			g.emit("    ORG 0x8000")
			for _, pair := range []string{"AF", "BC", "DE", "HL"} {
				g.emitf("    PUSH %s", pair)
			}
			g.pickupCallResults([]parallelCopy{
				{srcName: "A", dstName: g.loc(1), ty: TyU8},
				{srcName: "C", dstName: g.loc(2), ty: TyU8},
			}, []string{"AF", "BC", "DE", "HL"}, false)
			g.emit("    RET")
			asm := sb.String()
			if strings.Contains(asm, "LD "+half+", (IX+") {
				t.Fatalf("illegal indexed half load:\n%s", asm)
			}
			if !strings.Contains(asm, "LD "+half+", A") {
				t.Fatalf("missing legal transfer through A:\n%s", asm)
			}
			if _, err := z80asm.NewAssembler().AssembleString(asm); err != nil {
				t.Fatalf("assemble: %v\n%s", err, asm)
			}
		})
	}
}
