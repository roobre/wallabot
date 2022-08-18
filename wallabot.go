package wallabot

import (
	"fmt"
	"roob.re/wallabot/database"
	"roob.re/wallabot/metrics"
	"roob.re/wallabot/search"
	"roob.re/wallabot/telegram"
	"roob.re/wallabot/wallapop"
)

type Wallabot struct {
	c Config

	db *database.Database
	tg *telegram.Wallabot
	wp *wallapop.Client
	se *search.Searcher
	re *metrics.Reporter
}

type Config struct {
	DBPath               string
	Token                string
	MetricsListenAddress string
	telegram.WallabotConfig
}

func New(c Config) (*Wallabot, error) {
	w := &Wallabot{
		c: c,
	}
	var err error

	w.db, err = database.New(c.DBPath)
	if err != nil {
		return nil, fmt.Errorf("creating db: %w", err)
	}

	w.re = metrics.New()

	w.wp = wallapop.New()

	w.tg, err = telegram.NewWallabot(c.Token, w.db, w.wp, c.WallabotConfig)
	if err != nil {
		return nil, fmt.Errorf("creating bot: %w", err)
	}

	w.se = search.New(w.db, w.wp, w.tg.Notify)

	return w, nil
}

func (w *Wallabot) Start() error {
	eChan := make(chan error)

	go func() {
		eChan <- w.tg.Start()
	}()

	go func() {
		w.se.Start()
	}()

	go func() {
		w.re.Watch(w.db, w.tg, w.se)
		eChan <- w.re.ListenAndServe(w.c.MetricsListenAddress)
	}()

	return <-eChan
}
