package domain

type Cookie struct {
	Domain  string `json:"domain"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Path    string `json:"path"`
	Secure  bool   `json:"secure"`
	Expires int64  `json:"expires"`
}

type ExtractedData struct {
	LogFile       LogFile  `json:"log_file"`
	ULPs          []ULP    `json:"ulps"`
	Passwords     string   `json:"passwords"`
	Cookies       []Cookie `json:"cookies"`
	Information   string   `json:"information"`
	GoogleTokens  string   `json:"google_tokens"`
	CreditCards   string   `json:"credit_cards"`
	DiscordTokens string   `json:"discord_tokens"`
	SteamTokens   string   `json:"steam_tokens"`
}
