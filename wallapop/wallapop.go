package wallapop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	wphttp "roob.re/wallabot/wallapop/http"
)

const searchPagesDefault = 8

var errEmptyPage = fmt.Errorf("search results empty")

type Client struct {
	http *wphttp.Client
}

func New() *Client {
	return &Client{
		http: wphttp.New(),
	}
}

func (sa SearchArgs) WithDefaults() SearchArgs {
	if sa.pages == 0 {
		sa.pages = searchPagesDefault
	}

	return sa
}

func (c *Client) Search(args SearchArgs) ([]Item, error) {
	args = args.WithDefaults()

	var items []Item

	var pageItems []Item
	var pageParams string // Returned by Wallapop API, collection of GET params that can be used to fetch the next page
	var err error

	for page := 0; page < args.pages; page++ {
		pageItems, pageParams, err = c.searchPage(args, pageParams)
		if err != nil && err != errEmptyPage {
			return items, err
		}

		for _, item := range pageItems {
			if args.Strict && !containsAny(item.Title, strings.Fields(args.Keywords)) {
				continue
			}

			if args.NoZero && item.Price == 0 {
				continue
			}

			items = append(items, item)
		}

		if err == errEmptyPage {
			break
		}
	}

	return items, nil
}

func (c *Client) searchPage(args SearchArgs, pageParams string) ([]Item, string, error) {
	const searchPath = "/general/search"
	const nextPageHeader = "X-NextPage"

	url := searchPath + "?" + pageParams
	response, err := c.http.Request(url, http.MethodGet, args)
	if err != nil {
		return nil, "", fmt.Errorf("could not make http request: %w", err)
	}
	defer func() {
		err := response.Body.Close()
		if err != nil {
			log.Warnf("error closing body: %v", err)
		}
	}()

	if response.StatusCode != 200 {
		return nil, "", fmt.Errorf("server responded with %d to %s", response.StatusCode, url)
	}

	sr := &searchResponse{}
	err = json.NewDecoder(response.Body).Decode(sr)
	if err != nil {
		return nil, "", fmt.Errorf("decoding http response: %w", err)
	}

	if len(sr.Items) == 0 {
		return nil, "", errEmptyPage
	}

	pageParams = response.Header.Get(nextPageHeader)

	return sr.Items, pageParams, nil
}

func containsAny(haystack string, needles []string) bool {
	haystack = strings.ToLower(haystack)
	for _, needle := range needles {
		if strings.Contains(haystack, strings.ToLower(needle)) {
			return true
		}
	}

	return false
}
