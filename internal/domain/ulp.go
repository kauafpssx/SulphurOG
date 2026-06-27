package domain

import "fmt"

type ULP struct {
	URL      string `json:"url"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (u ULP) String() string {
	return fmt.Sprintf("%s:%s:%s", u.URL, u.Login, u.Password)
}
