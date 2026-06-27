package main

import (
	"fmt"
	"strings"

	"github.com/sulphurog/sulphurog/internal/infrastructure/parser"
)

func main() {
	p := parser.NewStealerParser()

	// Formato 1: YAML
	yamlContent := `
url: https://www.ssnit.org.gh/
username: 
password: 

url: https://mobilife.enterprisegroup.net.gh/login
username: 0249597467
password: 5566

url: https://pdfsimpli.com/account/register
username: andrewkwarteng847@gmail.com
password: Service200
`

	// Formato 2: Key-Value com -----
	kvContent := `
Soft: Microsoft Edge (Default)
Host: https://www.tiktok.com/
Login: Private Link - t.me/+sKDTuIfs5HYyNzYx
Password: WPOmVbmNUi
-----
Soft: Microsoft Edge (Default)
Host: https://tlauncher.org/
Password: Lfvgsse7k6
-----
Soft: Microsoft Edge (Default)
Host: https://uwag.radiusbycampusmgmt.com/
Password: 9kLl63DNiu
-----
Soft: Microsoft Edge (Default)
Host: https://login.yahoo.com/
Login: Tg Channel - Admin-@logadm
Password: 5NHWP5bouI
`

	fmt.Println("=== Formato YAML ===")
	ulps1 := p.ParsePasswords(yamlContent)
	fmt.Printf("ULPs encontrados: %d\n", len(ulps1))
	for _, u := range ulps1 {
		fmt.Printf("  %s:%s:%s\n", u.URL, u.Login, u.Password)
	}

	fmt.Println("\n=== Formato Key-Value ===")
	ulps2 := p.ParsePasswords(kvContent)
	fmt.Printf("ULPs encontrados: %d\n", len(ulps2))
	for _, u := range ulps2 {
		fmt.Printf("  %s:%s:%s\n", u.URL, u.Login, u.Password)
	}

	// Testar deteccao de formato
	fmt.Println("\n=== Deteccao de Formato ===")
	fmt.Printf("YAML detected: %v\n", p.IsULPFormat([]byte(yamlContent)))
	fmt.Printf("KV detected: %v\n", p.IsULPFormat([]byte(kvContent)))

	// Testar com conteudo que tem Host: (mais um formato)
	anotherFormat := `
Soft: Google Chrome (Profile 1)
Host: https://www.facebook.com/
Login: user@email.com
Password: mypass123
`
	ulps3 := p.ParsePasswords(anotherFormat)
	fmt.Printf("\n=== Outro formato (Host:) ===\n")
	fmt.Printf("ULPs encontrados: %d\n", len(ulps3))
	for _, u := range ulps3 {
		fmt.Printf("  %s:%s:%s\n", u.URL, u.Login, u.Password)
	}

	_ = strings.TrimSpace("")
}
