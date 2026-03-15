// Package c89 implements a C89/C99 frontend for the MinZ compiler.
//
// Architecture:
//
//	C source → modernc.org/cc/v4.Translate() → cc.AST → lower.go → *hir.Module
//
// The heavy lifting (preprocessing, parsing, type checking, implicit conversions)
// is handled by modernc.org/cc. This package only walks the typed AST and emits
// HIR nodes.
//
// See docs/adr/0024-c89-frontend-strategy.md for the full design.
// See docs/adr/0025-struct-return-promotion.md for struct→tuple promotion.
// See docs/adr/0026-c-stdlib-print-variadics.md for @print and variadics.
package c89

import (
	"encoding/binary"
	"fmt"
	"strings"

	cc "modernc.org/cc/v4"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// z80 predefined macros and type declarations for the C type checker.
const z80Predefined = `
#define __CHAR_BIT__ 8
#define __SIZEOF_INT__ 2
#define __SIZEOF_LONG__ 4
#define __SIZEOF_POINTER__ 2
#define __SIZEOF_SHORT__ 2
#define __SIZE_TYPE__ unsigned int
#define __PTRDIFF_TYPE__ int
#define __WCHAR_TYPE__ int
#define __INT_MAX__ 32767
#define __LONG_MAX__ 2147483647
#define __SCHAR_MAX__ 127
#define __SHRT_MAX__ 32767
#define NULL ((void*)0)
#define __Z80__ 1
#define __MINZ__ 1
typedef unsigned char uint8_t;
typedef signed char int8_t;
typedef unsigned int uint16_t;
typedef signed int int16_t;
typedef unsigned long uint32_t;
typedef signed long int32_t;
typedef unsigned int size_t;
typedef int ptrdiff_t;
int __predefined_declarator;
typedef unsigned int __predefined_size_t;
typedef int __predefined_ptrdiff_t;
typedef int __predefined_wchar_t;
typedef void *__builtin_va_list;
`

// z80ABI returns the ABI configuration for Z80 (16-bit int, 16-bit pointers).
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

// Compile parses C source code and produces an HIR module.
func Compile(src, name string) (*hir.Module, error) {
	cfg := &cc.Config{
		ABI: z80ABI(),
	}

	sources := []cc.Source{
		{Name: "<predefined>", Value: z80Predefined},
		{Name: name, Value: src},
	}

	ast, err := cc.Translate(cfg, sources)
	if err != nil {
		return nil, fmt.Errorf("c89 parse: %w", err)
	}

	l := &lowerer{
		ast:      ast,
		hm:       &hir.Module{Name: strings.TrimSuffix(name, ".c")},
		globals:  make(map[string]mir2.Ty),
		typedefs: make(map[string]mir2.Ty),
		structs:  make(map[string]*mir2.StructTy),
	}

	if err := l.lower(); err != nil {
		return nil, fmt.Errorf("c89 lower: %w", err)
	}

	return l.hm, nil
}
