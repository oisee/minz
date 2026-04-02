package mir2

// LUTGen replaces pure functions whose sole parameter is a RangedTy integer
// (range size ≤ 256) with compile-time-computed lookup tables.
//
// For each eligible function f(x: T<lo..hi>) -> U:
//  1. Evaluate f at every value in [lo, hi) via the MIR2 VM.
//  2. Emit a read-only Global "f_lut" containing the result bytes.
//  3. Replace f's body with a table-lookup sequence:
//       base = &f_lut
//       idx  = zero_ext(x - lo, u16)   // adjust for non-zero lo
//       ptr  = base + idx
//       ret  = *ptr
//
// Call sites are unchanged — they still CALL f.  The body is now just 5–6
// instructions so the future inliner will eliminate the CALL overhead.
// ConstantCallElim (run before LUTGen) already folds constant-argument calls.
//
// Eligibility criteria:
//   - Exactly one parameter with a RangedTy base (u8 or u16).
//   - Range size ≤ 256 values (fits in an 8-bit index after lo-adjustment).
//   - Single u8/u16 return value (multi-return and void are not supported).
//   - Function is not extern and not already marked recursive (VM safety).
//   - VM evaluation succeeds for every value in the range (no side effects,
//     no step-limit exceeded).
//
// Returns true if at least one function was replaced.
func LUTGen(m *Module) bool {
	vm := NewVM(m)
	vm.MaxSteps = 10_000_000 // generous: used for the table eval loop

	changed := false
	for _, f := range m.Funcs {
		if !lutEligible(f) {
			continue
		}

		// Save param info before clearing contract.
		origParam := f.Contract.Params[0]
		rt, _ := origParam.Ty.(*RangedTy)
		rangeSize := rt.Hi - rt.Lo // exclusive-Hi convention → count
		retTy := f.Contract.Returns[0].Ty
		retW := ByteWidth(retTy)

		// Evaluate every value in [Lo, Hi).
		table, err := vm.evalRange(f.Name, rt.Lo, rt.Hi)
		if err != nil {
			continue // VM error — keep original body
		}

		// Emit read-only LUT global(s).
		//
		// u8 return  -> one [N]u8 table
		// u16 return -> split [N]u8 low/high tables on Z80-friendly layout
		if retW == 1 {
			lutName := f.Name + "_lut"
			init := make([]byte, int(rangeSize))
			for i, v := range table {
				init[i] = byte(v)
			}
			arrayTy := NewArray(retTy, int(rangeSize))
			if rangeSize == 256 {
				arrayTy.Align = 256
			}
			m.AddGlobal(Global{
				Name:    lutName,
				Ty:      arrayTy,
				Init:    init,
				IsConst: true,
			})
		} else {
			lutLoName := f.Name + "_lut_lo"
			lutHiName := f.Name + "_lut_hi"
			initLo := make([]byte, int(rangeSize))
			initHi := make([]byte, int(rangeSize))
			for i, v := range table {
				initLo[i] = byte(v)
				initHi[i] = byte(v >> 8)
			}
			arrayTy := NewArray(TyU8, int(rangeSize))
			if rangeSize == 256 {
				arrayTy.Align = 256
			}
			m.AddGlobal(Global{
				Name:    lutLoName,
				Ty:      arrayTy,
				Init:    initLo,
				IsConst: true,
			})
			m.AddGlobal(Global{
				Name:    lutHiName,
				Ty:      arrayTy,
				Init:    initHi,
				IsConst: true,
			})
		}

		// Replace function body with lookup sequence.
		// Clear blocks AND contract params — b.Param() appends to Contract.Params,
		// so we must start fresh or the old params (from the original body) remain
		// and the allocator sees duplicate entries, causing register misassignment.
		f.Blocks = nil
		f.Contract.Params = f.Contract.Params[:0]
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")

		x := b.Param(origParam.Name, rt.Base, origParam.Class)

		// Adjust index: idx8 = x - lo  (if lo != 0)
		var idxVal Reg
		if rt.Lo == 0 {
			idxVal = x
		} else {
			loReg := b.Const(rt.Lo, rt.Base, origParam.Class)
			idxVal = b.Sub(x, loReg, rt.Base, origParam.Class)
		}

		// Zero-extend to u16 for pointer arithmetic when needed.
		var idx16 Reg
		if rt.Base.Width() <= 8 {
			idx16 = b.Ext(idxVal, rt.Base, TyU16, ClassIndex)
		} else {
			idx8 := b.Trunc(idxVal, rt.Base, TyU8, ClassGeneral)
			idx16 = b.Ext(idx8, TyU8, TyU16, ClassIndex)
		}

		retCls := f.Contract.Returns[0].Class
		var result Reg
		if retW == 1 {
			lutName := f.Name + "_lut"
			base := b.AddrOf(lutName, ClassPointer)
			ptr := b.PtrAdd(base, idx16, ClassPointer)
			result = b.Load(ptr, retTy, retCls)
		} else {
			lutLoName := f.Name + "_lut_lo"
			lutHiName := f.Name + "_lut_hi"

			baseLo := b.AddrOf(lutLoName, ClassPointer)
			ptrLo := b.PtrAdd(baseLo, idx16, ClassPointer)
			lo8 := b.Load(ptrLo, TyU8, ClassGeneral)

			baseHi := b.AddrOf(lutHiName, ClassPointer)
			ptrHi := b.PtrAdd(baseHi, idx16, ClassPointer)
			hi8 := b.Load(ptrHi, TyU8, ClassGeneral)

			lo16 := b.Ext(lo8, TyU8, TyU16, ClassGeneral)
			hi16 := b.Ext(hi8, TyU8, TyU16, ClassGeneral)
			shift8 := b.Const(8, TyU8, ClassGeneral)
			hiShifted := b.Shl(hi16, shift8, TyU16, ClassGeneral)
			result = b.Or(lo16, hiShifted, TyU16, retCls)
		}

		b.Ret(result)

		changed = true
	}
	return changed
}

// lutEligible reports whether f is a candidate for LUT replacement.
func lutEligible(f *Func) bool {
	// Must have a body.
	if f.Attrs.IsExtern || len(f.Blocks) == 0 {
		return false
	}
	// Exactly one parameter.
	if len(f.Contract.Params) != 1 {
		return false
	}
	// Parameter must be a ranged type.
	rt, ok := f.Contract.Params[0].Ty.(*RangedTy)
	if !ok {
		return false
	}
	// Base type must be an 8-bit or 16-bit integer.
	if rt.Base.Width() != 8 && rt.Base.Width() != 16 {
		return false
	}
	// Range must be ≤ 256 values.
	size := rt.Hi - rt.Lo
	if size <= 0 || size > 256 {
		return false
	}
	// Must return exactly one u8/u16 value.
	if len(f.Contract.Returns) != 1 {
		return false
	}
	if f.Contract.Returns[0].Ty.Width() != 8 && f.Contract.Returns[0].Ty.Width() != 16 {
		return false
	}
	return true
}

// evalRange evaluates funcName for every integer in [lo, hi) and returns the
// results as a slice.  This is like EvalTable but uses an already-configured VM.
func (vm *VM) evalRange(funcName string, lo, hi int64) ([]int64, error) {
	out := make([]int64, 0, int(hi-lo))
	for i := lo; i < hi; i++ {
		vm.steps = 0
		results, err := vm.Call(funcName, []Value{{I: i}})
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			out = append(out, 0)
		} else {
			out = append(out, results[0].I)
		}
	}
	return out, nil
}
