package telegram

import (
	"errors"
	"fmt"
	"gopkg.in/tucnak/telebot.v2"
	"log"
	"net/http"
	"roob.re/wallabot/database"
	"roob.re/wallabot/wallapop"
	"strconv"
	"strings"
	"time"
)

type Wallabot struct {
	bot *telebot.Bot
	wp  *wallapop.Client
	db  *database.Database
}

type WallabotConfig struct {
	Token   string
	Verbose bool
	Timeout time.Duration
	DBPath  string
}

func (wc WallabotConfig) WithDefaults() (WallabotConfig, error) {
	if wc.Token == "" {
		return wc, errors.New("token must not be empty")
	}

	if wc.Timeout == 0 {
		wc.Timeout = 30 * time.Second
	}

	if wc.DBPath == "" {
		wc.DBPath = "./data"
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
		ParseMode: telebot.ModeMarkdown,
		Reporter: func(err error) {
			log.Printf("telebot recovered from: %v", err)
		},
		Client: http.DefaultClient,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating bot: %w", err)
	}

	db, err := database.New(c.DBPath)
	if err != nil {
		return nil, fmt.Errorf("creating db: %w", err)
	}

	return (&Wallabot{
		bot: bot,
		wp:  wallapop.New(),
		db:  db,
	}).withHandlers(), nil
}

func (wb *Wallabot) withHandlers() *Wallabot {
	wb.bot.Handle("/search", wb.withUser(wb.HandleSearch))
	wb.bot.Handle("/list", wb.withUser(wb.HandleSavedSearches))
	wb.bot.Handle("/new", wb.withUser(wb.HandleNewSearch))
	wb.bot.Handle("/delete", wb.withUser(wb.HandleDeleteSearch))
	wb.bot.Handle(telebot.OnText, wb.HandleHelp)

	_ = wb.bot.SetCommands([]telebot.Command{
		{
			Text:        "/search",
			Description: "Immediately search for items",
		},
		{
			Text:        "/list",
			Description: "See my saved searches",
		},
		{
			Text:        "/new",
			Description: "Create a new saved search",
		},
		{
			Text:        "/delete",
			Description: "Delete a saved search",
		},
	})

	return wb
}

func (wb *Wallabot) Start() error {
	wb.bot.Start()
	return nil
}

func (wb *Wallabot) HandleSearch(m *telebot.Message) {
	const maxResults = 10

	keywords := strings.TrimSpace(m.Payload)
	if len(keywords) == 0 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("`Usage: %s [keywords...]`", "/search"),
		))
		return
	}

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
		sendLog(wb.bot.Reply(m, item.Markdown(), &telebot.SendOptions{
			ParseMode: telebot.ModeMarkdownV2,
		}))
	}
}

func (wb *Wallabot) HandleNewSearch(m *telebot.Message) {
	parts := strings.Split(m.Payload, " ")
	if len(parts) < 2 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("`Usage: %s <max price> [keywords...]`", "/new"),
		))
		return
	}

	// If user uses , as a decimal separator, replace it
	if strings.Contains(parts[0], ",") && !strings.Contains(parts[0], ".") {
		parts[0] = strings.ReplaceAll(parts[0], ",", ".")
	}

	maxPrice, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("I did not understand `%s` as a maximum price, the first word must be a number!", parts[0]),
		))
		return
	}

	keywords := strings.TrimSpace(strings.Join(parts[1:], " "))
	if len(keywords) == 0 {
		sendLog(wb.bot.Reply(m,
			"You need to specify at least one keyword",
		))
		return
	}

	err = wb.db.UserUpdate(m.Sender.ID, func(u *database.User) error {
		u.Searches.Set(&database.SavedSearch{
			Keywords: keywords,
			MaxPrice: maxPrice,
		})
		return nil
	})
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("error creating search: %v", err),
		))
	}

	sendLog(wb.bot.Reply(m,
		fmt.Sprintf("Created new saved search for `%s` and max price of %s", keywords, parts[0]),
	))
}

func (wb *Wallabot) HandleSavedSearches(m *telebot.Message) {
	var searches database.SavedSearches
	err := wb.db.User(m.Sender.ID, func(u *database.User) error {
		searches = u.Searches
		return nil
	})
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("error getting saved searches: %v", err),
		))
		return
	}

	if len(searches) == 0 {
		sendLog(wb.bot.Reply(m,
			"You do not have any saved search. You can create one with /new.",
		))
		return
	}

	var msg string
	for _, ss := range searches {
		msg += fmt.Sprintf("- `%s` (%.2f, %d)\n", ss.Keywords, ss.MaxPrice, len(ss.SentItems))
	}
	sendLog(wb.bot.Reply(m, strings.TrimSpace(msg)))
}

func (wb *Wallabot) HandleDeleteSearch(m *telebot.Message) {
	keywords := strings.TrimSpace(m.Payload)

	if len(keywords) == 0 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("`Usage: %s [keywords...]`", "/delete"),
		))
		return
	}

	var found bool
	err := wb.db.UserUpdate(m.Sender.ID, func(u *database.User) error {
		found = u.Searches.Delete(m.Payload)
		return nil
	})
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("error getting saved searches: %v", err),
		))
		return
	}

	if !found {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("You do not have any saved search for `%s`", keywords),
		))
		return
	}

	sendLog(wb.bot.Reply(m,
		fmt.Sprintf("Search `%s` has been deleted", keywords),
	))
}

func (wb *Wallabot) HandleHelp(m *telebot.Message) {
	sendLog(wb.bot.Reply(m,
		"Oopsie woopsie, I did not get that command :(\n"+
			"Right now I support:\n"+
			"- /search <search query>",
	))
}

func (wb *Wallabot) withUser(handler func(message *telebot.Message)) func(message *telebot.Message) {
	return func(m *telebot.Message) {
		u := &database.User{
			ID:     m.Sender.ID,
			Name:   m.Sender.Username,
			ChatID: m.Chat.ID,
			Searches: database.SavedSearches{},
		}

		err := wb.db.AssertUser(u)
		if err != nil {
			log.Printf("error asserting user '%d': %v", m.Sender.ID, err)
			return
		}

		handler(m)
	}
}

func sendLog(m *telebot.Message, err error) *telebot.Message {
	if err != nil {
		log.Printf("error sending message: %v", err)
	}

	return m
}
