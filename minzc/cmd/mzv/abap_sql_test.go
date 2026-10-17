package main

import (
	"encoding/binary"
	"testing"

	"github.com/minz/minzc/pkg/abap"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestABAPSQLActualSQLiteTransport(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		want      int32
		status    int32
		fault     byte
	}{
		{"wide", "INSERT INTO t VALUES (65537)", 65537, 0, 0},
		{"negative", "INSERT INTO t VALUES (-65537)", -65537, 0, 0},
		{"max", "INSERT INTO t VALUES (2147483647)", 2147483647, 0, 0},
		{"min", "INSERT INTO t VALUES (-2147483648)", -2147483648, 0, 0},
		{"missing", "", 17, 4, 0},
		{"overflow", "INSERT INTO t VALUES (2147483648)", 17, 0, 5},
		{"null", "INSERT INTO t VALUES (NULL)", 17, 0, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := abap.Compile("REPORT zsql. DATA x TYPE i VALUE 17. DATA y TYPE i. SELECT SINGLE n FROM t INTO x. y = sy-subrc.", "zsql")
			if err != nil {
				t.Fatal(err)
			}
			steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
				v := mir2.NewVM(module)
				registerSQLiteHosts(v, false)
				registerABAPHosts(v, true, false)
				call := func(name string, args ...mir2.Value) []mir2.Value {
					t.Helper()
					r, err := v.Hosts[name](args)
					if err != nil {
						t.Fatal(err)
					}
					return r
				}
				str := func(s string) mir2.Value { return v.AllocHeap(append([]byte(s), 0)) }
				db := call("sqlite_open", str(":memory:"))[0]
				call("sqlite_exec", db, str("CREATE TABLE t(n);"))
				if tc.sql != "" {
					call("sqlite_exec", db, str(tc.sql))
				}
				// The test initializes an actual SQLite database, then injects only its
				// open handle. Query/step/column/finalize run through the real host API.
				v.Hosts["sqlite_open"] = func([]mir2.Value) ([]mir2.Value, error) { return []mir2.Value{db}, nil }
				_, err = v.Call("main", nil)
				if (err != nil) != (tc.fault != 0) {
					t.Fatalf("error %v want fault %d", err, tc.fault)
				}
				if code := v.ReadHeap(v.GlobalAddr("_abap_error_code"), 1)[0]; code != tc.fault {
					t.Fatalf("fault %d want %d", code, tc.fault)
				}
				for name, want := range map[string]int32{"x": tc.want, "y": tc.status} {
					if got := int32(binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr(name), 4))); got != want {
						t.Fatalf("%s=%d want %d", name, got, want)
					}
				}
				if tc.fault != 0 { // terminal faults intentionally retain the active statement
					call("sqlite_finalize", mir2.Value{I: 1})
				}
				call("sqlite_close", db)
			}
		})
	}
}

func TestABAPSQLActualSQLiteCleanup(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		closes     []int64
	}{
		{"return", "FORM probe. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDFORM. PERFORM probe.", []int64{1}},
		{"nested-return", "FORM probe. SELECT n FROM t INTO x. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDSELECT. ENDFORM. PERFORM probe.", []int64{2, 1}},
		{"caller-isolation", "FORM probe. SELECT n FROM t INTO x. RETURN. ENDSELECT. ENDFORM. SELECT n FROM t INTO x. PERFORM probe. EXIT. ENDSELECT.", []int64{2, 1}},
		{"continue", "SELECT n FROM t INTO x. CONTINUE. x = 99. ENDSELECT.", []int64{1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := abap.Compile("REPORT zsql. DATA x TYPE i VALUE 17. "+tc.body, "zsql")
			if err != nil {
				t.Fatal(err)
			}
			steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
				v := mir2.NewVM(module)
				registerSQLiteHosts(v, false)
				registerABAPHosts(v, true, false)
				str := func(s string) mir2.Value { return v.AllocHeap(append([]byte(s), 0)) }
				opened, err := v.Hosts["sqlite_open"]([]mir2.Value{str(":memory:")})
				if err != nil || len(opened) != 1 || opened[0].I == 0 {
					t.Fatalf("open %v %v", opened, err)
				}
				db := opened[0]
				defer v.Hosts["sqlite_close"]([]mir2.Value{db})
				for _, sql := range []string{"CREATE TABLE t(n)", "INSERT INTO t VALUES (65537),(65537),(65537)"} {
					r, err := v.Hosts["sqlite_exec"]([]mir2.Value{db, str(sql)})
					if err != nil || len(r) != 1 || r[0].I != 0 {
						t.Fatalf("seed %v %v", r, err)
					}
				}
				v.Hosts["sqlite_open"] = func([]mir2.Value) ([]mir2.Value, error) { return []mir2.Value{db}, nil }
				finalize := v.Hosts["sqlite_finalize"]
				var closed []int64
				v.Hosts["sqlite_finalize"] = func(args []mir2.Value) ([]mir2.Value, error) {
					closed = append(closed, args[0].I)
					return finalize(args)
				}
				if _, err := v.Call("main", nil); err != nil {
					t.Fatal(err)
				}
				if len(closed) != len(tc.closes) {
					t.Fatalf("closed=%v want %v", closed, tc.closes)
				}
				for i, h := range closed {
					if h != tc.closes[i] {
						t.Fatalf("close order=%v want %v", closed, tc.closes)
					}
				}
				if got := int32(binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr("x"), 4))); got != 65537 {
					t.Fatalf("x=%d", got)
				}
				if code := v.ReadHeap(v.GlobalAddr("_abap_error_code"), 1)[0]; code != 0 {
					t.Fatalf("fault=%d", code)
				}
			}
		})
	}
}
