package librarychat

import "testing"

func TestVoiceContentTypeForExt(t *testing.T) {
	if g := VoiceContentTypeForExt(".aac"); g != "audio/aac" {
		t.Fatalf("aac: %q", g)
	}
	if g := VoiceContentTypeForExt(".m4a"); g != "audio/mp4" {
		t.Fatalf("m4a: %q", g)
	}
}

func TestVoiceContentTypeForM4AFile(t *testing.T) {
	iso := []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'M', '4', 'A', ' '}
	if g := VoiceContentTypeForM4AFile(iso); g != "audio/mp4" {
		t.Fatalf("M4A : got %q", g)
	}
	threegp := []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', '3', 'g', 'p', '5'}
	if g := VoiceContentTypeForM4AFile(threegp); g != "audio/3gpp" {
		t.Fatalf("3gp5: got %q", g)
	}
}
