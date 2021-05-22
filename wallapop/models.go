package wallapop

type searchResponse struct {
	Items []Item `json:"search_objects"`

	From int `json:"from"`
	To   int `json:"to"`
}

type SearchArgs struct {
	Keywords  string  `url:"keywords"`
	Latitude  float64 `url:"latitude,omitempty"`
	Longitude float64 `url:"longitude,omitempty"`
	OrderBy   string  `url:"order_by,omitempty"`
	Language  string  `url:"language,omitempty"`
	Urgent    bool    `url:"urgent,omitempty"`
	Shipping  bool    `url:"shipping,omitempty"`
	Exchange  bool    `url:"exchange,omitempty"`
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
