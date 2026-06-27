package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

func main() {
	zipPath := `data/temp/Echo Cloud - Logs - 26-06-2026.zip`
	password := "https://t.me/echocloudlink"
	filePath := `26-06-2026\[BD]0JG0K210S2_2026_06-26\Passwords.txt`

	// Teste 1: 7z e -so (extract without paths)
	fmt.Println("=== Teste 7z e -so ===")
	cmd1 := exec.Command("7-ZIP\\7z.exe", "e", "-p"+password, "-so", filePath, zipPath)
	var out1 bytes.Buffer
	cmd1.Stdout = &out1
	cmd1.Stderr = &out1
	err1 := cmd1.Run()
	fmt.Printf("Error: %v\n", err1)
	fmt.Printf("Output: %d bytes\n", out1.Len())
	if out1.Len() > 0 {
		fmt.Printf("Content: %s\n", out1.String()[:min(200, out1.Len())])
	}

	// Teste 2: 7z x -so (extract with paths)
	fmt.Println("\n=== Teste 7z x -so ===")
	cmd2 := exec.Command("7-ZIP\\7z.exe", "x", "-p"+password, "-so", filePath, zipPath)
	var out2 bytes.Buffer
	cmd2.Stdout = &out2
	cmd2.Stderr = &out2
	err2 := cmd2.Run()
	fmt.Printf("Error: %v\n", err2)
	fmt.Printf("Output: %d bytes\n", out2.Len())
	if out2.Len() > 0 {
		fmt.Printf("Content: %s\n", out2.String()[:min(200, out2.Len())])
	}

	// Teste 3: 7z e -so com aspas
	fmt.Println("\n=== Teste 7z e -so com path ===")
	cmd3 := exec.Command("7-ZIP\\7z.exe", "e", "-p"+password, "-so", zipPath, filePath)
	var out3 bytes.Buffer
	cmd3.Stdout = &out3
	cmd3.Stderr = &out3
	err3 := cmd3.Run()
	fmt.Printf("Error: %v\n", err3)
	fmt.Printf("Output: %d bytes\n", out3.Len())
	if out3.Len() > 0 {
		fmt.Printf("Content: %s\n", out3.String()[:min(200, out3.Len())])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
