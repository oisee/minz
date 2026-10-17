package main

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestSQLiteIteratorErrorIsNotExhaustion(t *testing.T) {
	v := mir2.NewVM(&mir2.Module{})
	registerSQLiteHosts(v, false)
	call := func(name string, args ...mir2.Value) mir2.Value {
		t.Helper()
		values, err := v.Hosts[name](args)
		if err != nil || len(values) != 1 {
			t.Fatalf("%s: %v %v", name, values, err)
		}
		return values[0]
	}
	text := func(s string) mir2.Value { return v.AllocHeap(append([]byte(s), 0)) }
	db := call("sqlite_open", text(":memory:"))
	defer call("sqlite_close", db)
	// The first row succeeds; the second row raises SQLite integer overflow.
	query := "WITH RECURSIVE c(n) AS (VALUES(0) UNION ALL SELECT n+1 FROM c WHERE n<1) SELECT CASE WHEN n=0 THEN 7 ELSE abs(-9223372036854775808) END FROM c"
	stmt := call("sqlite_query", db, text(query))
	if stmt.I == 0 {
		t.Fatal("query failed before iterator test")
	}
	if got := call("sqlite_step", stmt).I; got != 1 {
		t.Fatalf("first step=%d", got)
	}
	if got := call("sqlite_column_int", stmt, mir2.Value{I: 0}).I; got != 7 {
		t.Fatalf("first value=%d", got)
	}
	if got := call("sqlite_step", stmt).I; got != 2 {
		t.Fatalf("iterator error returned %d; expected error 2", got)
	}
	if got := call("sqlite_step", stmt).I; got != 2 {
		t.Fatalf("closed handle returned %d", got)
	}
}
