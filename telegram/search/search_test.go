package search_test

import (
	"testing"

	"roob.re/wallabot/telegram/search"
)

func TestNew(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		expected search.Search
	}{
		{
			raw: "a simple search",
			expected: search.Search{
				Keywords: "a simple search",
			},
		},
		{
			raw: "search with price=100 price",
			expected: search.Search{
				Keywords: "search with price",
				MaxPrice: 100,
			},
		},
		{
			raw: "search with radius radius=10",
			expected: search.Search{
				Keywords: "search with radius",
				RadiusKm: 10,
			},
		},
		{
			raw: "strict=true strict search",
			expected: search.Search{
				Keywords: "strict search",
				Strict:   true,
			},
		},
		{
			raw: "strict=true radius=3 all the nozero=true things price=200 search",
			expected: search.Search{
				Keywords: "all the things search",
				Strict:   true,
				RadiusKm: 3,
				MaxPrice: 200,
				NoZero:   true,
			},
		},
	} {
		actual, err := search.New(tc.raw)
		if err != nil {
			t.Fatal(err)
		}

		if tc.expected != actual {
			t.Fatalf("Search %q does not match {%v}", tc.raw, tc.expected)
		}
	}
}
