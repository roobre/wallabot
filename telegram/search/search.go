package search

import (
	"fmt"
	"strconv"
	"strings"

	"roob.re/wallabot/wallapop"
)

type Search struct {
	Keywords string
	MaxPrice int
	MinPrice int
	Strict   bool
	RadiusKm int
	NoZero   bool
}

const keyValueSeparator = "="

func New(raw string) (Search, error) {
	s := Search{}

	var keywords []string
	for _, field := range strings.Fields(strings.TrimSpace(raw)) {
		if !strings.Contains(field, keyValueSeparator) {
			keywords = append(keywords, field)
			continue
		}

		parts := strings.Split(field, keyValueSeparator)
		if len(parts) != 2 {
			keywords = append(keywords, parts...)
			continue
		}

		var err error
		key := strings.ToLower(parts[0])
		value := strings.ToLower(parts[1])
		switch key {
		case "max", "price":
			s.MaxPrice, err = strconv.Atoi(value)
			if err != nil {
				return s, fmt.Errorf("parsing price %q: %w", value, err)
			}

		case "min", "minprice":
			s.MinPrice, err = strconv.Atoi(value)
			if err != nil {
				return s, fmt.Errorf("parsing price %q: %w", value, err)
			}

		case "strict":
			s.Strict, err = strconv.ParseBool(value)
			if err != nil {
				return s, fmt.Errorf("parsing strict: %w", err)
			}

		case "nozero":
			s.NoZero, err = strconv.ParseBool(value)
			if err != nil {
				return s, fmt.Errorf("parsing nozero: %w", err)
			}

		case "radius":
			s.RadiusKm, err = strconv.Atoi(value)
			if err != nil {
				return s, fmt.Errorf("parsing radius: %w", err)
			}

		default:
			return s, fmt.Errorf("unknown key %s", key)
		}
	}

	s.Keywords = strings.Join(keywords, " ")

	return s, nil
}

func (s Search) Args() wallapop.SearchArgs {
	return wallapop.SearchArgs{
		Keywords: s.Keywords,
		MaxPrice: s.MaxPrice,
		MinPrice: s.MinPrice,
		RadiusM:  s.RadiusKm * 1000,
		Strict:   s.Strict,
		NoZero:   s.NoZero,
	}
}
