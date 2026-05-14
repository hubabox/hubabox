package librarychat

import (
	"path/filepath"
	"strings"
)

// VoiceContentTypeForFilename returns a MIME type suitable for <audio>/<source type="…">
// from a stored voice basename (e.g. "….m4a"). Unknown extensions default to octet-stream.
func VoiceContentTypeForFilename(basename string) string {
	return VoiceContentTypeForExt(strings.ToLower(filepath.Ext(basename)))
}

// VoiceContentTypeForExt expects a dotted extension in lowercase (e.g. ".m4a").
func VoiceContentTypeForExt(ext string) string {
	switch ext {
	case ".webm":
		return "audio/webm"
	case ".ogg", ".oga":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4" // may be overridden at serve time for 3GP-family brands; see VoiceContentTypeForM4AFile
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".caf":
		return "audio/x-caf"
	case ".amr":
		return "audio/amr"
	default:
		return ""
	}
}

// VoiceContentTypeForM4AFile picks an HTTP Content-Type for a file stored as .m4a by inspecting
// the leading bytes (ISO BMFF major brand). Many phones write 3GPP-style brands while still using a .m4a name.
func VoiceContentTypeForM4AFile(prefix []byte) string {
	brand := FtypMajorBrand(prefix)
	if brand == "" {
		return "audio/mp4"
	}
	if strings.HasPrefix(brand, "3gp") || strings.HasPrefix(brand, "3g2") ||
		strings.HasPrefix(brand, "3ge") || strings.HasPrefix(brand, "3gg") {
		return "audio/3gpp"
	}
	return "audio/mp4"
}
