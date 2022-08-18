package database

import (
	"fmt"
	"strings"

	"roob.re/wallabot/telegram/search"
	"roob.re/wallabot/wallapop"
)

const (
	defaultLat  = 41.383333
	defaultLong = 2.183333
)

type User struct {
	ID       int
	Name     string
	ChatID   int64
	Lat      float64
	Long     float64
	RadiusKm int
	Searches SavedSearches
}

func (u *User) Location() (float64, float64) {
	if u.Lat != 0 && u.Long != 0 {
		return u.Lat, u.Long
	}

	return defaultLat, defaultLong
}

type SavedSearches map[string]*SavedSearch

type SavedSearch struct {
	Search    search.Search
	Muted     bool
	SentItems SentItems
	Keywords  string  // Deprecated
	RadiusKm  int     // Deprecated
	MinPrice  float64 // Deprecated
	MaxPrice  float64 // Deprecated
}

func (ss *SavedSearch) LegacyFill() {
	if ss.RadiusKm != 0 {
		ss.Search.RadiusKm = ss.RadiusKm
	}

	if ss.MaxPrice != 0 {
		ss.Search.MaxPrice = int(ss.MaxPrice)
	}
	if ss.MinPrice != 0 {
		ss.Search.MinPrice = int(ss.MinPrice)
	}

	if ss.Keywords != "" {
		ss.Search.Keywords = ss.Keywords
	}
}

func (ss SavedSearch) Emojify() string {
	str := &strings.Builder{}

	fmt.Fprintf(str, "- `%s` | %d 🔔", ss.Search.Keywords, len(ss.SentItems))
	fmt.Fprintf(str, "| <= %d€", ss.Search.MaxPrice)
	if ss.Search.RadiusKm != 0 {
		fmt.Fprintf(str, " | 📍 %dKm", ss.Search.RadiusKm)
	}

	if ss.Search.Strict {
		fmt.Fprintf(str, " | 🔬 Strict")
	}

	if ss.Search.NoZero {
		fmt.Fprintf(str, " | ⛔ No zero")
	}

	return str.String()
}

// SentItems is a map of sent itemIDs and their price when they were sent the last time
type SentItems map[string]float64

// Get returns a SavedSearch given the canonical string representation of search.Search
func (ss SavedSearches) Get(keywords string) *SavedSearch {
	return ss[keywords]
}

func (ss SavedSearches) Set(search *SavedSearch) {
	if search.SentItems == nil {
		search.SentItems = SentItems{}
	}

	ss[search.Search.Keywords] = search
}

func (ss SavedSearches) Delete(keywords string) bool {
	if _, found := ss[keywords]; found {
		delete(ss, keywords)
		return true
	}

	return false
}

// Notification models a matching result for a search, which the user should be notified about
type Notification struct {
	User   *User
	Item   *wallapop.Item
	Search string
}
