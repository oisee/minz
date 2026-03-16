package rzx

import (
	"fmt"

	"github.com/minz/minzc/pkg/spectrum"
	"github.com/minz/minzc/pkg/spectrum/formats"
)

// Player drives RZX replay on a Machine, feeding recorded IN values
// frame by frame. Use Next() in a loop to advance one frame at a time.
type Player struct {
	Rec     *Recording
	Machine *spectrum.Machine
	Frame   int // current frame index (0-based)

	prevINValues []byte // for repeat frames
}

// NewPlayer creates an RZX player for the given recording and machine.
// If the recording contains an embedded snapshot, it is applied automatically.
func NewPlayer(rec *Recording, m *spectrum.Machine) (*Player, error) {
	// Apply embedded snapshot if present.
	if rec.Snapshot != nil {
		snap, err := formats.ParseSNA(rec.Snapshot)
		if err != nil {
			return nil, fmt.Errorf("rzx: apply snapshot: %w", err)
		}
		formats.ApplySnapshot(m, snap)
	}

	return &Player{
		Rec:     rec,
		Machine: m,
	}, nil
}

// Done reports whether all frames have been played.
func (p *Player) Done() bool {
	return p.Frame >= len(p.Rec.Frames)
}

// Next advances the emulator by one frame using recorded input.
// Returns false when all frames have been played.
func (p *Player) Next() bool {
	if p.Done() {
		return false
	}

	f := p.Rec.Frames[p.Frame]

	var inValues []byte
	if f.IsRepeat() {
		inValues = p.prevINValues
	} else {
		inValues = f.INValues
		p.prevINValues = inValues
	}

	p.Machine.RunRZXFrame(inValues)
	p.Frame++
	return true
}

// NextFast is like Next but uses RunRZXFrameFast (no per-T-state rendering).
func (p *Player) NextFast() bool {
	if p.Done() {
		return false
	}

	f := p.Rec.Frames[p.Frame]

	var inValues []byte
	if f.IsRepeat() {
		inValues = p.prevINValues
	} else {
		inValues = f.INValues
		p.prevINValues = inValues
	}

	p.Machine.RunRZXFrameFast(inValues)
	p.Frame++
	return true
}

// TotalFrames returns the number of frames in the recording.
func (p *Player) TotalFrames() int {
	return len(p.Rec.Frames)
}

// ScreenSCR returns a copy of the current VRAM as a 6912-byte .scr file.
func (p *Player) ScreenSCR() []byte {
	return p.Machine.ReadVRAM()
}
