package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func main() {
	zipPath := `C:\Users\Administrator\Pictures\SulphurOG\data\temp\Echo Cloud - Logs - 26-06-2026.zip`
	password := "https://t.me/echocloudlink"

	// Teste 1: 7z l para ver os paths
	fmt.Println("=== Teste 7z l ===")
	listCmd := exec.Command("7z", "l", "-ba", "-p"+password, zipPath)
	output, _ := listCmd.Output()
	lines := strings.Split(string(output), "\n")

	// Encontrar primeiro passwords.txt
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "passwords.txt") {
			fmt.Printf("Found: %s\n", line)
			parts := strings.Fields(line)
			if len(parts) > 0 {
				filePath := parts[len(parts)-1]
				fmt.Printf("Path: %s\n", filePath)

				// Teste 2: 7z x -so com o path exato
				fmt.Printf("\n=== Teste 7z x -so %s ===\n", filePath)
				extractCmd := exec.Command("7z", "x", "-p"+password, "-so", filePath, zipPath)
				var out bytes.Buffer
				extractCmd.Stdout = &out
				extractCmd.Stderr = &out
				err := extractCmd.Run()
				fmt.Printf("Error: %v\n", err)
				fmt.Printf("Output length: %d bytes\n", out.Len())
				if out.Len() > 0 {
					content := out.String()
					if len(content) > 500 {
						fmt.Printf("First 500 chars: %s...\n", content[:500])
					} else {
						fmt.Printf("Content: %s\n", content)
					}
				}
			}
			break
		}
	}
}
