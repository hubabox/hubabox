package librarychat

import "bytes"

// SniffPrefixBytes is how many leading bytes we inspect to guess the on-disk extension.
const SniffPrefixBytes = 512

// SniffVoiceExtension returns a dotted extension (e.g. ".m4a") when prefix matches a known
// audio container, or "" when unknown. Callers may combine this with multipart metadata.
func SniffVoiceExtension(prefix []byte) string {
	if len(prefix) < 4 {
		return ""
	}
	switch {
	case bytes.HasPrefix(prefix, []byte("fLaC")):
		return ".flac"
	case bytes.HasPrefix(prefix, []byte("OggS")):
		return ".ogg"
	case bytes.HasPrefix(prefix, []byte("ID3")):
		return ".mp3"
	case bytes.HasPrefix(prefix, []byte("caff")):
		return ".caf"
	case len(prefix) >= 4 && prefix[0] == 0x1a && prefix[1] == 0x45 && prefix[2] == 0xdf && prefix[3] == 0xa3:
		return ".webm"
	case len(prefix) >= 12 && bytes.HasPrefix(prefix, []byte("RIFF")):
		if bytes.Equal(prefix[8:12], []byte("WAVE")) {
			return ".wav"
		}
		if bytes.Equal(prefix[8:12], []byte("WEBM")) {
			return ".webm"
		}
	case sniffISOFTYPAligned(prefix):
		// ISO BMFF (M4A/AAC, MP4 audio, many phone exports). ftyp is usually at offset 4 but not always.
		return ".m4a"
	case isAACADTS(prefix):
		// Raw AAC (common from Android “Sound recorder” / share sheets); must come before MP3 sync check.
		return ".aac"
	case len(prefix) >= 2 && prefix[0] == 0xff && prefix[1]&0xe0 == 0xe0:
		// MPEG-1/2 Layer III
		return ".mp3"
	case len(prefix) >= 5 && bytes.HasPrefix(prefix, []byte("#!AMR\n")):
		return ".amr"
	}
	return ""
}

// sniffISOFTYPAligned looks for an 'ftyp' box type on 4-byte-aligned offsets (common MP4/M4A layout).
func sniffISOFTYPAligned(prefix []byte) bool {
	n := len(prefix)
	if n < 12 {
		return false
	}
	scan := n
	if scan > 128 {
		scan = 128
	}
	for off := 4; off+8 <= scan; off += 4 {
		if bytes.Equal(prefix[off:off+4], []byte("ftyp")) {
			return true
		}
	}
	return false
}

// FtypMajorBrand returns the ISO BMFF major brand (4 bytes, often space-padded per spec)
// when an 'ftyp' box is found on 4-byte-aligned offsets within the first 128 bytes, else "".
func FtypMajorBrand(prefix []byte) string {
	n := len(prefix)
	if n < 12 {
		return ""
	}
	scan := n
	if scan > 128 {
		scan = 128
	}
	for off := 4; off+8 <= scan; off += 4 {
		if bytes.Equal(prefix[off:off+4], []byte("ftyp")) {
			if off+8 <= len(prefix) {
				return string(prefix[off+4 : off+8])
			}
			return ""
		}
	}
	return ""
}

// isAACADTS detects MPEG-2/4 ADTS (raw AAC) framing: 12-bit sync 0xfff and layer field 00.
// MP3 layer III uses layer bits 01, so (prefix[1] & 0x06) != 0 for typical 0xfb frames.
func isAACADTS(prefix []byte) bool {
	if len(prefix) < 2 {
		return false
	}
	if prefix[0] != 0xff {
		return false
	}
	if prefix[1]&0xf0 != 0xf0 {
		return false
	}
	return (prefix[1] & 0x06) == 0
}
