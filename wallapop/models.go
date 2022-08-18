package wallapop

import (
	"fmt"
	"regexp"
	"strings"
)

type searchResponse struct {
	Items []Item `json:"search_objects"`

	From int `json:"from"`
	To   int `json:"to"`
}

type SearchArgs struct {
	Keywords  string  `url:"keywords"`
	MinPrice  int     `url:"min_sale_price,omitempty"`
	MaxPrice  int     `url:"max_sale_price,omitempty"`
	RadiusM   int     `url:"distance,omitempty"`
	Latitude  float64 `url:"latitude,omitempty"`
	Longitude float64 `url:"longitude,omitempty"`
	OrderBy   string  `url:"order_by,omitempty"`
	Language  string  `url:"language,omitempty"`
	Urgent    bool    `url:"urgent,omitempty"`
	Shipping  bool    `url:"shipping,omitempty"`
	Exchange  bool    `url:"exchange,omitempty"`

	// Internal parameters, not passed down to Wallapop API
	Strict bool `url:"-"` // If true, wallabot will not filter out results not containing any of the keywords in the title.
	NoZero bool `url:"-"` // If true, wallabot will ignore results with a prize of 0€

	pages int `url:"-"`
}

type Item struct {
	ID string `json:"id"`

	Title       string `json:"title"`
	Description string `json:"description"`

	Images []ItemImage `json:"images"`

	Price    float64 `json:"price"`
	Currency string  `json:"currency"`

	Slug string `json:"web_slug"`
}

type ItemImage struct {
	OriginalURL string `json:"original"`
}

// Special characters in markdown
var mdSpecial = regexp.MustCompile("[\\[\\]()~`>#+\\-=|{}.!_*\\\\]")

func markdownEscape(source string) string {
	return mdSpecial.ReplaceAllString(source, `\$0`)
}

func replaceCurrency(source string) string {
	return strings.NewReplacer("EUR", "€", "USD", "$").Replace(source)
}

func (i *Item) Markdown() string {
	const wpLinkBase = "https://es.wallapop.com/item"
	return fmt.Sprintf(
		"*%s*\n"+
			"*%d%s*\n"+
			//"%.80s\\.\\.\\.\n"+
			"%s/%s",
		markdownEscape(i.Title), int(i.Price), replaceCurrency(i.Currency),
		//markdownEscape(i.Description),
		markdownEscape(wpLinkBase), markdownEscape(i.Slug),
	)
}
