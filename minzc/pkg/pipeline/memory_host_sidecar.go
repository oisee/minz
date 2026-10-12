package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
)

type MemoryHostResolvedAccess struct {
	PC         uint16                  `json:"pc"`
	Descriptor mir2.NativeMemoryAccess `json:"descriptor"`
	Address    uint16                  `json:"address"`
	Opcode     []byte                  `json:"opcode"`
}

// resolveMemoryHostSidecar binds a generated operation descriptor to one final
// instruction. Actual effective addresses are checked again on every execution.
func resolveMemoryHostSidecar(a *z80asm.Result, records []mir2.NativeMemoryAccess) (map[uint16][]memoryHostByteRecipe, []MemoryHostResolvedAccess, error) {
	// Symbol offsets come only from internally generated roots, not comments.
	result := map[uint16][]memoryHostByteRecipe{}
	resolved := []MemoryHostResolvedAccess{}
	seen := map[string]bool{}
	for _, record := range records {
		if seen[record.ID] || record.StorageKind != "global" && record.StorageKind != "spill" || record.Write != record.Root.Write || record.ByteOffset < 0 || record.Bytes < 1 || record.Bytes > 2 || record.ByteOffset+record.Bytes > record.Root.Width {
			return nil, nil, fmt.Errorf("invalid sidecar descriptor")
		}
		seen[record.ID] = true
		var line *z80asm.ListingLine
		marker := "@minz-memory-access:" + record.ID
		for i := range a.Listing {
			l := &a.Listing[i]
			comment := strings.SplitN(l.SourceLine, ";", 2)
			if len(comment) == 2 && mir2.NativeAnnotationCount(l.SourceLine, marker) == 1 {
				if line != nil {
					return nil, nil, fmt.Errorf("duplicate access marker")
				}
				line = l
			} else if strings.Contains(l.SourceLine, marker) {
				return nil, nil, fmt.Errorf("malformed access marker")
			}
		}
		if line == nil || line.Address < 0 || line.Address+len(line.Bytes) > 65536 || len(line.Bytes) == 0 {
			return nil, nil, fmt.Errorf("missing access instruction")
		}
		base, ok := workloadSymbol(a.Symbols, record.Root.Symbol)
		if !ok {
			return nil, nil, fmt.Errorf("sidecar global symbol missing")
		}
		address := base + record.Root.Offset + record.ByteOffset
		if address < 0 || address+record.Bytes > 65536 {
			return nil, nil, fmt.Errorf("sidecar address overflow")
		}
		if result[uint16(line.Address)] != nil {
			return nil, nil, fmt.Errorf("duplicate sidecar PC")
		}
		if e := validateMemoryHostOpcode(line.Bytes, record.Bytes, record.Write, address); e != nil {
			return nil, nil, e
		}
		recipe := []memoryHostByteRecipe{}
		for i := 0; i < record.Bytes; i++ {
			recipe = append(recipe, memoryHostByteRecipe{uint16(address + i), record.Write})
		}
		result[uint16(line.Address)] = recipe
		resolved = append(resolved, MemoryHostResolvedAccess{uint16(line.Address), record, uint16(address), append([]byte(nil), line.Bytes...)})
	}
	return result, resolved, nil
}
func validateMemoryHostOpcode(code []byte, count int, write bool, address int) error {
	b := code[0]
	width := 0
	direction := false
	direct := -1
	word := func(n int) int {
		if len(code) < n+2 {
			return -1
		}
		return int(code[n]) | int(code[n+1])<<8
	}
	switch b {
	case 0x0a, 0x1a:
		width = 1
	case 0x02, 0x12:
		width = 1
		direction = true
	case 0x3a:
		width = 1
		direct = word(1)
	case 0x32:
		width = 1
		direction = true
		direct = word(1)
	case 0x2a:
		width = 2
		direct = word(1)
	case 0x22:
		width = 2
		direction = true
		direct = word(1)
	case 0x36:
		width = 1
		direction = true
	}
	if b >= 0x40 && b < 0x80 && b != 0x76 {
		if b&7 == 6 {
			width = 1
		} else if (b>>3)&7 == 6 {
			width = 1
			direction = true
		}
	}
	if b == 0xed && len(code) >= 4 {
		op := code[1]
		if op&0xcf == 0x4b {
			width = 2
			direct = word(2)
		} else if op&0xcf == 0x43 {
			width = 2
			direction = true
			direct = word(2)
		}
	}
	if (b == 0xdd || b == 0xfd) && len(code) >= 3 {
		op := code[1]
		if op == 0x2a {
			width = 2
			direct = word(2)
		} else if op == 0x22 {
			width = 2
			direction = true
			direct = word(2)
		} else if op == 0x36 {
			width = 1
			direction = true
		} else if op >= 0x40 && op < 0x80 && op != 0x76 {
			if op&7 == 6 {
				width = 1
			} else if (op>>3)&7 == 6 {
				width = 1
				direction = true
			}
		}
	}
	length := 1
	if b == 0x3a || b == 0x32 || b == 0x2a || b == 0x22 {
		length = 3
	} else if b == 0x36 {
		length = 2
	} else if b == 0xed {
		length = 4
	} else if b == 0xdd || b == 0xfd {
		length = 3
		if len(code) >= 2 && (code[1] == 0x36 || code[1] == 0x2a || code[1] == 0x22) {
			length = 4
		}
	}
	if len(code) != length || width != count || direction != write || direct >= 0 && direct != address {
		return fmt.Errorf("native access opcode does not match descriptor")
	}
	return nil
}
