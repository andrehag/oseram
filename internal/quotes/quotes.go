package quotes

import (
	"crypto/rand"
	"embed"
	"encoding/json"
	"fmt"
	"math/big"
)

//go:embed quotes.json
var quoteFS embed.FS

type Quote struct {
	Attribution string `json:"attribution"`
	Quote       string `json:"quote"`
}

func Random() (Quote, error) {
	data, err := quoteFS.ReadFile("quotes.json")
	if err != nil {
		return Quote{}, err
	}
	var quotes []Quote
	if err := json.Unmarshal(data, &quotes); err != nil {
		return Quote{}, err
	}
	if len(quotes) == 0 {
		return Quote{}, fmt.Errorf("no quotes available")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(quotes))))
	if err != nil {
		return Quote{}, err
	}
	return quotes[n.Int64()], nil
}

func Format(q Quote) string {
	return fmt.Sprintf("\n\"%s\"\n— %s", q.Quote, q.Attribution)
}
