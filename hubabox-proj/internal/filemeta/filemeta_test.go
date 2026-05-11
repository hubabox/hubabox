package filemeta

import "testing"

func TestKind(t *testing.T) {
	if got := Kind("x.PDF"); got != "pdf" {
		t.Errorf("Kind(PDF)=%q", got)
	}
	if got := Kind("a.png"); got != "image" {
		t.Errorf("Kind(png)=%q", got)
	}
}

func TestHumanSize(t *testing.T) {
	if HumanSize(500) != "500 B" {
		t.Errorf("HumanSize(500)=%q", HumanSize(500))
	}
	if HumanSize(2048) != "2.0 KB" {
		t.Errorf("HumanSize(2048)=%q", HumanSize(2048))
	}
}
