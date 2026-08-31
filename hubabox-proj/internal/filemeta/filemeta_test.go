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

func TestPreviewable(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"photo.jpeg", true},
		{"dir/photo.png", true},
		{"x.svg", false},
		{"doc.pdf", true},
		{"clip.mp3", true},
		{"clip.mp4", true},
		{"clip.webm", true},
		{"a.mkv", false},
		{"a.mov", false},
		{"a.avi", false},
		{"readme.txt", true},
		{"readme.md", true},
		{"sheet.csv", true},
		{"blob.exe", false},
		{"x.zip", false},
		{"x.docx", false},
	}
	for _, tc := range tests {
		if got := Previewable(tc.name); got != tc.want {
			t.Errorf("Previewable(%q)=%v want %v", tc.name, got, tc.want)
		}
	}
}

func TestPreviewContentType_textIsPlain(t *testing.T) {
	if got := PreviewContentType("notes.html"); got != "text/plain; charset=utf-8" {
		t.Fatalf("PreviewContentType(html as code ext)=%q want text/plain", got)
	}
	if got := PreviewContentType("x.pdf"); got != "application/pdf" {
		t.Fatalf("PreviewContentType(pdf)=%q", got)
	}
}
