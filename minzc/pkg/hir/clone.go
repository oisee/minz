package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// CloneModule owns all mutable HIR nodes, slices and maps, including source
// maps keyed by statement identity. Types are immutable and retain identity.
// Pointer memoisation preserves sharing (and the statement keys' association
// with their cloned bodies). It also handles recursive metadata graphs.
func CloneModule(m *Module) *Module {
	seen := map[reflect.Value]reflect.Value{}
	ty := reflect.TypeOf((*mir2.Ty)(nil)).Elem()
	var clone func(reflect.Value) reflect.Value
	clone = func(v reflect.Value) reflect.Value {
		if !v.IsValid() {
			return v
		}
		if v.Type().Implements(ty) {
			return v
		}
		switch v.Kind() {
		case reflect.Interface:
			if v.IsNil() {
				return reflect.Zero(v.Type())
			}
			out := reflect.New(v.Type()).Elem()
			out.Set(clone(v.Elem()))
			return out
		case reflect.Pointer:
			if v.IsNil() {
				return reflect.Zero(v.Type())
			}
			if out, ok := seen[v]; ok {
				return out
			}
			out := reflect.New(v.Type().Elem())
			seen[v] = out
			out.Elem().Set(clone(v.Elem()))
			return out
		case reflect.Slice:
			if v.IsNil() {
				return reflect.Zero(v.Type())
			}
			out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
			for i := 0; i < v.Len(); i++ {
				out.Index(i).Set(clone(v.Index(i)))
			}
			return out
		case reflect.Map:
			if v.IsNil() {
				return reflect.Zero(v.Type())
			}
			out := reflect.MakeMapWithSize(v.Type(), v.Len())
			iter := v.MapRange()
			for iter.Next() {
				out.SetMapIndex(clone(iter.Key()), clone(iter.Value()))
			}
			return out
		case reflect.Struct:
			out := reflect.New(v.Type()).Elem()
			out.Set(v)
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					out.Field(i).Set(clone(v.Field(i)))
				}
			}
			return out
		case reflect.Array:
			out := reflect.New(v.Type()).Elem()
			for i := 0; i < v.Len(); i++ {
				out.Index(i).Set(clone(v.Index(i)))
			}
			return out
		}
		return v
	}
	return clone(reflect.ValueOf(m)).Interface().(*Module)
}
