package main

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestSQLiteMemoryHandlesShareConnectionsButStayIsolated(t *testing.T) {
	vms := []*mir2.VM{mir2.NewVM(&mir2.Module{}), mir2.NewVM(&mir2.Module{})}
	for _, v := range vms {
		registerSQLiteHosts(v, false)
	}
	type database struct {
		vm     *mir2.VM
		handle mir2.Value
		value  int64
	}
	var databases []database
	call := func(v *mir2.VM, name string, args ...mir2.Value) mir2.Value {
		t.Helper()
		r, err := v.Hosts[name](args)
		if err != nil || len(r) != 1 {
			t.Fatalf("%s: %v %v", name, r, err)
		}
		return r[0]
	}
	text := func(v *mir2.VM, s string) mir2.Value { return v.AllocHeap(append([]byte(s), 0)) }
	for i, v := range []*mir2.VM{vms[0], vms[0], vms[1]} {
		handle := call(v, "sqlite_open", text(v, ":memory:"))
		if handle.I == 0 {
			t.Fatal("open failed")
		}
		defer v.Hosts["sqlite_close"]([]mir2.Value{handle})
		value := int64(7 + 4*i)
		for _, sql := range []string{"CREATE TABLE t(n)", fmt.Sprintf("INSERT INTO t VALUES (%d)", value)} {
			if call(v, "sqlite_exec", handle, text(v, sql)).I != 0 {
				t.Fatal("isolated seed failed")
			}
		}
		databases = append(databases, database{v, handle, value})
	}
	for _, db := range databases {
		var cursors []mir2.Value
		// Keep the first cursor alive to force another connection to the same DB.
		for n := 0; n < 2; n++ {
			cursor := call(db.vm, "sqlite_query", db.handle, text(db.vm, "SELECT n FROM t"))
			if cursor.I == 0 {
				t.Fatal("nested query lost in-memory table")
			}
			cursors = append(cursors, cursor)
			if call(db.vm, "sqlite_step", cursor).I != 1 {
				t.Fatal("missing row")
			}
			if got := call(db.vm, "sqlite_column_int", cursor, mir2.Value{I: 0}).I; got != db.value {
				t.Fatalf("database contents crossed handle/VM: %d want %d", got, db.value)
			}
		}
		for _, cursor := range cursors {
			if call(db.vm, "sqlite_finalize", cursor).I != 0 {
				t.Fatal("close failed")
			}
		}
	}
}
