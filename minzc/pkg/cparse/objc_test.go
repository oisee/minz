package cparse

import (
	"testing"
)

func TestObjCTokens_ScanAtKeywords(t *testing.T) {
	src := `@interface Foo
@end
`
	cfg := &Config{ABI: defaultABI()}
	sources := []Source{
		{Name: "<predefined>", Value: predefined},
		{Name: "test.m", Value: src},
	}

	ast, err := Translate(cfg, sources)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Walk AST to find ObjC interface.
	found := false
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed != nil && ed.Case == ExternalDeclarationObjCInterface {
			found = true
			iface := ed.ObjCInterface
			if iface == nil {
				t.Fatal("ObjCInterface is nil")
			}
			name := iface.ClassName.SrcStr()
			if name != "Foo" {
				t.Errorf("class name = %q, want Foo", name)
			}
			t.Logf("@interface %s parsed OK", name)
		}
	}
	if !found {
		t.Error("did not find ObjC @interface in AST")
	}
}

func TestObjC_InterfaceWithIvarsAndMethods(t *testing.T) {
	src := `
@interface Counter {
    int count;
}
-(int)value;
-(void)increment;
-(void)addAmount:(int)n;
@end
`
	cfg := &Config{ABI: defaultABI()}
	sources := []Source{
		{Name: "<predefined>", Value: predefined},
		{Name: "test.m", Value: src},
	}

	ast, err := Translate(cfg, sources)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Case != ExternalDeclarationObjCInterface {
			continue
		}
		iface := ed.ObjCInterface
		t.Logf("@interface %s", iface.ClassName.SrcStr())
		t.Logf("  %d ivars, %d methods", len(iface.Ivars), len(iface.Methods))

		if len(iface.Ivars) != 1 {
			t.Errorf("expected 1 ivar, got %d", len(iface.Ivars))
		}
		if len(iface.Methods) != 3 {
			t.Errorf("expected 3 methods, got %d", len(iface.Methods))
		}
		for _, m := range iface.Methods {
			t.Logf("  method: %s (kind=%d)", m.MethodName(), m.Kind)
		}
	}
}

func TestObjC_Implementation(t *testing.T) {
	src := `
@implementation Counter
-(int)value {
    return self->count;
}
-(void)increment {
    self->count = self->count + 1;
}
@end
`
	cfg := &Config{ABI: defaultABI()}
	sources := []Source{
		{Name: "<predefined>", Value: predefined},
		{Name: "test.m", Value: src},
	}

	ast, err := Translate(cfg, sources)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil || ed.Case != ExternalDeclarationObjCImplementation {
			continue
		}
		impl := ed.ObjCImplementation
		t.Logf("@implementation %s — %d methods", impl.ClassName.SrcStr(), len(impl.Methods))
		if len(impl.Methods) != 2 {
			t.Errorf("expected 2 methods, got %d", len(impl.Methods))
		}
		for _, m := range impl.Methods {
			t.Logf("  method: %s (has body: %v)", m.MethodName(), m.Body != nil)
			if m.Body == nil {
				t.Errorf("method %s has no body", m.MethodName())
			}
		}
	}
}

func TestObjC_MixedWithC(t *testing.T) {
	// ObjC and plain C in the same file.
	src := `
int add(int a, int b) { return a + b; }

@interface Calc
-(int)compute:(int)x;
@end

int mul(int a, int b) { return a * b; }
`
	cfg := &Config{ABI: defaultABI()}
	sources := []Source{
		{Name: "<predefined>", Value: predefined},
		{Name: "test.m", Value: src},
	}

	ast, err := Translate(cfg, sources)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	cFuncs := 0
	objcIfaces := 0
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil {
			continue
		}
		switch ed.Case {
		case ExternalDeclarationFuncDef:
			cFuncs++
		case ExternalDeclarationObjCInterface:
			objcIfaces++
		}
	}
	t.Logf("C functions: %d, ObjC interfaces: %d", cFuncs, objcIfaces)
	if cFuncs != 2 {
		t.Errorf("expected 2 C functions, got %d", cFuncs)
	}
	if objcIfaces != 1 {
		t.Errorf("expected 1 ObjC interface, got %d", objcIfaces)
	}
}

const predefined = "int __predefined_declarator;\ntypedef unsigned int __predefined_size_t;\ntypedef int __predefined_ptrdiff_t;\ntypedef int __predefined_wchar_t;\n"

func TestObjCTokens_PlainC_StillWorks(t *testing.T) {
	// Verify that adding ObjC tokens doesn't break plain C parsing.
	src := `int add(int a, int b) { return a + b; }`
	cfg := &Config{ABI: defaultABI()}
	sources := []Source{
		{Name: "<predefined>", Value: predefined},
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
