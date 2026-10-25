package nanz_test

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalImportedStructPointerNative(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "record.nanz"), []byte(`
struct Record { pad:u8, value:u16 }
fun Record.bump(self:^Record) -> u16 {
    self.value = self.value + 257
    return self.value
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := nanz.ParseWithOpts(`import record
fun probe(value:u16) -> u16 {
    var record:Record
    record.pad = 165
    record.value = value
    var p:^Record = &record
    let q:^Record = p
    let first:u16 = p.bump()
    let second:u16 = q.bump()
    if second != first + 257 { return 1 }
    if p.pad != 165 { return 2 }
    return q.value
}
`, "local-pointee.nanz", nanz.ParseOpts{BaseDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []int64{0, 255, 256, 65535} {
		m.Asserts = append(m.Asserts, hir.Assert{FuncName: "probe", Args: []int64{x}, Expected: int64(uint16(x + 514)), Source: "local var/let imported ^Struct field and UFCS"})
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
