package extractor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func Find7z() string {
	if path := os.Getenv("SEVENZ_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	if runtime.GOOS == "windows" {
		// Pasta 7-ZIP local no projeto (Windows)
		localPath := filepath.Join("7-ZIP", "7z.exe")
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
		if path, err := exec.LookPath("7z.exe"); err == nil {
			return path
		}
		return ""
	}

	// Linux/Mac: preferir 7z, fallback pra 7zz (pacote 7zip no Ubuntu/Debian 12+)
	for _, candidate := range []string{"7z", "7zz"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	for _, p := range []string{
		"/usr/bin/7z", "/usr/local/bin/7z", "/snap/bin/7z",
		"/usr/bin/7zz", "/usr/local/bin/7zz",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

func Run7z(args ...string) ([]byte, error) {
	bin := Find7z()
	if bin == "" {
		return nil, fmt.Errorf("7z not found")
	}

	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("7z error: %w\n%s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

func ListFiles(archivePath, password string) ([]string, error) {
	args := []string{"l", "-ba"}
	if password != "" {
		args = append(args, "-p"+password)
	}
	args = append(args, archivePath)

	output, err := Run7z(args...)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) > 0 {
			path := parts[len(parts)-1]
			if !strings.HasSuffix(path, "/") {
				files = append(files, path)
			}
		}
	}
	return files, nil
}

func ExtractToTemp(archivePath, password, tempDir string) error {
	args := []string{"x", "-y"}
	if password != "" {
		args = append(args, "-p"+password)
	}
	args = append(args, "-o"+tempDir, archivePath)
	_, err := Run7z(args...)
	return err
}
