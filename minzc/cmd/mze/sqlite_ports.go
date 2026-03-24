package main

// SQLite bridge via I/O ports for CP/M Z80 programs.
//
// Port protocol (odd ports, avoids eZ80 $80-$FF):
//   $41 (cmd):    write triggers command execution
//   $43 (status): read returns last status/result
//   $45 (data):   read/write data bytes (string + handle transfer)
//   $47 (datahi): read/write hi byte (u16 values, column index)
//
// String transfer: write bytes to $45, null-terminated. Read likewise.
// Handle transfer: single byte via $45 before command.

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"

	"github.com/minz/minzc/pkg/emulator"
)

type sqlStmtState struct {
	rows *sql.Rows
	vals []interface{}
}

func setupSQLitePorts(z80 *emulator.RemogattoZ80WithScreen, verbose bool) {
	// State
	dbs := make(map[byte]*sql.DB)
	nextDB := byte(1)

	stmts := make(map[byte]*sqlStmtState)
	nextStmt := byte(1)

	// Buffers for port protocol
	var strBuf []byte    // accumulates string bytes written to $45
	var readBuf []byte   // string being read from $45
	var readPos int      // current position in readBuf
	var dataByte byte    // last byte written to $45 (handle/param)
	var dataHi byte      // last byte written to $47
	var status byte      // last status (read from $43)
	var resultLo byte    // result lo byte (read from $45 after command)
	var resultHi byte    // result hi byte (read from $47 after command)
	var readingStr bool  // true when reading string result from $45

	// Set up I/O port handlers
	prevRead := z80.RemogattoZ80.GetIORead()
	prevWrite := z80.RemogattoZ80.GetIOWrite()

	z80.RemogattoZ80.SetIOHandlers(
		// Read handler
		func(port uint16) byte {
			p := byte(port & 0xFF)
			switch p {
			case 0x43: // status
				return status
			case 0x45: // data read
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
					return 0 // null terminator
				}
				return resultLo
			case 0x47: // datahi read
				return resultHi
			default:
				if prevRead != nil {
					return prevRead(port)
				}
				return 0xFF
			}
		},
		// Write handler
		func(port uint16, value byte) {
			p := byte(port & 0xFF)
			switch p {
			case 0x41: // command
				execSQLiteCmd(value, &strBuf, dataByte, dataHi,
					&status, &resultLo, &resultHi,
					&readBuf, &readPos, &readingStr,
					dbs, &nextDB, stmts, &nextStmt, verbose)
				strBuf = nil // reset string buffer after command
			case 0x45: // data write
				dataByte = value
				if value == 0 {
					// null terminator — string complete (already in strBuf)
				} else {
					strBuf = append(strBuf, value)
				}
			case 0x47: // datahi write
				dataHi = value
			default:
				if prevWrite != nil {
					prevWrite(port, value)
				}
			}
		},
	)

	if verbose {
		fmt.Fprintf(os.Stderr, "mze: SQLite I/O ports registered ($41/$43/$45/$47)\n")
	}
}

func execSQLiteCmd(cmd byte, strBuf *[]byte, dataByte, dataHi byte,
	status, resultLo, resultHi *byte,
	readBuf *[]byte, readPos *int, readingStr *bool,
	dbs map[byte]*sql.DB, nextDB *byte,
	stmts map[byte]*sqlStmtState, nextStmt *byte, verbose bool) {

	str := string(*strBuf)

	switch cmd {
	case 1: // open
		db, err := sql.Open("sqlite", str)
		if err != nil {
			*status = 0 // error: handle = 0
			if verbose {
				fmt.Fprintf(os.Stderr, "  sql:open(%q) error: %v\n", str, err)
			}
			return
		}
		db.SetMaxIdleConns(1)
		h := *nextDB
		dbs[h] = db
		*nextDB++
		*status = h
		if verbose {
			fmt.Fprintf(os.Stderr, "  sql:open(%q) → handle %d\n", str, h)
		}

	case 2: // close
		h := dataByte
		if db, ok := dbs[h]; ok {
			db.Close()
			delete(dbs, h)
		}
		*status = 0
		if verbose {
			fmt.Fprintf(os.Stderr, "  sql:close(%d)\n", h)
		}

	case 3: // query (handle in dataHi, SQL in strBuf)
		h := dataHi
		db, ok := dbs[h]
		if !ok {
			*status = 0
			return
		}
		rows, err := db.Query(str)
		if err != nil {
			*status = 0
			if verbose {
				fmt.Fprintf(os.Stderr, "  sql:query(%d, %q) error: %v\n", h, str, err)
			}
			return
		}
		sh := *nextStmt
		stmts[sh] = &sqlStmtState{rows: rows}
		*nextStmt++
		*status = sh
		if verbose {
			fmt.Fprintf(os.Stderr, "  sql:query(%d, %q) → stmt %d\n", h, str, sh)
		}

	case 4: // step
		sh := dataByte
		st, ok := stmts[sh]
		if !ok {
			*status = 3 // SQLITE_DONE
			return
		}
		if st.rows.Next() {
			cols, _ := st.rows.Columns()
			st.vals = make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range st.vals {
				ptrs[i] = &st.vals[i]
			}
			st.rows.Scan(ptrs...)
			*status = 2 // SQLITE_ROW
		} else {
			st.rows.Close()
			delete(stmts, sh)
			*status = 3 // SQLITE_DONE
			if verbose {
				fmt.Fprintf(os.Stderr, "  sql:step(%d) → done\n", sh)
			}
		}

	case 5: // column_text
		sh := dataByte
		col := int(dataHi)
		st, ok := stmts[sh]
		if !ok || col >= len(st.vals) {
			*readBuf = []byte{0}
			*readPos = 0
			*readingStr = true
			return
		}
		s := fmt.Sprintf("%v", st.vals[col])
		*readBuf = append([]byte(s), 0)
		*readPos = 0
		*readingStr = true
		if verbose {
			fmt.Fprintf(os.Stderr, "  sql:col_text(%d, %d) → %q\n", sh, col, s)
		}

	case 6: // column_int
		sh := dataByte
		col := int(dataHi)
		st, ok := stmts[sh]
		if !ok || col >= len(st.vals) {
			*resultLo = 0
			*resultHi = 0
			return
		}
		var v int64
		switch val := st.vals[col].(type) {
		case int64:
			v = val
		case float64:
			v = int64(val)
		case []byte:
			fmt.Sscanf(string(val), "%d", &v)
		case string:
			fmt.Sscanf(val, "%d", &v)
		}
		*resultLo = byte(v & 0xFF)
		*resultHi = byte((v >> 8) & 0xFF)
		if verbose {
			fmt.Fprintf(os.Stderr, "  sql:col_int(%d, %d) → %d\n", sh, col, v)
		}

	case 7: // finalize
		sh := dataByte
		if st, ok := stmts[sh]; ok {
			st.rows.Close()
			delete(stmts, sh)
		}
		*status = 0
		if verbose {
			fmt.Fprintf(os.Stderr, "  sql:finalize(%d)\n", sh)
		}

	case 8: // exec (handle in dataHi, SQL in strBuf)
		h := dataHi
		db, ok := dbs[h]
		if !ok {
			*status = 1
			return
		}
		_, err := db.Exec(str)
		if err != nil {
			*status = 1
			if verbose {
				fmt.Fprintf(os.Stderr, "  sql:exec(%d, %q) error: %v\n", h, str, err)
			}
		} else {
			*status = 0
			if verbose {
				fmt.Fprintf(os.Stderr, "  sql:exec(%d, %q) → ok\n", h, str)
			}
		}
	}
}
