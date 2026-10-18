package extractor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SevenZExtractor struct{}

func NewSevenZExtractor() *SevenZExtractor {
	return &SevenZExtractor{}
}

func (e *SevenZExtractor) Extract(archivePath, password, destDir string) ([]string, error) {
	// Detectar tipo
	ext := strings.ToLower(filepath.Ext(archivePath))

	switch ext {
	case ".rar":
		return e.extractRAR(archivePath, password, destDir)
	case ".7z":
		return e.extract7z(archivePath, password, destDir)
	default:
		return nil, fmt.Errorf("unsupported extension: %s", ext)
	}
}

// extractRAR extrai arquivos RAR usando unrar-free (preferencialmente) ou 7zz como fallback.
// O unrar-free suporta RAR5, enquanto o p7zip (7z) não suporta.
func (e *SevenZExtractor) extractRAR(archivePath, password, destDir string) ([]string, error) {
	// Tentar unrar-free primeiro (suporta RAR5 natively)
	if path, err := exec.LookPath("unrar"); err == nil {
		args := []string{"x", "-y"}
		if password != "" {
			args = append(args, "-p"+password)
		}
		args = append(args, "-o"+destDir, archivePath)

		cmd := exec.Command(path, args...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			return e.listFiles(destDir)
		}
		_ = output // unrar falhou, tenta 7zz abaixo
	}

	// Fallback: usar 7zz (7-Zip oficial, suporta RAR5) via Run7z/Find7z
	return e.extract7z(archivePath, password, destDir)
}

// extract7z extrai usando Run7z que utiliza Find7z() para localizar o binário correto
// (7z ou 7zz). O 7-Zip oficial (7zz) suporta RAR5, enquanto o p7zip (7z antigo) não.
func (e *SevenZExtractor) extract7z(archivePath, password, destDir string) ([]string, error) {
	args := []string{"x", "-y"}
	if password != "" {
		args = append(args, "-p"+password)
	}
	args = append(args, "-o"+destDir, archivePath)

	output, err := Run7z(args...)
	if err != nil {
		return nil, fmt.Errorf("7z extraction failed: %s\n%s", err.Error(), string(output))
	}

	return e.listFiles(destDir)
}

func (e *SevenZExtractor) listFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
