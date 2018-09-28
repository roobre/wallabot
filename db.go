package main

import (
	"encoding/json"
	"github.com/pkg/errors"
	"log"
	"os"
	"sync"
)

type Database interface {
	// Called to commit memory changes to disk
	Save() error

	// Mutex methods
	sync.Locker
	RLock()
	RUnlock()

	Users() map[int64]*user
	Config() *config
}

type user struct {
	Location string
	Searches map[string]map[string]interface{}
	Sent     map[uint64]uint
}

func (u *user) Copy() *user {
	if u == nil {
		return nil
	}

	searches := make(map[string]map[string]interface{})
	for k, v := range u.Searches {
		values := make(map[string]interface{})
		for l, w := range v {
			values[l] = w
		}
		searches[k] = values
	}
	// TODO: Copy sent too, for now is not needed
	return &user{Location: u.Location, Searches: searches}
}

type config struct {
	searchesPerUser uint
	maxUsers        uint
	lastUpdate      int
}

type jsondb struct {
	sync.RWMutex `json:"-"`
	file         *os.File

	UsersData  map[int64]*user `json:"users"`
	ConfigData *config         `json:"config"`
	//SentData   map[string]string `json:"sent"`
}

func (db *jsondb) Save() error {
	db.RLock()
	defer db.RUnlock()

	var err error

	db.file.Seek(0, 0)
	db.file.Truncate(0)

	encoder := json.NewEncoder(db.file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(db)
	if err != nil {
		log.Println("Could write file back to disk: " + err.Error())
	}

	return nil
}

func (db *jsondb) Users() map[int64]*user {
	return db.UsersData
}
func (db *jsondb) Config() *config {
	return db.ConfigData
}

func openJsonDB(file string) (Database, error) {
	dbfile, err := os.OpenFile(file, os.O_RDWR, 0640)
	if err != nil {
		return nil, errors.New("Could not open db.json: " + err.Error())
	}

	db := &jsondb{file: dbfile}
	db.UsersData = make(map[int64]*user)
	db.ConfigData = &config{}

	err = json.NewDecoder(dbfile).Decode(&db)
	if err != nil {
		return nil, errors.New("Could not decode json database: " + err.Error())
	}

	return db, nil
}
