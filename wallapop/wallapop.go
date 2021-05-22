package wallapop

import (
	"encoding/json"
	"fmt"
	"net/http"
	wphttp "roob.re/wallabot/wallapop/http"
)

type Client struct {
	http *wphttp.Client
}

func New() *Client {
	return &Client{
		http: wphttp.New(),
	}
}

func (sa SearchArgs) WithDefaults() SearchArgs {
	return sa
}

func (c *Client) Search(args SearchArgs) ([]Item, error) {
	response, err := c.http.Request(searchUrl, http.MethodGet, args.WithDefaults())
	if err != nil {
		return nil, fmt.Errorf("could not make http request: %w", err)
	}

	sr := &searchResponse{}
	err = json.NewDecoder(response.Body).Decode(sr)
	if err != nil {
		return nil, err
	}

	return sr.Items, nil
}
