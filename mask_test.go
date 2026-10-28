package log

import "testing"

func TestMaskPhone(t *testing.T) {
	cases := map[string]string{
		"":               "",
		"123":            "***",
		"19912341002":    "199****1002",
		"+8619912341002": "+86199****1002",
		"199-1234-1002":  "199-****-1002",
	}
	for in, want := range cases {
		if got := MaskPhone(in); got != want {
			t.Fatalf("MaskPhone(%q)=%q want %q", in, got, want)
		}
	}
}
