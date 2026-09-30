package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/oisee/z80-optimizer/pkg/enr"
	"os"
)

func main() {
	b := []byte{'E', 'N', 'R', 'T', 1, 0, 0, 0}
	b = binary.LittleEndian.AppendUint32(b, 2)
	b = append(b, 4, 12, 0, 0, 255)
	r, e := enr.NewReader(bytes.NewReader(b))
	if e != nil {
		panic(e)
	}
	_, e1 := r.Next()
	_, e2 := r.Next()
	json.NewEncoder(os.Stdout).Encode(map[string]any{"declared": r.NEntries, "read": r.Position(), "first_error": fmt.Sprint(e1), "second_error": fmt.Sprint(e2)})
}
