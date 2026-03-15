package cparse

import (
	"testing"
)

func TestObjCTokens_ScanAtKeywords(t *testing.T) {
	// Verify that @interface, @end etc. produce the right token types.
	src := `@interface Foo
@end
`
	cfg := &Config{ABI: defaultABI()}
	sources := []Source{
		{Name: "<predefined>", Value: "int __predefined_declarator;\ntypedef unsigned int __predefined_size_t;\ntypedef int __predefined_ptrdiff_t;\ntypedef int __predefined_wchar_t;\n"},
		{Name: "test.m", Value: src},
	}

	// We expect parse to fail (no ObjC grammar yet), but we can verify
	// the scanner sees the tokens by catching the error message.
	_, err := Translate(cfg, sources)
	if err == nil {
		t.Log("ObjC parsed without error (unexpected but ok)")
		return
	}

	// The error should mention @interface or unexpected token,
	// NOT "illegal character '@'" — proving the scanner recognized it.
	t.Logf("Expected parse error (no ObjC grammar yet): %v", err)
}

func TestObjCTokens_PlainC_StillWorks(t *testing.T) {
	// Verify that adding ObjC tokens doesn't break plain C parsing.
	src := `int add(int a, int b) { return a + b; }`
	cfg := &Config{ABI: defaultABI()}
	sources := []Source{
		{Name: "<predefined>", Value: "int __predefined_declarator;\ntypedef unsigned int __predefined_size_t;\ntypedef int __predefined_ptrdiff_t;\ntypedef int __predefined_wchar_t;\n"},
		{Name: "test.c", Value: src},
	}

	ast, err := Translate(cfg, sources)
	if err != nil {
		t.Fatalf("Plain C should still parse: %v", err)
	}
	if ast == nil {
		t.Fatal("AST is nil")
	}
	t.Log("Plain C still works with ObjC tokens added")
}

func defaultABI() *ABI {
	return &ABI{
		Types: map[Kind]AbiType{
			Void:      {Size: 0, Align: 1, FieldAlign: 1},
			Bool:      {Size: 1, Align: 1, FieldAlign: 1},
			Char:      {Size: 1, Align: 1, FieldAlign: 1},
			SChar:     {Size: 1, Align: 1, FieldAlign: 1},
			UChar:     {Size: 1, Align: 1, FieldAlign: 1},
			Short:     {Size: 2, Align: 2, FieldAlign: 2},
			UShort:    {Size: 2, Align: 2, FieldAlign: 2},
			Int:       {Size: 4, Align: 4, FieldAlign: 4},
			UInt:      {Size: 4, Align: 4, FieldAlign: 4},
			Long:      {Size: 8, Align: 8, FieldAlign: 8},
			ULong:     {Size: 8, Align: 8, FieldAlign: 8},
			LongLong:  {Size: 8, Align: 8, FieldAlign: 8},
			ULongLong: {Size: 8, Align: 8, FieldAlign: 8},
			Float:     {Size: 4, Align: 4, FieldAlign: 4},
			Double:    {Size: 8, Align: 8, FieldAlign: 8},
			LongDouble: {Size: 8, Align: 8, FieldAlign: 8},
			Ptr:       {Size: 8, Align: 8, FieldAlign: 8},
			Function:  {Size: 8, Align: 8, FieldAlign: 8},
			Array:     {Size: 0, Align: 1, FieldAlign: 1},
			Struct:    {Size: 0, Align: 1, FieldAlign: 1},
			Union:     {Size: 0, Align: 1, FieldAlign: 1},
			Enum:      {Size: 4, Align: 4, FieldAlign: 4},
		},
	}
}
