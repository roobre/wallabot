package database

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger/v3"
)

type Database struct {
	bdg *badger.DB
}

type User struct {
	ID       int
	Name     string
	Searches SavedSearches
}

type SavedSearches map[string]*SavedSearch

type SavedSearch struct {
	Keywords  string
	MaxPrice  float64
	SentItems SentItems
}

type SentItems map[ItemID]ItemPrice
type ItemID string
type ItemPrice float64

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
	err := db.createUserIfMissing(id)
	if err != nil {
		return fmt.Errorf("asserting user existence: %w", err)
	}

	idb, err := idToBytes(id)
	if err != nil {
		return err
	}

	return db.bdg.View(func(txn *badger.Txn) error {
		user, err := db.getUser(idb, txn)
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
	err := db.createUserIfMissing(id)
	if err != nil {
		return fmt.Errorf("asserting user existence: %w", err)
	}

	idb, err := idToBytes(id)
	if err != nil {
		return err
	}

	return db.bdg.Update(func(txn *badger.Txn) error {
		user, err := db.getUser(idb, txn)
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

func (db *Database) getUser(idb []byte, txn *badger.Txn) (*User, error) {
	user := &User{}
	item, err := txn.Get(idb)
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

	idb, err := idToBytes(user.ID)
	if err != nil {
		return fmt.Errorf("converting id to bytes: %w", err)
	}

	userJson, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("marshalling user into json: %w", err)
	}

	return txn.Set(idb, userJson)
}

func (db *Database) createUserIfMissing(userId int) error {
	var exists bool
	idb, err := idToBytes(userId)
	if err != nil {
		return err
	}

	err = db.bdg.View(func(txn *badger.Txn) error {
		_, err := txn.Get(idb)
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		} else if err == nil {
			exists = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	err = db.bdg.Update(func(txn *badger.Txn) error {
		user := &User{
			ID:       userId,
			Name:     "",
			Searches: SavedSearches{},
		}
		encodedUser, err := json.Marshal(user)
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

func idToBytes(id int) ([]byte, error) {
	byteID := &bytes.Buffer{}
	err := binary.Write(byteID, binary.LittleEndian, int64(id))
	if err != nil {
		return nil, fmt.Errorf("converting uid to []byte: %w", err)
	}

	return byteID.Bytes(), nil
}
