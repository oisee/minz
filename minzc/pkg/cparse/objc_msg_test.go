package cparse

import "testing"

func TestObjC_MessageUnary(t *testing.T) {
	// [counter value]
	src := `
@interface Counter
-(int)value;
@end

int test(void) {
    int x = [self value];
    return x;
}
`
	ast, err := objcParse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	funcs, ifaces := countObjC(ast)
	t.Logf("funcs=%d, ifaces=%d", funcs, ifaces)
	if funcs != 1 {
		t.Errorf("expected 1 func, got %d", funcs)
	}
	if ifaces != 1 {
		t.Errorf("expected 1 iface, got %d", ifaces)
	}
}

func TestObjC_MessageKeyword(t *testing.T) {
	// [obj add:5]
	src := `
int test(int x) {
    int r = [self add:x];
    return r;
}
`
	_, err := objcParse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	t.Log("[self add:x] parsed OK")
}

func TestObjC_MessageMultiKeyword(t *testing.T) {
	// [obj addX:1 andY:2]
	src := `
int test(void) {
    return [self addX:1 andY:2];
}
`
	_, err := objcParse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	t.Log("[self addX:1 andY:2] parsed OK")
}

func TestObjC_MessageNested(t *testing.T) {
	// [[Foo alloc] init]
	src := `
int test(void) {
    int x = [[self alloc] init];
    return x;
}
`
	_, err := objcParse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	t.Log("[[self alloc] init] parsed OK")
}

func TestObjC_MessageInExpression(t *testing.T) {
	// message result used in expression
	src := `
int test(int a) {
    return [self value] + a * 2;
}
`
	_, err := objcParse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	t.Log("[self value] + a * 2 parsed OK")
}

func TestObjC_FullClass(t *testing.T) {
	// Full ObjC class: interface + implementation + usage
	src := `
@interface Point {
    int x;
    int y;
}
-(int)getX;
-(int)getY;
-(void)setX:(int)newX;
-(int)manhattan;
@end

@implementation Point
-(int)getX {
    return self->x;
}
-(int)getY {
    return self->y;
}
-(void)setX:(int)newX {
    self->x = newX;
}
-(int)manhattan {
    int ax = self->x;
    int ay = self->y;
    if (ax < 0) ax = -ax;
    if (ay < 0) ay = -ay;
    return ax + ay;
}
@end

int test(void) {
    return 42;
}
`
	ast, err := objcParse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	funcs, ifaces := countObjC(ast)
	impls := 0
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed != nil && ed.Case == ExternalDeclarationObjCImplementation {
			impls++
			impl := ed.ObjCImplementation
			t.Logf("@implementation %s — %d methods", impl.ClassName.SrcStr(), len(impl.Methods))
			for _, m := range impl.Methods {
				t.Logf("  %s (body: %v)", m.MethodName(), m.Body != nil)
			}
		}
	}
	t.Logf("funcs=%d, ifaces=%d, impls=%d", funcs, ifaces, impls)
	if ifaces != 1 || impls != 1 || funcs != 1 {
		t.Errorf("expected 1/1/1, got %d/%d/%d", ifaces, impls, funcs)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────

func objcParse(src string) (*AST, error) {
	cfg := &Config{ABI: defaultABI()}
	sources := []Source{
		{Name: "<predefined>", Value: predefined},
		{Name: "test.m", Value: src},
	}
	return Translate(cfg, sources)
}

func countObjC(ast *AST) (funcs, ifaces int) {
	for tu := ast.TranslationUnit; tu != nil; tu = tu.TranslationUnit {
		ed := tu.ExternalDeclaration
		if ed == nil {
			continue
		}
		switch ed.Case {
		case ExternalDeclarationFuncDef:
			funcs++
		case ExternalDeclarationObjCInterface:
			ifaces++
		}
	}
	return
}
