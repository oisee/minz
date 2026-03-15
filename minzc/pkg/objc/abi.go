package objc

import (
	"encoding/binary"

	cc "github.com/minz/minzc/pkg/cparse"
)

// z80Predefined provides type declarations for the C type checker.
const z80Predefined = `
int __predefined_declarator;
typedef unsigned int __predefined_size_t;
typedef int __predefined_ptrdiff_t;
typedef int __predefined_wchar_t;
typedef unsigned char uint8_t;
typedef signed char int8_t;
typedef unsigned int uint16_t;
typedef signed int int16_t;
typedef int BOOL;
`

func z80ABI() *cc.ABI {
	mkTy := func(size int64) cc.AbiType {
		return cc.AbiType{Size: size, Align: 1, FieldAlign: 1}
	}
	return &cc.ABI{
		ByteOrder:  binary.LittleEndian,
		SignedChar: true,
		Types: map[cc.Kind]cc.AbiType{
			cc.Void:       mkTy(0),
			cc.Bool:       mkTy(1),
			cc.Char:       mkTy(1),
			cc.SChar:      mkTy(1),
			cc.UChar:      mkTy(1),
			cc.Short:      mkTy(2),
			cc.UShort:     mkTy(2),
			cc.Int:        mkTy(2),
			cc.UInt:       mkTy(2),
			cc.Long:       mkTy(4),
			cc.ULong:      mkTy(4),
			cc.LongLong:   mkTy(4),
			cc.ULongLong:  mkTy(4),
			cc.Float:      mkTy(4),
			cc.Double:     mkTy(4),
			cc.LongDouble: mkTy(4),
			cc.Ptr:        mkTy(2),
			cc.Function:   mkTy(2),
			cc.Array:      mkTy(0),
			cc.Struct:     mkTy(0),
			cc.Union:      mkTy(0),
			cc.Enum:       mkTy(2),
		},
	}
}
