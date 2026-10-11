package pipeline

import "fmt"

type memoryHostByteRecipe struct {
	Address uint16
	Write   bool
}

// memoryHostMemoryRecipe decodes only the supported data-access opcodes from
// actual bytes. Every address uses registers captured before the instruction.
// Unsupported encodings never acquire global, spill or stack permission.
func memoryHostMemoryRecipe(m *memoryHostZ80Memory, pc uint16) ([]memoryHostByteRecipe, error) {
	b := m.data[pc]
	next := func(n int) byte { return m.data[uint16(int(pc)+n)] }
	word := func(n int) uint16 { return uint16(next(n)) | uint16(next(n+1))<<8 }
	bytes := func(a uint16, n int, write bool) []memoryHostByteRecipe {
		r := []memoryHostByteRecipe{}
		for i := 0; i < n; i++ {
			r = append(r, memoryHostByteRecipe{uint16(int(a) + i), write})
		}
		return r
	}
	stackPush := func() []memoryHostByteRecipe {
		sp := m.cpu.SP()
		return []memoryHostByteRecipe{{sp - 1, true}, {sp - 2, true}}
	}
	stackPop := func() []memoryHostByteRecipe { return bytes(m.cpu.SP(), 2, false) }
	condition := func(c byte) bool {
		f := m.cpu.F
		switch c {
		case 0:
			return f&0x40 == 0
		case 1:
			return f&0x40 != 0
		case 2:
			return f&1 == 0
		case 3:
			return f&1 != 0
		case 4:
			return f&4 == 0
		case 5:
			return f&4 != 0
		case 6:
			return f&0x80 == 0
		case 7:
			return f&0x80 != 0
		}
		return false
	}
	if b&0xcf == 0xc5 {
		return stackPush(), nil
	}
	if b&0xcf == 0xc1 {
		return stackPop(), nil
	}
	if b == 0xcd {
		return stackPush(), nil
	}
	if b == 0xc9 {
		return stackPop(), nil
	}
	if b&0xc7 == 0xc4 {
		if condition((b >> 3) & 7) {
			return stackPush(), nil
		}
		return nil, nil
	}
	if b&0xc7 == 0xc0 {
		if condition((b >> 3) & 7) {
			return stackPop(), nil
		}
		return nil, nil
	}
	switch b {
	case 0x0a:
		return bytes(m.cpu.BC(), 1, false), nil
	case 0x1a:
		return bytes(m.cpu.DE(), 1, false), nil
	case 0x02:
		return bytes(m.cpu.BC(), 1, true), nil
	case 0x12:
		return bytes(m.cpu.DE(), 1, true), nil
	case 0x3a:
		return bytes(word(1), 1, false), nil
	case 0x32:
		return bytes(word(1), 1, true), nil
	case 0x2a:
		return bytes(word(1), 2, false), nil
	case 0x22:
		return bytes(word(1), 2, true), nil
	case 0x36:
		return bytes(m.cpu.HL(), 1, true), nil
	}
	if b >= 0x40 && b < 0x80 && b != 0x76 {
		if b&7 == 6 {
			return bytes(m.cpu.HL(), 1, false), nil
		}
		if (b>>3)&7 == 6 {
			return bytes(m.cpu.HL(), 1, true), nil
		}
		return nil, nil
	}
	if b == 0xed {
		op := next(1)
		if op&0xcf == 0x4b {
			return bytes(word(2), 2, false), nil
		}
		if op&0xcf == 0x43 {
			return bytes(word(2), 2, true), nil
		}
	}
	if b == 0xdd || b == 0xfd {
		base := m.cpu.IX()
		if b == 0xfd {
			base = m.cpu.IY()
		}
		op := next(1)
		if op == 0xe5 {
			return stackPush(), nil
		}
		if op == 0xe1 {
			return stackPop(), nil
		}
		if op == 0x2a {
			return bytes(word(2), 2, false), nil
		}
		if op == 0x22 {
			return bytes(word(2), 2, true), nil
		}
		addr := uint16(int(base) + int(int8(next(2))))
		if op == 0x36 {
			return bytes(addr, 1, true), nil
		}
		if op >= 0x40 && op < 0x80 && op != 0x76 {
			if op&7 == 6 {
				return bytes(addr, 1, false), nil
			}
			if (op>>3)&7 == 6 {
				return bytes(addr, 1, true), nil
			}
			return nil, nil
		}
	}
	return nil, fmt.Errorf("unsupported protected-memory opcode %02x at %04x", b, pc)
}

func memoryHostFrameOpcode(m *memoryHostZ80Memory, pc uint16) bool {
	b := m.data[pc]
	if b == 0xdd || b == 0xfd {
		b = m.data[pc+1]
		return b == 0xe5 || b == 0xe1
	}
	return b&0xcf == 0xc5 || b&0xcf == 0xc1 || b == 0xcd || b == 0xc9 || b&0xc7 == 0xc4 || b&0xc7 == 0xc0
}
