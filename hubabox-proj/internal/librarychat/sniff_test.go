package librarychat

import (
	"bytes"
	"testing"
)

func TestSniffVoiceExtension(t *testing.T) {
	cases := []struct {
		prefix []byte
		want   string
	}{
		{[]byte("fLaC\x00\x00"), ".flac"},
		{[]byte("OggS\x00\x00"), ".ogg"},
		{[]byte("ID3\x04\x00"), ".mp3"},
		{[]byte{0xff, 0xf1, 0x00, 0x00}, ".aac"},
		{[]byte{0xff, 0xfb, 0x90, 0x00}, ".mp3"},
		{append(append(bytes.Repeat([]byte{0}, 12), []byte("ftypM4A ")...), 0x00), ".m4a"},
		{[]byte("RIFFxxxxWAVE\x00\x00"), ".wav"},
		{[]byte("RIFFxxxxWEBM\x00\x00"), ".webm"},
		{[]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', '3', 'g', 'p', '5'}, ".m4a"},
		{[]byte{0x1a, 0x45, 0xdf, 0xa3, 0x01}, ".webm"},
		{[]byte("#!AMR\n"), ".amr"},
		{[]byte("caff\x00"), ".caf"},
		{[]byte("???"), ""},
		{[]byte(""), ""},
	}
	for _, tc := range cases {
		got := SniffVoiceExtension(tc.prefix)
		if got != tc.want {
			t.Fatalf("prefix %q: got %q want %q", tc.prefix, got, tc.want)
		}
	}
}

func TestFtypMajorBrand(t *testing.T) {
	p := []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', '3', 'g', 'p', '5'}
	if g := FtypMajorBrand(p); g != "3gp5" {
		t.Fatalf("got %q", g)
	}
	if g := FtypMajorBrand([]byte("no")); g != "" {
		t.Fatalf("want empty got %q", g)
	}
}
