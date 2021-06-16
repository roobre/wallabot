package database

import (
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
	Keywords  string
	MaxPrice  float64
	SentItems SentItems
}

// SentItems is a map of sent itemIDs and their price when they were sent the last time
type SentItems map[string]float64

func (ss SavedSearches) Get(keywords string) *SavedSearch {
	return ss[keywords]
}

func (ss SavedSearches) Set(search *SavedSearch) {
	if search.SentItems == nil {
		search.SentItems = SentItems{}
	}

	ss[search.Keywords] = search
}

func (ss SavedSearches) Delete(keywords string) bool {
	_, found := ss[keywords]
	if found {
		delete(ss, keywords)
	}

	return found
}

// Notification models a matching result for a search, which the user should be notified about
type Notification struct {
	User   *User
	Item   *wallapop.Item
	Search string
}
