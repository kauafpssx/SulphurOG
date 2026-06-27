package main

import (
	"fmt"
	zipPass "github.com/alexmullins/zip"
)

func main() {
	zipPath := `C:\Users\Administrator\Pictures\SulphurOG\data\temp\Echo Cloud - Logs - 26-06-2026.zip`

	r, err := zipPass.OpenReader(zipPath)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	defer r.Close()

	fmt.Printf("Total files: %d\n\n", len(r.File))

	// Mostrar primeiros 10 arquivos com flags
	count := 0
	for _, f := range r.File {
		if count >= 10 {
			break
		}
		fmt.Printf("Name: %s\n", f.Name)
		fmt.Printf("  CompressedSize: %d\n", f.CompressedSize64)
		fmt.Printf("  UncompressedSize: %d\n", f.UncompressedSize64)
		fmt.Printf("  Method: %d\n", f.Method)
		fmt.Printf("  Flags: %d (binary: %016b)\n", f.Flags, f.Flags)
		fmt.Printf("  IsEncrypted: %v\n", f.IsEncrypted())
		fmt.Printf("  IsZip64: %v\n", f.IsZip64)
		fmt.Println()
		count++
	}
}
