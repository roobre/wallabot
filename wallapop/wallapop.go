package wallapop

import (
	"fmt"
	"github.com/google/go-querystring/query"
	"net/http"
)

const itemsURL = "https://es.wallapop.com/rest/items"

type Response struct {
	Items []*Item `json:"search_objects"`

	From int `json:"from"`
	To   int `json:"to"`
}

type Item struct {
	ID string `json:"id"`

	Title       string `json:"title"`
	Description string `json:"description"`

	Images []map[string]string `json:"images"` // TODO

	Price    int    `json:"price"`
	Currency string `json:"currency"`

	Slug string `json:"web_slug"`
}

type SearchArgs struct {
	Latitude  float64 `url:"latitude"`
	Longitude float64 `url:"longitude"`
	OrderBy   string  `url:"order_by"`
	Language  string  `url:"language"`
	Urgent    bool    `url:"urgent,omitempty"`
	Shipping  bool    `url:"shipping,omitempty"`
	Exchange  bool    `url:"exchange,omitempty"`
}

func Search(keywords string, args SearchArgs) []Item {
	return nil
}

func WpReq(params map[string]interface{}, cookies map[string]string) *http.Request {
	v, _ := query.Values(params)
	url := itemsURL + "?_p=1" + v.Encode()
	for k, v := range params {
		url += fmt.Sprintf("&%s=%v", k, v)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic("Error building request: " + err.Error())
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.113 Safari/537.36")

	for key, value := range cookies {
		req.AddCookie(&http.Cookie{
			Name:  key,
			Value: value,
		})
	}

	return req
}
