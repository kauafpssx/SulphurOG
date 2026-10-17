// Package localstorage stores processed artifacts on the host filesystem.
package localstorage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Storage struct {
	root string
}

func New(root string) (*Storage, error) {
	if root == "" {
		return nil, fmt.Errorf("local storage directory is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve storage directory: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0755); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}
	return &Storage{root: absRoot}, nil
}

// Root retorna o diretório raiz do storage.
func (s *Storage) Root() string {
	return s.root
}

// ListFiles retorna todos os arquivos (caminhos relativos) abaixo da raiz.
func (s *Storage) ListFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}

// ReadFile lê um arquivo relativo do storage.
func (s *Storage) ReadFile(relPath string) ([]byte, error) {
	clean := filepath.Clean(relPath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("invalid file path: %q", relPath)
	}
	path := filepath.Join(s.root, clean)
	return os.ReadFile(path)
}

// WriteFile salva um arquivo relativo no storage (sobrescreve).
func (s *Storage) WriteFile(relPath string, data []byte) error {
	clean := filepath.Clean(relPath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid file path: %q", relPath)
	}
	path := filepath.Join(s.root, clean)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// DeleteFile remove um arquivo relativo do storage.
func (s *Storage) DeleteFile(relPath string) error {
	clean := filepath.Clean(relPath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid file path: %q", relPath)
	}
	path := filepath.Join(s.root, clean)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove file: %w", err)
	}
	return nil
}

// FileExists verifica se um arquivo relativo existe no storage.
func (s *Storage) FileExists(relPath string) (bool, error) {
	clean := filepath.Clean(relPath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("invalid file path: %q", relPath)
	}
	path := filepath.Join(s.root, clean)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Write atomically saves data below the configured root. Paths must be relative
// and cannot escape that root.
func (s *Storage) Write(name string, data []byte) (string, error) {
	clean := filepath.Clean(name)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid artifact path: %q", name)
	}
	path := filepath.Join(s.root, clean)
	rel, err := filepath.Rel(s.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact path escapes storage root: %q", name)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("create artifact directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".artifact-*")
	if err != nil {
		return "", fmt.Errorf("create temporary artifact: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close artifact: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("publish artifact: %w", err)
	}
	return path, nil
}
