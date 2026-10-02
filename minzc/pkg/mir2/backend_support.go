package mir2

import "fmt"

// RejectUnsupportedOps lets backends reject operations before emitting partial
// output, rather than silently leaving their destination registers undefined.
func RejectUnsupportedOps(m *Module, backend string, unsupported ...Op) error {
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				for _, op := range unsupported {
					if i.Op == op {
						return fmt.Errorf("%s: unsupported %s in %s/%s", backend, op, f.Name, b.Label)
					}
				}
			}
		}
	}
	return nil
}
