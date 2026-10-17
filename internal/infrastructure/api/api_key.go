package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
)

// GenerateAPIKey gera uma API key aleatória de 32 bytes (64 chars hex).
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// LoadOrCreateAPIKey carrega uma API key existente ou gera uma nova.
// A key é armazenada em data/api.key para persistência entre reinicializações.
func LoadOrCreateAPIKey(keyPath string) (string, error) {
	// Tenta carregar do arquivo
	if data, err := os.ReadFile(keyPath); err == nil {
		key := string(data)
		if key != "" {
			return key, nil
		}
	}

	// Gera uma nova key
	key, err := GenerateAPIKey()
	if err != nil {
		return "", err
	}

	// Salva no arquivo
	if err := os.MkdirAll("data", 0755); err != nil {
		return "", fmt.Errorf("failed to create data dir: %w", err)
	}
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		return "", fmt.Errorf("failed to save API key: %w", err)
	}

	fmt.Printf("Generated API key: %s\n", key)
	return key, nil
}