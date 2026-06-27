package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sulphurog/sulphurog/internal/infrastructure/extractor"
	"github.com/sulphurog/sulphurog/internal/infrastructure/parser"
)

func main() {
	zipPath := `data/temp/Echo Cloud - Logs - 26-06-2026.zip`
	password := "https://t.me/echocloudlink"
	extractDir := `data/temp/test_echo`

	os.MkdirAll(extractDir, 0755)
	defer os.RemoveAll(extractDir)

	fmt.Println("Extracting...")
	if err := extractor.ExtractToTemp(zipPath, password, extractDir); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Println("Done!")

	p := parser.NewStealerParser()
	totalULPs := 0
	filesScanned := 0

	filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
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
			rel, _ := filepath.Rel(extractDir, path)
			fmt.Printf("[%d] %s -> %d ULPs\n", filesScanned, rel, len(ulps))
		}
		return nil
	})

	fmt.Printf("\n=== %d files, %d ULPs ===\n", filesScanned, totalULPs)
}
