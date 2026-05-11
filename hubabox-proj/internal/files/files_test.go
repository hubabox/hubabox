package files

import "testing"

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  bool
	}{
		{"doc.pdf", "doc.pdf", false},
		{"/etc/passwd", "passwd", false},
		{"..", "", true},
		{"", "", true},
		{"  ok.txt  ", "ok.txt", false},
	}
	for _, tc := range tests {
		got, err := SanitizeName(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("%q: want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}
