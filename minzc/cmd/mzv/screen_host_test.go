package main

import (
	"os"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestSelectionHostSigned32Input(t *testing.T) {
	for _, c := range []struct {
		input string
		value int64
		bad   bool
	}{
		{"\n", -65537, false}, {"2147483647\n", 2147483647, false},
		{"-2147483648\n", -2147483648, false}, {"  +65536  \n", 65536, false},
		{"2147483648\n", -65537, true}, {"-2147483649\n", -65537, true},
		{"1x\n", -65537, true}, {"\t1\n", -65537, true}, {" \n", -65537, true},
	} {
		t.Run(c.input, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if _, err = w.WriteString(c.input); err != nil {
				t.Fatal(err)
			}
			w.Close()
			old := os.Stdin
			os.Stdin = r
			defer func() { os.Stdin = old }()
			v := mir2.NewVM(&mir2.Module{})
			registerScreenHosts(v, true, false)
			name := v.AllocHeap([]byte("P\x00"))
			if _, err = v.Hosts["sel_register_int"]([]mir2.Value{name, {I: -65537}}); err != nil {
				t.Fatal(err)
			}
			_, err = v.Hosts["sel_show"](nil)
			if (err != nil) != c.bad {
				t.Fatalf("show error=%v expected bad=%t", err, c.bad)
			}
			got, err := v.Hosts["sel_get_int"]([]mir2.Value{{I: 0}})
			if err != nil || len(got) != 1 || got[0].I != c.value {
				t.Fatalf("get=%v error=%v want %d", got, err, c.value)
			}
		})
	}
}
