package pipeline

import (
	"github.com/minz/minzc/pkg/nanz"
	"strings"
	"testing"
)

// Expected displacements are hand specified, independent of generated
// placement/address helpers. Inspect actual ULA frames and raw/final VM RAM.
func TestZXSpriteRowEffectMachine(t *testing.T) {
	wave := []int{0, 1, 1, 0, 0, -1, -1, 0}
	runZXTypedSpriteSceneMachine(t, "typed-sprite-effects", "MINZ_SPRITE_EFFECT_EXPORT", func(frame, row int) int { return wave[(frame+row)%8] })
}

func TestZXSpriteRowShiftAdmission(t *testing.T) {
	for _, field := range []string{"row_shifts 1", "row_shifts\n row_shifts"} {
		src := "import zx.sprite\n@zx_sprite(\"Invalid\") {\n size 8 1\n pixels \"########\"\n" + field + "\n}"
		_, err := nanz.ParseWithOpts(src, "invalid-row-shifts.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil || !strings.Contains(err.Error(), "row_shifts") {
			t.Fatalf("%s: %v", field, err)
		}
	}
}
