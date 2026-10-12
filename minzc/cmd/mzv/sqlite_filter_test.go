package main

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"testing"
)

func TestSQLiteFilteredQueryRealOrderedRows(t *testing.T) {
	db, e := sql.Open("sqlite", ":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.Exec("CREATE TABLE scarr(carrid TEXT);INSERT INTO scarr VALUES ('BA'),('LH'),('AA'),('O''H')"); e != nil {
		t.Fatal(e)
	}
	base := "SELECT carrid FROM scarr ORDER BY carrid"
	for _, c := range []struct {
		filter string
		want   []string
	}{{"*", []string{"AA", "BA", "LH", "O'H"}}, {"LH", []string{"LH"}}, {"A*", []string{"AA"}}, {"*A*", []string{"AA", "BA"}}, {"O'H", []string{"O'H"}}, {"x' OR 1=1 --", nil}} {
		q, args, e := sqliteFilteredQuery(base, "carrid", c.filter)
		if e != nil {
			t.Fatal(e)
		}
		rows, e := db.Query(q, args...)
		if e != nil {
			t.Fatal(c.filter, q, e)
		}
		var got []string
		for rows.Next() {
			var v string
			if e = rows.Scan(&v); e != nil {
				t.Fatal(e)
			}
			got = append(got, v)
		}
		if e := rows.Err(); e != nil {
			t.Fatal(e)
		}
		rows.Close()
		if len(got) != len(c.want) {
			t.Fatalf("%q got%v want%v", c.filter, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatal(got, c.want)
			}
		}
	}
	q, args, e := sqliteFilteredQuery("SELECT carrid FROM scarr WHERE carrid!='BA' ORDER BY carrid LIMIT 3;", "carrid", "*H*")
	if e != nil {
		t.Fatal(e)
	}
	rows, e := db.Query(q, args...)
	if e != nil {
		t.Fatal(q, e)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var v string
		if e := rows.Scan(&v); e != nil {
			t.Fatal(e)
		}
		got = append(got, v)
	}
	if len(got) != 2 || got[0] != "LH" || got[1] != "O'H" {
		t.Fatal(got)
	}
	if e := rows.Err(); e != nil {
		t.Fatal(e)
	}
	rows.Close()
	q, args, e = sqliteFilteredQuery("SELECT carrid FROM scarr ORDER BY carrid LIMIT 1", "carrid", "LH")
	if e != nil {
		t.Fatal(e)
	}
	limited, e := db.Query(q, args...)
	if e != nil {
		t.Fatal(e)
	}
	if limited.Next() {
		t.Fatal("filter applied before LIMIT")
	}
	if e := limited.Err(); e != nil {
		t.Fatal(e)
	}
	limited.Close()
	q, args, e = sqliteFilteredQuery(base, "unknown", "LH")
	if e != nil {
		t.Fatal(e)
	}
	if unknown, e := db.Query(q, args...); e == nil {
		unknown.Close()
		t.Fatal("unknown outputcolumn became DQS literal")
	}

	for _, column := range []string{"", "carrid) OR 1=1--", "schema.carrid"} {
		if _, _, e := sqliteFilteredQuery(base, column, "LH"); e == nil {
			t.Fatal("accepted executable column", column)
		}
	}
}
