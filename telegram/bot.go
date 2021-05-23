package telegram

import (
	"errors"
	"fmt"
	"gopkg.in/tucnak/telebot.v2"
	"log"
	"net/http"
	"roob.re/wallabot/wallapop"
	"time"
)

type Wallabot struct {
	bot *telebot.Bot
	wp  *wallapop.Client
}

type WallabotConfig struct {
	Token   string
	Verbose bool
	Timeout time.Duration
}

func (wc WallabotConfig) WithDefaults() (WallabotConfig, error) {
	if wc.Token == "" {
		return wc, errors.New("token must not be empty")
	}

	if wc.Timeout == 0 {
		wc.Timeout = 30 * time.Second
	}

	return wc, nil
}

func NewWallabot(c WallabotConfig) (*Wallabot, error) {
	c, err := c.WithDefaults()
	if err != nil {
		return nil, fmt.Errorf("error expanding config: %w", err)

	}

	bot, err := telebot.NewBot(telebot.Settings{
		Token:     c.Token,
		Verbose:   c.Verbose,
		Poller:    &telebot.LongPoller{Timeout: 10 * time.Second},
		ParseMode: telebot.ModeMarkdownV2,
		Reporter: func(err error) {
			log.Printf("telebot recovered from: %v", err)
		},
		Client: http.DefaultClient,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating bot: %w", err)
	}

	return (&Wallabot{
		bot: bot,
		wp:  wallapop.New(),
	}).withHandlers(), nil
}

func (wb *Wallabot) withHandlers() *Wallabot {
	wb.bot.Handle("/search", wb.HandleSearch)
	wb.bot.Handle(telebot.OnText, wb.HandleHelp)

	return wb
}

func (wb *Wallabot) Start() error {
	wb.bot.Start()
	return nil
}

func (wb *Wallabot) HandleSearch(m *telebot.Message) {
	const maxResults = 10

	keywords := m.Payload

	results, err := wb.wp.Search(wallapop.SearchArgs{
		Keywords: keywords,
	})
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Error processing your search: %v", err),
		))
	}

	if len(results) == 0 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Could not find any results for '%s'", keywords),
		))

		return
	}

	if len(results) > maxResults {
		results = results[:maxResults]
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("__Limiting search results to %d__\n", maxResults),
		))
	}
	for _, item := range results {
		//var msg interface{}
		//if len(item.Images) > 0 {
		//	msg = &telebot.Photo{
		//		File:      telebot.FromURL(item.Images[0].OriginalURL),
		//		Caption:   item.Markdown(),
		//	}
		//} else {
		//	msg = item.Markdown()
		//}
		sendLog(wb.bot.Reply(m, item.Markdown()))
	}
}

func (wb *Wallabot) HandleHelp(m *telebot.Message) {
	sendLog(wb.bot.Reply(m,
		"Oopsie woopsie, I did not get that command :(\n"+
			"Right now I support:\n"+
			"- /search <search query>",
		&telebot.SendOptions{
			ParseMode: telebot.ModeMarkdown,
		},
	))
}

func sendLog(m *telebot.Message, err error) *telebot.Message {
	if err != nil {
		log.Printf("error sending message: %v", err)
	}
	return m
}
