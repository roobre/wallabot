package wallabot

import (
	"fmt"
	"roob.re/wallabot/database"
	"roob.re/wallabot/search"
	"roob.re/wallabot/telegram"
	"roob.re/wallabot/wallapop"
	"sync"
)

type Wallabot struct {
	db *database.Database
	tg *telegram.Wallabot
	wp *wallapop.Client
	se *search.Searcher
}

type Config struct {
	DBPath string
	Token string
	telegram.WallabotConfig
}

func New(c Config) (*Wallabot, error) {
	w := &Wallabot{}
	var err error

	w.db, err = database.New(c.DBPath)
	if err != nil {
		return nil, fmt.Errorf("creating db: %w", err)
	}

	w.wp = wallapop.New()

	w.tg, err = telegram.NewWallabot(c.Token, w.db, w.wp, c.WallabotConfig)
	if err != nil {
		return nil, fmt.Errorf("creating bot: %w", err)
	}

	w.se = search.New(w.db, w.wp, w.tg.Notify)

	return w, nil
}

func (w *Wallabot) Start() error {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		_ = w.tg.Start()
		wg.Done()
	}()
	go func() {
		w.se.Start()
		wg.Done()
	}()

	wg.Wait()
	return nil
}
