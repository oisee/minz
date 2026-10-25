package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
)

// These names identify the portable non-owning descriptor, not its backing data.
const (
	BorrowedViewData = "data"
	BorrowedViewLen  = "len"
)

// BorrowedViewType uses ordinary addressable HIR struct storage (four bytes on
// current 16-bit-pointer targets). It does not reinterpret MIR2 SliceTy's product
// ABI as a struct address. Frontends admit length conversions and lifetimes.
func BorrowedViewType(name string, elem mir2.Ty) (*mir2.StructTy, error) {
	if name == "" || nilHIR(elem) || mir2.ByteWidth(elem) <= 0 {
		return nil, fmt.Errorf("borrowed view needs a name and sized element type")
	}
	return &mir2.StructTy{Name: name, Fields: []mir2.StructField{
		{Name: BorrowedViewData, Ty: mir2.PointerTo(elem)},
		{Name: BorrowedViewLen, Ty: mir2.TyU16},
	}}, nil
}
