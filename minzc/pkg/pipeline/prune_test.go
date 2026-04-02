package pipeline

import "testing"

func TestFunctionPruneRootClassification(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "main", want: true},
		{name: "compute", want: true},
		{name: "Acc_add", want: true},
		{name: "lib__math__add", want: false},
		{name: "foo__bar__baz", want: false},
		{name: "__tag", want: false},
		{name: "__payload", want: false},
		{name: "__mpay_0", want: false},
		{name: "__arr_0", want: false},
		{name: "lambda_0", want: false},
	}

	for _, tc := range tests {
		if got := isUserFacingPruneRoot(tc.name); got != tc.want {
			t.Errorf("%s: root=%v, want %v", tc.name, got, tc.want)
		}
	}
}
