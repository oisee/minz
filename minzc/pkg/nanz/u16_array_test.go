package nanz_test

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestU16ArrayRoundTrip(t *testing.T) {
	src := `
global vals: [u16; 4] = [0, 0, 0, 0]
global bytes: [u8; 2] = [0, 0]

fun store_read() -> u16 {
    vals[1] = 1695
    return vals[1]
}

fun neighbors() -> u16 {
    vals[0] = 4660
    vals[1] = 22136
    return vals[0]
}

fun read_at(i: u8) -> u16 {
    return vals[i]
}

fun local_array() -> u16 {
    var buf: [u16; 3]
    buf[0] = 258
    buf[2] = 43981
    return buf[2]
}

fun local_initializer() -> u16 {
    let buf: [u16; 3] = [258, 1695, 43981]
    return buf[1]
}

fun byte_array() -> u8 {
    bytes[1] = 207
    return bytes[1]
}
`
	hm, err := nanz.Parse(src, "u16_array.nanz")
	if err != nil {
		t.Fatal(err)
	}
	module := hir.LowerModule(hm)
	if err := mir2.Verify(module); err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(module)
	for _, tc := range []struct {
		fn   string
		args []mir2.Value
		want int64
	}{
		{"store_read", nil, 1695},
		{"neighbors", nil, 4660},
		{"read_at", []mir2.Value{{I: 1}}, 22136},
		{"local_array", nil, 43981},
		{"local_initializer", nil, 1695},
		{"byte_array", nil, 207},
	} {
		got, err := vm.Call(tc.fn, tc.args)
		if err != nil {
			t.Fatalf("%s: %v", tc.fn, err)
		}
		if len(got) != 1 || got[0].I != tc.want {
			t.Fatalf("%s = %v, want %d", tc.fn, got, tc.want)
		}
	}
}
