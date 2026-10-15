package pipeline

import (
	"fmt"
	"testing"
)

// Current automatic product, not the frozen historical menu fixture. Exercise
// two-/four-line transitions and gauge saturation with the existing independent
// game/clock oracle, complete fetched ULA image and PC/SP/INT observer.
func TestTetrisLineProgressionMultiClearMachine(t *testing.T) {
	for _, rows := range []int{2, 4} {
		for _, threshold := range []int{16, 24, 32, 40, 48, 56, 64, 160} {
			lines := threshold - rows
			t.Run(fmt.Sprintf("clear-%d/lines-%d", rows, lines), func(t *testing.T) {
				runTetrisMainMachineConfigured(t, "../../../examples/zx/tetris_playable.nanz", false, 4, true, rows, lines)
			})
		}
	}
}
