package spectrum

// Keyboard implements the ZX Spectrum 8x5 matrix keyboard.
// 8 half-rows of 5 keys each, active low (0 = pressed, 1 = not pressed).
type Keyboard struct {
	rows [8]byte // half-row state, each bit 0-4 is a key (active low)
}

// NewKeyboard creates a keyboard with all keys released.
func NewKeyboard() *Keyboard {
	k := &Keyboard{}
	k.Reset()
	return k
}

// Reset releases all keys.
func (k *Keyboard) Reset() {
	for i := range k.rows {
		k.rows[i] = 0x1F // bits 0-4 all high = no keys pressed
	}
}

// KeyPress presses a key (row 0-7, bit 0-4).
func (k *Keyboard) KeyPress(row, bit int) {
	if row >= 0 && row < 8 && bit >= 0 && bit < 5 {
		k.rows[row] &^= 1 << uint(bit) // clear bit = pressed
	}
}

// KeyRelease releases a key.
func (k *Keyboard) KeyRelease(row, bit int) {
	if row >= 0 && row < 8 && bit >= 0 && bit < 5 {
		k.rows[row] |= 1 << uint(bit) // set bit = released
	}
}

// Read reads the keyboard state for the given high byte of the port address.
// The ULA selects half-rows via the high byte: each bit 0-7 selects a half-row
// (active low). Multiple rows can be selected simultaneously — results are ANDed.
func (k *Keyboard) Read(highByte byte) byte {
	result := byte(0x1F) // all released by default
	for row := 0; row < 8; row++ {
		if highByte&(1<<uint(row)) == 0 { // active low: 0 = selected
			result &= k.rows[row]
		}
	}
	return result
}

// Half-row layout (accent = active low bit position):
//
//   Row  Bit4    Bit3    Bit2    Bit1    Bit0   Port high byte
//   0    V       C       X       Z       Shift  $FE (0xFEFE)
//   1    G       F       D       S       A      $FD (0xFDFE)
//   2    T       R       E       W       Q      $FB (0xFBFE)
//   3    5       4       3       2       1      $F7 (0xF7FE)
//   4    6       7       8       9       0      $EF (0xEFFE)
//   5    Y       U       I       O       P      $DF (0xDFFE)
//   6    H       J       K       L       Enter  $BF (0xBFFE)
//   7    B       N       M       Sym     Space  $7F (0x7FFE)

// SpecKey identifies a ZX Spectrum key for the mapping table.
type SpecKey struct {
	Row, Bit int
}

// Standard key positions.
var (
	KeyShift = SpecKey{0, 0}
	KeyZ     = SpecKey{0, 1}
	KeyX     = SpecKey{0, 2}
	KeyC     = SpecKey{0, 3}
	KeyV     = SpecKey{0, 4}

	KeyA = SpecKey{1, 0}
	KeyS = SpecKey{1, 1}
	KeyD = SpecKey{1, 2}
	KeyF = SpecKey{1, 3}
	KeyG = SpecKey{1, 4}

	KeyQ = SpecKey{2, 0}
	KeyW = SpecKey{2, 1}
	KeyE = SpecKey{2, 2}
	KeyR = SpecKey{2, 3}
	KeyT = SpecKey{2, 4}

	Key1 = SpecKey{3, 0}
	Key2 = SpecKey{3, 1}
	Key3 = SpecKey{3, 2}
	Key4 = SpecKey{3, 3}
	Key5 = SpecKey{3, 4}

	Key0 = SpecKey{4, 0}
	Key9 = SpecKey{4, 1}
	Key8 = SpecKey{4, 2}
	Key7 = SpecKey{4, 3}
	Key6 = SpecKey{4, 4}

	KeyP = SpecKey{5, 0}
	KeyO = SpecKey{5, 1}
	KeyI = SpecKey{5, 2}
	KeyU = SpecKey{5, 3}
	KeyY = SpecKey{5, 4}

	KeyEnter = SpecKey{6, 0}
	KeyL     = SpecKey{6, 1}
	KeyK     = SpecKey{6, 2}
	KeyJ     = SpecKey{6, 3}
	KeyH     = SpecKey{6, 4}

	KeySpace = SpecKey{7, 0}
	KeySym   = SpecKey{7, 1}
	KeyM     = SpecKey{7, 2}
	KeyN     = SpecKey{7, 3}
	KeyB     = SpecKey{7, 4}
)
