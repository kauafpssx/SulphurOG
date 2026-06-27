package extractor

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	zipPassword "github.com/alexmullins/zip"
)

type ZIPExtractor struct {
	detector *Detector
}

func NewZIPExtractor() *ZIPExtractor {
	return &ZIPExtractor{
		detector: NewDetector(),
	}
}

func (e *ZIPExtractor) DetectType(filePath string) string {
	return e.detector.DetectType(filePath)
}

// Extract extrai apenas arquivos necessarios do ZIP
func (e *ZIPExtractor) Extract(archivePath, password, destDir string) ([]string, error) {
	// Tentar sem senha primeiro
	files, err := e.extractNoPassword(archivePath, destDir)
	if err == nil {
		return files, nil
	}

	// Tentar com senha
	return e.extractWithPassword(archivePath, password, destDir)
}

func (e *ZIPExtractor) extractNoPassword(archivePath, destDir string) ([]string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return e.extractFiles(r.File, destDir)
}

func (e *ZIPExtractor) extractWithPassword(archivePath, password, destDir string) ([]string, error) {
	r, err := zipPassword.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var extracted []string
	for _, f := range r.File {
		f.SetPassword(password)

		// Extrair apenas arquivos necessarios
		if !isImportantFile(f.Name) {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		defer rc.Close()

		outPath := filepath.Join(destDir, filepath.Base(f.Name))
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			continue
		}

		outFile, err := os.Create(outPath)
		if err != nil {
			continue
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, rc); err != nil {
			continue
		}

		extracted = append(extracted, outPath)
	}

	return extracted, nil
}

func (e *ZIPExtractor) extractFiles(files []*zip.File, destDir string) ([]string, error) {
	var extracted []string
	for _, f := range files {
		if f.FileInfo().IsDir() {
			continue
		}

		// Extrair apenas arquivos necessarios
		if !isImportantFile(f.Name) {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			continue
		}
		defer rc.Close()

		outPath := filepath.Join(destDir, filepath.Base(f.Name))
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			continue
		}

		outFile, err := os.Create(outPath)
		if err != nil {
			continue
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, rc); err != nil {
			continue
		}

		extracted = append(extracted, outPath)
	}

	return extracted, nil
}

// ReadPasswordsFromZIP le passwords.txt do ZIP sem extrair tudo
func (e *ZIPExtractor) ReadPasswordsFromZIP(archivePath, password string) (string, error) {
	// Tentar com senha primeiro (a maioria dos ZIPs de stealer tem senha)
	if password != "" {
		content, err := e.readFromZIPWithPassword(archivePath, password, "passwords.txt")
		if err == nil {
			return content, nil
		}
	}

	// Tentar sem senha
	content, err := e.readFromZIPNoPassword(archivePath, "passwords.txt")
	if err == nil {
		return content, nil
	}

	return "", fmt.Errorf("passwords.txt not found")
}

func (e *ZIPExtractor) readFromZIPWithPassword(archivePath, password, targetFile string) (string, error) {
	r, err := zipPassword.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		// Procurar em qualquer subpasta
		if strings.Contains(strings.ToLower(f.Name), strings.ToLower(targetFile)) {
			f.SetPassword(password)

			rc, err := f.Open()
			if err != nil {
				continue
			}
			defer rc.Close()

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				continue
			}

			return buf.String(), nil
		}
	}

	return "", fmt.Errorf("%s not found", targetFile)
}

func (e *ZIPExtractor) readFromZIPNoPassword(archivePath, targetFile string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.Contains(strings.ToLower(f.Name), strings.ToLower(targetFile)) {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			defer rc.Close()

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				continue
			}

			return buf.String(), nil
		}
	}

	return "", fmt.Errorf("%s not found", targetFile)
}

// isImportantFile verifica se o arquivo e importante pra extrair
func isImportantFile(name string) bool {
	lower := strings.ToLower(name)
	important := []string{
		"passwords.txt",
		"cookies/",
		"information.txt",
		"creditcards/",
		"soft/discord/",
		"soft/steam/",
		"googleaccounts/",
	}

	for _, imp := range important {
		if strings.Contains(lower, imp) {
			return true
		}
	}

	return false
}
