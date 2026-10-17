package mir2

import "fmt"

// VerifyGlobalInitializers enforces the type layout shared by native targets
// and the VM. Symbolic slots also have to fit their declared storage.
func VerifyGlobalInitializers(m *Module) error {
	for _, g := range m.Globals {
		producer := g.Producer
		if producer == "" {
			producer = fmt.Sprintf("module %q", m.Name)
		}
		if g.Ty == nil {
			return fmt.Errorf("mir2: global %q has no type (producer: %s)", g.Name, producer)
		}
		width := ByteWidth(g.Ty)
		if len(g.Init) > width {
			return fmt.Errorf("mir2: global %q: initializer is %d bytes, type width is %d bytes (producer: %s)", g.Name, len(g.Init), width, producer)
		}
		if len(g.VtableSyms)*2 > width {
			return fmt.Errorf("mir2: global %q: vtable needs %d bytes, type width is %d bytes (producer: %s)", g.Name, len(g.VtableSyms)*2, width, producer)
		}
	}
	return nil
}
