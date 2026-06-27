package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sulphurog/sulphurog/internal/infrastructure/parser"
)

func main() {
	zipPath := `C:\Users\Administrator\Pictures\SulphurOG\data\temp\Echo Cloud - Logs - 26-06-2026.zip`
	password := "https://t.me/echocloudlink"
	extractDir := `C:\Users\Administrator\Pictures\SulphurOG\data\temp\extracted_test`

	os.MkdirAll(extractDir, 0755)
	defer os.RemoveAll(extractDir)

	// Extrair com 7z
	fmt.Println("Extraindo com 7z...")
	args := []string{"x", "-p" + password, "-o" + extractDir, "-y", zipPath}
	cmd := exec.Command("7z", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("7z ERROR: %v\n%s\n", err, string(output))
		return
	}
	fmt.Println("Extraido com sucesso!")

	p := parser.NewStealerParser()
	totalULPs := 0
	filesScanned := 0

	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(info.Name()), "passwords.txt") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		ulps := p.ParsePasswords(string(content))
		if len(ulps) > 0 {
			filesScanned++
			totalULPs += len(ulps)
			fmt.Printf("[%d] %s -> %d ULPs\n", filesScanned, info.Name, len(ulps))
			for i, u := range ulps {
				if i < 2 {
					fmt.Printf("  %s:%s:%s\n", u.URL, u.Login, u.Password)
				}
			}
			if len(ulps) > 2 {
				fmt.Printf("  ... +%d more\n", len(ulps)-2)
			}
		}
		return nil
	})

	fmt.Printf("\n=== %d arquivos, %d ULPs total ===\n", filesScanned, totalULPs)
}
