package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"roob.re/wallabot/wallapop"
)

type Database struct {
	bdg *badger.DB
}

const userKeyPrefix = "user_"

type User struct {
	ID       int
	Name     string
	ChatID   int64
	Lat      float64
	Long     float64
	RadiusKm int
	Searches SavedSearches
}

type SavedSearches map[string]*SavedSearch

type SavedSearch struct {
	Keywords  string
	MaxPrice  float64
	SentItems SentItems
}

// SentItems is a map of sent itemIDs and their price
type SentItems map[string]float64

type Notification struct {
	User   *User
	Item   *wallapop.Item
	Search string
}

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

func New(path string) (*Database, error) {
	bdg, err := badger.Open(badger.DefaultOptions(path))
	if err != nil {
		return nil, err
	}

	return &Database{
		bdg: bdg,
	}, nil
}

func (db *Database) User(id int, f func(u *User) error) error {
	idb := userKey(id)
	return db.bdg.View(func(txn *badger.Txn) error {
		user, err := db.getUser(idb, txn.Get)
		if err != nil {
			return err
		}

		err = f(user)
		if err != nil {
			return fmt.Errorf("user function: %w", err)
		}
		return nil
	})
}

func (db *Database) UserUpdate(id int, f func(u *User) error) error {
	idb := userKey(id)
	return db.bdg.Update(func(txn *badger.Txn) error {
		user, err := db.getUser(idb, txn.Get)
		if err != nil {
			return err
		}

		err = f(user)
		if err != nil {
			return fmt.Errorf("user function: %w", err)
		}

		return db.putUser(user, txn)
	})
}

func (db *Database) UserEach(f func(u *User) error) error {
	return db.bdg.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchSize:   64,
			PrefetchValues: true,
			Prefix:         []byte(userKeyPrefix),
		})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			user, err := db.getUser(nil, func(bytes []byte) (*badger.Item, error) {
				return it.Item(), nil
			})
			if err != nil {
				return err
			}

			err = f(user)
			if err != nil {
				return fmt.Errorf("user function: %w", err)
			}
		}

		return nil
	})
}

func (db *Database) getUser(idb []byte, getter func([]byte) (*badger.Item, error)) (*User, error) {
	user := &User{}
	item, err := getter(idb)
	if err != nil {
		return nil, fmt.Errorf("getting user from DB: %w", err)
	}

	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, user)
	})
	if err != nil {
		return nil, fmt.Errorf("unmarshalling user from DB: %w", err)
	}

	return user, nil
}

func (db *Database) putUser(user *User, txn *badger.Txn) error {
	if user.ID == 0 {
		return fmt.Errorf("refusing to store user with id 0")
	}

	idb := userKey(user.ID)

	userJson, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("marshalling user into json: %w", err)
	}

	return txn.Set(idb, userJson)
}

func (db *Database) AssertUser(u *User) error {
	if u.ID == 0 {
		return fmt.Errorf("cannot assert user with ID 0")
	}

	idb := userKey(u.ID)

	existingUser := &User{}
	err := db.bdg.View(func(txn *badger.Txn) error {
		item, err := txn.Get(idb)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, existingUser)
		})
	})
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return err
	}

	if u.ID == existingUser.ID &&
		u.ChatID == existingUser.ChatID &&
		u.Name == existingUser.Name {
		return nil
	}

	err = db.bdg.Update(func(txn *badger.Txn) error {
		encodedUser, err := json.Marshal(u)
		if err != nil {
			return err
		}
		return txn.Set(idb, encodedUser)
	})
	if err != nil {
		return err
	}

	return nil
}

func userKey(id int) []byte {
	return []byte(userKeyPrefix + fmt.Sprint(id))
}
