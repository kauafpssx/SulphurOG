package parser

import (
	"bufio"
	"regexp"
	"strings"

	"github.com/sulphurog/sulphurog/internal/domain"
)

type StealerParser struct{}

func NewStealerParser() *StealerParser {
	return &StealerParser{}
}

func (p *StealerParser) ParsePasswords(content string) []domain.ULP {
	content = stripASCIIHeaders(content)

	// Detectar formato
	if strings.Contains(content, "url:") && strings.Contains(content, "password:") {
		return p.parseYAMLFormat(content)
	}
	if strings.Contains(content, "Host:") && strings.Contains(content, "Password:") {
		return p.parseKeyValueFormat(content)
	}
	if strings.Contains(content, "passwords:") && strings.Contains(content, "accounts:") {
		return p.parseYAMLFormat(content)
	}

	return p.parseSimpleFormat(content)
}

// Formato 1: YAML
// url: https://example.com
// username: user@email.com
// password: mypass
func (p *StealerParser) parseYAMLFormat(content string) []domain.ULP {
	var ulps []domain.ULP
	var currentURL, currentLogin, currentPassword string

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "url:") || strings.HasPrefix(line, "Url:") {
			currentURL = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "url:"), "Url:"))
		}
		if strings.HasPrefix(line, "username:") || strings.HasPrefix(line, "Username:") || strings.HasPrefix(line, "login:") || strings.HasPrefix(line, "Login:") {
			val := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "username:"), "Username:"), "login:"), "Login:")
			currentLogin = strings.TrimSpace(val)
		}
		if strings.HasPrefix(line, "password:") || strings.HasPrefix(line, "Password:") {
			val := strings.TrimPrefix(strings.TrimPrefix(line, "password:"), "Password:")
			currentPassword = strings.TrimSpace(val)

			if currentURL != "" && currentPassword != "" {
				login := currentLogin
				if login == "" {
					login = "_"
				}
				ulps = append(ulps, domain.ULP{URL: currentURL, Login: login, Password: currentPassword})
			}
			currentURL = ""
			currentLogin = ""
			currentPassword = ""
		}
	}
	return ulps
}

// Formato 2: Key-Value com separador -----
// Soft: Microsoft Edge (Default)
// Host: https://example.com
// Login: user@email.com
// Password: mypass
// -----
func (p *StealerParser) parseKeyValueFormat(content string) []domain.ULP {
	var ulps []domain.ULP
	var currentURL, currentLogin, currentPassword string

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Separador ----- ou linha vazia
		if strings.HasPrefix(line, "-----") || (line == "" && currentURL != "") {
			if currentURL != "" && currentPassword != "" {
				login := currentLogin
				if login == "" {
					login = "_"
				}
				ulps = append(ulps, domain.ULP{URL: currentURL, Login: login, Password: currentPassword})
			}
			currentURL = ""
			currentLogin = ""
			currentPassword = ""
			continue
		}

		if strings.HasPrefix(line, "Host:") || strings.HasPrefix(line, "url:") || strings.HasPrefix(line, "Url:") {
			val := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "Host:"), "url:"), "Url:"), " ")
			currentURL = strings.TrimSpace(val)
		}
		if strings.HasPrefix(line, "Login:") || strings.HasPrefix(line, "login:") || strings.HasPrefix(line, "username:") || strings.HasPrefix(line, "Username:") {
			val := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "Login:"), "login:"), "username:"), "Username:"), " ")
			currentLogin = strings.TrimSpace(val)
		}
		if strings.HasPrefix(line, "Password:") || strings.HasPrefix(line, "password:") {
			val := strings.TrimPrefix(strings.TrimPrefix(line, "Password:"), "password:")
			currentPassword = strings.TrimSpace(val)
		}
	}

	// Ultimo bloco
	if currentURL != "" && currentPassword != "" {
		login := currentLogin
		if login == "" {
			login = "_"
		}
		ulps = append(ulps, domain.ULP{URL: currentURL, Login: login, Password: currentPassword})
	}

	return ulps
}

func (p *StealerParser) parseSimpleFormat(content string) []domain.ULP {
	var ulps []domain.ULP
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || isASCIIArt(line) || isDecorativeLine(line) {
			continue
		}
		ulp, ok := parseULPLine(line)
		if ok {
			ulps = append(ulps, ulp)
		}
	}
	return ulps
}

func (p *StealerParser) ParseFile(filePath string) (*domain.ExtractedData, error) {
	return nil, nil
}

func (p *StealerParser) ParseCookies(content string) []domain.Cookie {
	return nil
}

func (p *StealerParser) IsULPFormat(content []byte) bool {
	s := string(content)
	// Pular se parecer cookie (tab-separated Netscape format)
	if strings.Contains(s, "\tTRUE\t") || strings.Contains(s, "\tFALSE\t") {
		return false
	}
	if (strings.Contains(s, "url:") || strings.Contains(s, "Host:")) && strings.Contains(s, "password:") {
		return true
	}
	return false
}

func parseULPLine(line string) (domain.ULP, bool) {
	// Netscape cookie format usa tabs
	if strings.Contains(line, "\t") {
		return domain.ULP{}, false
	}

	// Split em todos os ':' — last=pass, second-to-last=login, rest=URL
	// Isso trata corretamente https://host:port/path:user:pass
	parts := strings.Split(line, ":")
	if len(parts) < 3 {
		return domain.ULP{}, false
	}

	password := strings.TrimSpace(parts[len(parts)-1])
	login := strings.TrimSpace(parts[len(parts)-2])
	url := strings.TrimSpace(strings.Join(parts[:len(parts)-2], ":"))

	urlLower := strings.ToLower(url)
	if !strings.HasPrefix(urlLower, "http://") && !strings.HasPrefix(urlLower, "https://") {
		return domain.ULP{}, false
	}

	if login == "" || password == "" {
		return domain.ULP{}, false
	}

	for _, inv := range []string{"None", "none", "null", "N/A"} {
		if password == inv || login == inv {
			return domain.ULP{}, false
		}
	}

	return domain.ULP{URL: url, Login: login, Password: password}, true
}

var reURL = regexp.MustCompile(`(?i)^(?:url|host):\s*(.+)`)
var reUser = regexp.MustCompile(`(?i)^(?:username|login):\s*(.+)`)
var rePass = regexp.MustCompile(`(?i)^password:\s*(.+)`)

func stripASCIIHeaders(content string) string {
	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if isASCIIArt(line) || isDecorativeLine(line) || isOnlySymbols(line) {
			continue
		}
		result.WriteString(line)
		result.WriteString("\n")
	}
	return result.String()
}

func isASCIIArt(line string) bool {
	if len(line) == 0 {
		return false
	}
	specialCount := 0
	for _, r := range line {
		if r > 127 {
			specialCount++
		}
	}
	return float64(specialCount)/float64(len([]rune(line))) > 0.3
}

func isDecorativeLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) == 0 || len(trimmed) <= 3 {
		return false
	}
	decorative := "=-_~*#░╔╗╚╝═║╠╣╦╩┼─│┌┐└┘├┤┬┴"
	for _, r := range trimmed {
		if !strings.ContainsRune(decorative, r) && r != ' ' {
			return false
		}
	}
	return true
}

func isOnlySymbols(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) == 0 {
		return false
	}
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
