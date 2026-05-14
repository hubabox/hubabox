package librarychat

import "testing"

func TestSanitizeVoiceBasename(t *testing.T) {
	ok := "0123456789abcdef0123456789abcdef.webm"
	got, err := SanitizeVoiceBasename(ok)
	if err != nil || got != ok {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := SanitizeVoiceBasename("../../../etc/passwd"); err == nil {
		t.Fatal("want reject")
	}
	if _, err := SanitizeVoiceBasename("short.webm"); err == nil {
		t.Fatal("want reject short")
	}
	for _, ext := range []string{"mp3", "m4a", "aac", "flac", "caf"} {
		name := "0123456789abcdef0123456789abcdef." + ext
		got, err := SanitizeVoiceBasename(name)
		if err != nil || got != name {
			t.Fatalf("%s: got %q err=%v", ext, got, err)
		}
	}
}
