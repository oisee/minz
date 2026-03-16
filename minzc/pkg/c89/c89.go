// Package c89 implements a C89/C99 frontend for the MinZ compiler.
//
// Architecture:
//
//	C source → cparse.Translate() → cc.AST → lower.go → *hir.Module
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
	"regexp"
	"strconv"
	"strings"

	cc "github.com/minz/minzc/pkg/cparse"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/interop"
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

// CompileOpts configures the C89 compiler.
type CompileOpts struct {
	// BaseDir is the source file directory for resolving #import paths.
	BaseDir string
	// Resolver handles cross-language #import directives.
	// If nil, #import directives are ignored.
	Resolver *interop.Resolver
}

// Compile parses C source code and produces an HIR module.
func Compile(src, name string) (*hir.Module, error) {
	return CompileWithOpts(src, name, CompileOpts{})
}

// CompileWithOpts parses C source code with options (including #import support).
func CompileWithOpts(src, name string, opts CompileOpts) (*hir.Module, error) {
	// Process #import directives before cc/v4 sees the source.
	var importedModules []*hir.Module
	if opts.Resolver != nil {
		cleaned, mods, err := opts.Resolver.Resolve(src)
		if err != nil {
			return nil, fmt.Errorf("c89 #import: %w", err)
		}
		src = cleaned
		importedModules = mods
	}

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

	modName := strings.TrimSuffix(name, ".c")
	modName = strings.TrimSuffix(modName, ".m")
	l := &lowerer{
		ast:         ast,
		hm:          &hir.Module{Name: modName},
		globals:     make(map[string]mir2.Ty),
		typedefs:    make(map[string]mir2.Ty),
		structs:     make(map[string]*mir2.StructTy),
		objcClasses:   make(map[string]*objcClassInfo),
		objcProtocols: make(map[string]*objcProtocolInfo),
	}

	if err := l.lower(); err != nil {
		return nil, fmt.Errorf("c89 lower: %w", err)
	}

	// Struct-return promotion (ADR-0025): small struct returns → tuple returns.
	// Must run before merge so promoted signatures are visible to importers.
	PromoteStructReturns(l.hm)

	// Merge imported modules into this module.
	if len(importedModules) > 0 {
		interop.MergeInto(l.hm, importedModules)
	}

	// Parse comment-based asserts and sandboxes.
	l.hm.Asserts, l.hm.Sandboxes = parseCommentDirectives(src)

	// Parse and generate ObjC assert wrappers.
	l.generateObjCAssertWrappers(src)

	return l.hm, nil
}

// assertRe matches: // assert fn(1, 2, 0xAB) == 42 [via mir2|z80]
var assertRe = regexp.MustCompile(
	`//\s*assert\s+(\w+)\s*\(([^)]*)\)\s*==\s*(-?(?:0[xX][0-9a-fA-F]+|\d+))(?:\s+via\s+(mir2|z80))?\s*$`,
)

// objcAssertRe matches: // assert-objc Counter{count:42}.value(5) == 42
// Groups: 1=ClassName, 2=field_inits (may be empty), 3=methodName, 4=args (may be empty), 5=expected, 6=via
var objcAssertRe = regexp.MustCompile(
	`//\s*assert-objc\s+(\w+)\{([^}]*)}\s*\.\s*(\w+)\s*\(([^)]*)\)\s*==\s*(-?(?:0[xX][0-9a-fA-F]+|\d+))(?:\s+via\s+(mir2|z80))?\s*$`,
)

// sandboxStartRe matches: // sandbox "name"
var sandboxStartRe = regexp.MustCompile(`//\s*sandbox\s+"([^"]+)"`)

// sandboxEndRe matches: // end sandbox
var sandboxEndRe = regexp.MustCompile(`//\s*end\s+sandbox`)

func parseAssertLine(line string, lineNo int) (hir.Assert, bool) {
	m := assertRe.FindStringSubmatch(line)
	if m == nil {
		return hir.Assert{}, false
	}
	funcName := m[1]
	argsStr := strings.TrimSpace(m[2])
	expectedStr := m[3]
	via := m[4]

	var args []int64
	if argsStr != "" {
		for _, s := range strings.Split(argsStr, ",") {
			s = strings.TrimSpace(s)
			v, err := strconv.ParseInt(s, 0, 64) // base 0: auto-detect 0x, 0b, octal
			if err != nil {
				continue
			}
			args = append(args, v)
		}
	}

	expected, _ := strconv.ParseInt(expectedStr, 0, 64) // base 0: auto-detect hex
	return hir.Assert{
		FuncName: funcName,
		Args:     args,
		Expected: expected,
		Source:   line,
		Line:     lineNo,
		Via:      via,
	}, true
}

// parseCommentDirectives scans C source for assert and sandbox directives in comments.
func parseCommentDirectives(src string) ([]hir.Assert, []hir.Sandbox) {
	var asserts []hir.Assert
	var sandboxes []hir.Sandbox

	var curSandbox *hir.Sandbox

	for i, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		lineNo := i + 1

		// Check sandbox start.
		if m := sandboxStartRe.FindStringSubmatch(line); m != nil {
			curSandbox = &hir.Sandbox{Name: m[1], Line: lineNo}
			continue
		}

		// Check sandbox end.
		if sandboxEndRe.MatchString(line) && curSandbox != nil {
			sandboxes = append(sandboxes, *curSandbox)
			curSandbox = nil
			continue
		}

		// Check assert.
		if a, ok := parseAssertLine(line, lineNo); ok {
			if curSandbox != nil {
				curSandbox.Asserts = append(curSandbox.Asserts, a)
			} else {
				asserts = append(asserts, a)
			}
		}
	}

	// Auto-close unclosed sandbox.
	if curSandbox != nil && len(curSandbox.Asserts) > 0 {
		sandboxes = append(sandboxes, *curSandbox)
	}

	return asserts, sandboxes
}
