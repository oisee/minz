package pipeline

import "testing"

// Pixel positions derive from an independent fixed wave and archived raster bytes.
func TestZXPixelWaveSceneMachine(t *testing.T) {
	wave := []int{0, 1, 1, 0, 0, -1, -1, 0}
	runZXTypedSpriteSceneMachine(t, "pixel-shifted-sprites/scene", "MINZ_PIXEL_SCENE_EXPORT", func(frame, row int) int { return wave[(frame+row)%8] }, true)
}
