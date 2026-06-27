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

func (e *SevenZExtractor) extractRAR(archivePath, password, destDir string) ([]string, error) {
	args := []string{"x"}

	if password != "" {
		args = append(args, "-p"+password)
	}

	args = append(args, "-o"+destDir, "-y", archivePath)

	cmd := exec.Command("unrar", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Tentar com 7z
		return e.extract7z(archivePath, password, destDir)
	}

	_ = output
	return e.listFiles(destDir)
}

func (e *SevenZExtractor) extract7z(archivePath, password, destDir string) ([]string, error) {
	args := []string{"x"}

	if password != "" {
		args = append(args, "-p"+password)
	}

	args = append(args, "-o"+destDir, "-y", archivePath)

	cmd := exec.Command("7z", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("7z extraction failed: %s", string(output))
	}

	_ = output
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
