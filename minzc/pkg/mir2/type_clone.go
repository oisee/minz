package mir2

// CloneSemanticType owns mutable composite ABI descriptors while preserving
// immutable canonical primitive types. The memo handles recursive type graphs.
func CloneSemanticType(ty Ty) Ty { return cloneSemanticType(ty, map[Ty]Ty{}) }
func cloneSemanticType(ty Ty, memo map[Ty]Ty) Ty {
	if ty == nil {
		return nil
	}
	if found, ok := memo[ty]; ok {
		return found
	}
	switch t := ty.(type) {
	case *StructTy:
		n := *t
		n.Fields = make([]StructField, len(t.Fields))
		n.FieldOffsets = append([]int(nil), t.FieldOffsets...)
		memo[ty] = &n
		for i, f := range t.Fields {
			n.Fields[i] = StructField{Name: f.Name, Ty: cloneSemanticType(f.Ty, memo)}
		}
		return &n
	case *ArrayTy:
		n := *t
		memo[ty] = &n
		n.Elem = cloneSemanticType(t.Elem, memo)
		return &n
	case *TupleTy:
		n := *t
		memo[ty] = &n
		n.Elems = make([]Ty, len(t.Elems))
		n.Classes = append([]RegClass(nil), t.Classes...)
		for i, e := range t.Elems {
			n.Elems[i] = cloneSemanticType(e, memo)
		}
		return &n
	case *SliceTy:
		n := *t
		memo[ty] = &n
		n.Elem = cloneSemanticType(t.Elem, memo)
		return &n
	case *RangedTy:
		n := *t
		memo[ty] = &n
		n.Base = cloneSemanticType(t.Base, memo)
		return &n
	case *ptrTy:
		if t.elem == nil {
			return ty
		}
		n := *t
		memo[ty] = &n
		n.elem = cloneSemanticType(t.elem, memo)
		return &n
	case *TyVarTy:
		n := *t
		memo[ty] = &n
		return &n
	default:
		return ty
	}
}

// CloneCallABIMetadata returns owned descriptor trees for an instruction copy.
func CloneCallABIMetadata(dst, src *Inst) {
	memo := map[Ty]Ty{}
	dst.IndirectParamTys = cloneABITypes(src.IndirectParamTys, memo)
	dst.IndirectReturnTys = cloneABITypes(src.IndirectReturnTys, memo)
	dst.StructReturnBuffers = CloneStructReturnBuffers(src.StructReturnBuffers, memo)
}
func cloneABITypes(src []Ty, memo map[Ty]Ty) []Ty {
	if src == nil {
		return nil
	}
	out := make([]Ty, len(src))
	for i, t := range src {
		out[i] = cloneSemanticType(t, memo)
	}
	return out
}
func CloneStructReturnBuffers(src []StructReturnBuffer, memo map[Ty]Ty) []StructReturnBuffer {
	if src == nil {
		return nil
	}
	out := append([]StructReturnBuffer(nil), src...)
	for i, s := range src {
		if s.Ty != nil {
			out[i].Ty = cloneSemanticType(s.Ty, memo).(*StructTy)
		}
	}
	return out
}
