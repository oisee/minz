package main

import (
	"bufio"
	"os"
	"strings"
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

func TestSelectionHostOwnedCharacterInput(t *testing.T) {
	for _, length := range []int{3, 300} {
		for _, input := range []string{"", "A", "ABCDE", "   ", "\x00A", "é"} {
			t.Run(input, func(t *testing.T) {
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				if _, err := w.WriteString(input + "\n"); err != nil {
					t.Fatal(err)
				}
				w.Close()
				old := os.Stdin
				os.Stdin = r
				defer func() { os.Stdin = old }()
				initial := []byte(strings.Repeat(" ", length))
				copy(initial, []byte("XYZ"))
				v := mir2.NewVM(&mir2.Module{Globals: []mir2.Global{
					{Name: "p", Ty: mir2.NewArray(mir2.TyU8, length), Init: initial},
					{Name: "guard", Ty: mir2.TyU8, Init: []byte{0xa5}},
				}})
				registerScreenHosts(v, true, false)
				name := v.AllocHeap([]byte("P\x00"))
				// Deliberately use the address-zero object to cover that ABI case.
				if v.GlobalAddr("p") != 0 {
					t.Fatal("expected address-zero fixture")
				}
				if _, err := v.Hosts["sel_register_c"]([]mir2.Value{name, {I: int64(length)}, {I: 0}}); err != nil {
					t.Fatal(err)
				}
				_, err = v.Hosts["sel_show"](nil)
				bad := input == "é"
				if (err != nil) != bad {
					t.Fatalf("show error=%v expected bad=%t", err, bad)
				}
				want := append([]byte(nil), initial...)
				if input != "" && !bad {
					want = []byte(strings.Repeat(" ", length))
					copy(want, []byte(input))
				}
				if got := string(v.ReadHeap(0, length)); got != string(want) {
					t.Fatalf("field=%q want %q", got, want)
				}
				if got := v.ReadHeap(v.GlobalAddr("guard"), 1)[0]; got != 0xa5 {
					t.Fatalf("fixed field wrote beyond its object: guard=%02x", got)
				}
			})
		}
	}
}

func TestSelectionHostConsumesSharedStdinTranscript(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	defer r.Close()
	oldFile := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldFile }()
	input := make(chan byte, 32)
	for _, b := range []byte("A\n65536\nq") {
		input <- b
	}
	close(input)
	oldChannel := stdinForTUI
	stdinForTUI = input
	defer func() { stdinForTUI = oldChannel }()
	v := mir2.NewVM(&mir2.Module{Globals: []mir2.Global{
		{Name: "p", Ty: mir2.NewArray(mir2.TyU8, 3), Init: []byte("XYZ")},
		{Name: "guard", Ty: mir2.TyU8, Init: []byte{0xa5}},
	}})
	registerScreenHosts(v, true, false)
	name := v.AllocHeap([]byte("FIELD\x00"))
	if _, err := v.Hosts["sel_register_c"]([]mir2.Value{name, {I: 3}, {I: 0}}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Hosts["sel_register_int"]([]mir2.Value{name, {I: -7}}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Hosts["sel_show"](nil); err != nil {
		t.Fatal(err)
	}
	if got := string(v.ReadHeap(0, 3)); got != "A  " {
		t.Fatalf("first field=%q", got)
	}
	value, err := v.Hosts["sel_get_int"]([]mir2.Value{{I: 1}})
	if err != nil || len(value) != 1 || value[0].I != 65536 {
		t.Fatalf("second field=%v error=%v", value, err)
	}
	if v.ReadHeap(v.GlobalAddr("guard"), 1)[0] != 0xa5 {
		t.Fatal("character-field boundary overwritten")
	}
	if next, ok := <-input; !ok || next != 'q' {
		t.Fatal("selection reader swallowed later program input")
	}
}

func TestSelectionKeysFromSharedChannel(t *testing.T) {
	for _, key := range []string{"\x1b[A", "\x1b[B", "\x1b[19~", "\x1b", "\t"} {
		input := make(chan byte, 16)
		for _, b := range []byte(key) {
			input <- b
		}
		close(input)
		reader := bufio.NewReader(selectionChannelReader{input})
		got, err := readSelectionKey(reader, input)
		if err != nil || string(got) != key {
			t.Fatalf("key=%q error=%v want %q", got, err, key)
		}
	}
}
