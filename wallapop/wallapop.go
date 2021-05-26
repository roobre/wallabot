package wallapop

import (
	"encoding/json"
	"fmt"
	"net/http"
	wphttp "roob.re/wallabot/wallapop/http"
)

const nextPageHeader = "X-NextPage"
const searchPagesDefault = 8

type Client struct {
	http *wphttp.Client
}

func New() *Client {
	return &Client{
		http: wphttp.New(),
	}
}

func (sa SearchArgs) WithDefaults() SearchArgs {
	if sa.Pages == 0 {
		sa.Pages = searchPagesDefault
	}

	return sa
}

func (c *Client) Search(args SearchArgs) ([]Item, error) {
	const searchPath = "/general/search"

	args = args.WithDefaults()

	var items []Item
	var nextPageParams string

	for page := 0; page < args.Pages; page++ {
		url := searchPath + "?" + nextPageParams
		response, err := c.http.Request(url, http.MethodGet, args)
		if err != nil {
			return nil, fmt.Errorf("could not make http request: %w", err)
		}
		defer response.Body.Close()

		if response.StatusCode != 200 {
			return nil, fmt.Errorf("server responded with %d to %s", response.StatusCode, url)
		}

		sr := &searchResponse{}
		err = json.NewDecoder(response.Body).Decode(sr)
		if err != nil {
			return items, fmt.Errorf("decoding http response: %w", err)
		}

		if len(sr.Items) == 0 {
			break
		}

		items = append(items, sr.Items...)

		nextPageParams = response.Header.Get(nextPageHeader)
		if nextPageParams == "" {
			break
		}
	}

	return items, nil
}
