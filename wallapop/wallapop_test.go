package wallapop_test

import (
	"testing"

	"roob.re/wallabot/wallapop"
)

func TestClient_Search(t *testing.T) {
	c := wallapop.New()
	items, err := c.Search(wallapop.SearchArgs{Keywords: "nvidia"})
	if err != nil {
		t.Fatalf("search returned error: %v", err)
	}

	if len(items) <= 0 {
		t.Fatalf("search returned no items")
	}
}
