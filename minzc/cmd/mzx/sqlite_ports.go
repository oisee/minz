package main

// SQLite bridge via I/O ports for MZX (ZX Spectrum emulator).
// Same protocol as mze's sqlite_ports.go but uses spectrum.Ports API.
//
// Ports: $41 (cmd), $43 (status), $45 (data), $47 (datahi)

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"

	"github.com/minz/minzc/pkg/spectrum"
)

func setupMZXSQLitePorts(machine *spectrum.Machine, verbose bool) {
	dbs := make(map[byte]*sql.DB)
	nextDB := byte(1)

	type stmtState struct {
		rows *sql.Rows
		vals []interface{}
	}
	stmts := make(map[byte]*stmtState)
	nextStmt := byte(1)

	var strBuf []byte
	var readBuf []byte
	var readPos int
	var dataByte byte
	var dataHi byte
	var status byte
	var resultLo byte
	var resultHi byte
	var readingStr bool

	// Port $41 — command (write only)
	machine.Ports.Register(&spectrum.PortDevice{
		Mask:  0x00FF,
		Value: 0x0041,
		Write: func(addr uint16, val byte) {
			cmd := val
			switch cmd {
			case 1: // open
				name := string(strBuf)
				strBuf = nil
				db, err := sql.Open("sqlite", name)
				if err != nil {
					status = 1
					if verbose {
						fmt.Fprintf(os.Stderr, "  sql:open(%q) → error: %v\n", name, err)
					}
					return
				}
				db.SetMaxIdleConns(1)
				h := nextDB
				dbs[h] = db
				nextDB++
				resultLo = h
				status = 0
				if verbose {
					fmt.Fprintf(os.Stderr, "  sql:open(%q) → handle %d\n", name, h)
				}

			case 2: // close
				h := dataByte
				if db, ok := dbs[h]; ok {
					db.Close()
					delete(dbs, h)
				}
				status = 0

			case 3: // query
				h := dataHi
				sqlStr := string(strBuf)
				strBuf = nil
				db, ok := dbs[h]
				if !ok {
					status = 1
					if verbose {
						fmt.Fprintf(os.Stderr, "   sql:query(%d, %q) error: no such handle\n", h, sqlStr)
					}
					return
				}
				rows, err := db.Query(sqlStr)
				if err != nil {
					status = 1
					if verbose {
						fmt.Fprintf(os.Stderr, "   sql:query(%d, %q) error: %v\n", h, sqlStr, err)
					}
					return
				}
				cols, _ := rows.Columns()
				sh := nextStmt
				stmts[sh] = &stmtState{rows: rows, vals: make([]interface{}, len(cols))}
				nextStmt++
				resultLo = sh
				status = 0
				if verbose {
					fmt.Fprintf(os.Stderr, "  sql:query(%d, %q) → stmt %d (%d cols)\n", h, sqlStr, sh, len(cols))
				}

			case 4: // step
				sh := dataByte
				st, ok := stmts[sh]
				if !ok {
					resultLo = 0
					status = 0
					return
				}
				if st.rows.Next() {
					ptrs := make([]interface{}, len(st.vals))
					for i := range st.vals {
						ptrs[i] = &st.vals[i]
					}
					st.rows.Scan(ptrs...)
					resultLo = 1
					if verbose {
						fmt.Fprintf(os.Stderr, "  sql:step(%d) → row %v\n", sh, st.vals)
					}
				} else {
					resultLo = 0
					if verbose {
						fmt.Fprintf(os.Stderr, "  sql:step(%d) → done\n", sh)
					}
				}
				status = 0

			case 5: // column_text
				sh := dataByte
				col := int(dataHi)
				st, ok := stmts[sh]
				if !ok || col >= len(st.vals) {
					readBuf = []byte{0}
					readPos = 0
					readingStr = true
					return
				}
				s := fmt.Sprintf("%v", st.vals[col])
				readBuf = append([]byte(s), 0)
				readPos = 0
				readingStr = true
				if verbose {
					fmt.Fprintf(os.Stderr, "  sql:col_text(%d, %d) → %q\n", sh, col, s)
				}

			case 6: // column_int
				sh := dataByte
				col := int(dataHi)
				st, ok := stmts[sh]
				if !ok || col >= len(st.vals) {
					resultLo = 0
					resultHi = 0
					return
				}
				var v int64
				switch val := st.vals[col].(type) {
				case int64:
					v = val
				case float64:
					v = int64(val)
				}
				resultLo = byte(v & 0xFF)
				resultHi = byte((v >> 8) & 0xFF)
				status = 0

			case 7: // finalize
				sh := dataByte
				if st, ok := stmts[sh]; ok {
					if st.rows != nil {
						st.rows.Close()
					}
					delete(stmts, sh)
				}
				status = 0
				if verbose {
					fmt.Fprintf(os.Stderr, "  sql:finalize(%d)\n", sh)
				}

			case 8: // exec
				h := dataHi
				sqlStr := string(strBuf)
				strBuf = nil
				db, ok := dbs[h]
				if !ok {
					status = 1
					return
				}
				_, err := db.Exec(sqlStr)
				if err != nil {
					status = 1
					if verbose {
						fmt.Fprintf(os.Stderr, "  sql:exec(%d, %q) error: %v\n", h, sqlStr, err)
					}
				} else {
					status = 0
					if verbose {
						fmt.Fprintf(os.Stderr, "  sql:exec(%d, %q) → ok\n", h, sqlStr)
					}
				}
			}
		},
		Name: "SQLite-cmd",
	})

	// Port $43 — status (read only)
	machine.Ports.Register(&spectrum.PortDevice{
		Mask:  0x00FF,
		Value: 0x0043,
		Read: func(addr uint16) byte {
			return status
		},
		Name: "SQLite-status",
	})

	// Port $45 — data (read/write)
	machine.Ports.Register(&spectrum.PortDevice{
		Mask:  0x00FF,
		Value: 0x0045,
		Read: func(addr uint16) byte {
			if readingStr && readPos < len(readBuf) {
				b := readBuf[readPos]
				readPos++
				if b == 0 {
					readingStr = false
				}
				return b
			}
			if readingStr {
				readingStr = false
				return 0
			}
			return resultLo
		},
		Write: func(addr uint16, val byte) {
			dataByte = val
			if val == 0 {
				// null terminator — string complete
			} else {
				strBuf = append(strBuf, val)
			}
		},
		Name: "SQLite-data",
	})

	// Port $47 — datahi (read/write)
	machine.Ports.Register(&spectrum.PortDevice{
		Mask:  0x00FF,
		Value: 0x0047,
		Read: func(addr uint16) byte {
			return resultHi
		},
		Write: func(addr uint16, val byte) {
			dataHi = val
		},
		Name: "SQLite-datahi",
	})

	if verbose {
		fmt.Fprintf(os.Stderr, "MZX: SQLite I/O ports registered ($41/$43/$45/$47)\n")
	}
}
