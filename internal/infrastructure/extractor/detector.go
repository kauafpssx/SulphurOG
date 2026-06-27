package extractor

import (
	"os"
	"strings"
)

type Detector struct{}

func NewDetector() *Detector {
	return &Detector{}
}

func (d *Detector) DetectType(filePath string) string {
	// Ler magic bytes
	f, err := os.Open(filePath)
	if err != nil {
		return "unknown"
	}
	defer f.Close()

	header := make([]byte, 4)
	if _, err := f.Read(header); err != nil {
		return "unknown"
	}

	// ZIP: PK\x03\x04
	if header[0] == 'P' && header[1] == 'K' && header[2] == 0x03 && header[3] == 0x04 {
		return "zip"
	}

	// RAR: Rar!
	if header[0] == 'R' && header[1] == 'a' && header[2] == 'r' && header[3] == '!' {
		return "rar"
	}

	// 7z: 7z\xBC\xAF
	if header[0] == '7' && header[1] == 'z' && header[2] == 0xBC && header[3] == 0xAF {
		return "7z"
	}

	// GZ: \x1F\x8B
	if header[0] == 0x1F && header[1] == 0x8B {
		return "gz"
	}

	// TXT puro
	if isTextFile(header) {
		return "text"
	}

	return "unknown"
}

func isTextFile(header []byte) bool {
	for _, b := range header {
		if b == 0 {
			return false
		}
	}
	return true
}

// DetectByExtension detecta tipo por extensao
func DetectByExtension(filename string) string {
	lower := strings.ToLower(filename)

	if strings.HasSuffix(lower, ".zip") {
		return "zip"
	}
	if strings.HasSuffix(lower, ".rar") {
		return "rar"
	}
	if strings.HasSuffix(lower, ".7z") {
		return "7z"
	}
	if strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".tar.gz") {
		return "gz"
	}
	if strings.HasSuffix(lower, ".txt") {
		return "text"
	}

	return "unknown"
}
