package asmutil

import "testing"

func TestSanitizeZ80Label(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"@mir2.str.0", "_mir2_str_0"}, {"fun-name$arg", "fun_name_arg"},
		{"A", "v_A"}, {"a", "v_a"}, {"ixh", "v_ixh"}, {"SP", "v_SP"},
		{"normal_9", "normal_9"}, {"", ""}, {"A'", "A'"}, {"with space", "with space"}, {"имя", "имя"},
	} {
		if got := SanitizeZ80Label(c.in); got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}
