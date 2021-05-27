package telegram

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"
	"net/http"
	"roob.re/wallabot/database"
	"roob.re/wallabot/wallapop"
	"strings"
	"time"
)

type Wallabot struct {
	Notify chan database.Notification

	bot *telebot.Bot
	wp  *wallapop.Client
	db  *database.Database

	c        WallabotConfig
	commands []commandEntry
}

type WallabotConfig struct {
	Verbose     bool
	Timeout     time.Duration
	QueueLength int
	VIPUsers    []string
}

type commandEntry struct {
	command     string
	description string
	handler     func(m *telebot.Message)
}

func (wc WallabotConfig) WithDefaults() WallabotConfig {
	if wc.Timeout == 0 {
		wc.Timeout = 30 * time.Second
	}

	if wc.QueueLength == 0 {
		wc.QueueLength = 64
	}

	return wc
}

func NewWallabot(token string, db *database.Database, wp *wallapop.Client, c WallabotConfig) (*Wallabot, error) {
	if token == "" {
		return nil, errors.New("token must not be empty")
	}

	bot, err := telebot.NewBot(telebot.Settings{
		Token:     token,
		Verbose:   c.Verbose,
		Poller:    &telebot.LongPoller{Timeout: 10 * time.Second},
		ParseMode: telebot.ModeMarkdown,
		Reporter: func(err error) {
			log.WithFields(log.Fields{
				"component": "bot",
			}).Printf("telebot recovered from: %v", err)
		},
		Client: http.DefaultClient,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating bot: %w", err)
	}

	wb := &Wallabot{
		bot:    bot,
		wp:     wp,
		db:     db,
		c:      c.WithDefaults(),
		Notify: make(chan database.Notification, c.QueueLength),
	}

	wb.commands = []commandEntry{
		{
			command:     "/start",
			description: "Show the welcome message",
			handler:     wb.HandleStart,
		},
		{
			command:     "/search",
			description: "Immediately search for items",
			handler:     wb.HandleSearch,
		},
		{
			command:     "/list",
			description: "See my saved searches",
			handler:     wb.withUser(wb.HandleSavedSearches),
		},
		{
			command:     "/new",
			description: "Create a new saved search",
			handler:     wb.withUser(wb.HandleNewSearch),
		},
		{
			command:     "/delete",
			description: "Delete a saved search",
			handler:     wb.withUser(wb.HandleDeleteSearch),
		},
		{
			command:     "/radius",
			description: "Show preferred search radius",
			handler:     wb.withUser(wb.HandleRadius),
		},
		{
			command:     "/location",
			description: "Show preferred location, manually",
			handler:     wb.withUser(wb.HandleLocationText),
		},
		{
			command:     "/me",
			description: "Show info about the current user",
			handler:     wb.withUser(wb.HandleMe),
		},
	}

	return wb.withHandlers(), nil
}

func (wb *Wallabot) withHandlers() *Wallabot {
	commandsHelp := make([]telebot.Command, 0, len(wb.commands))

	for _, cmd := range wb.commands {
		wb.bot.Handle(cmd.command, cmd.handler)

		commandsHelp = append(commandsHelp, telebot.Command{
			Text:        cmd.command,
			Description: cmd.description,
		})
	}

	wb.bot.Handle(telebot.OnLocation, wb.withUser(wb.HandleLocation))
	wb.bot.Handle(telebot.OnText, wb.HandleHelp)

	_ = wb.bot.SetCommands(commandsHelp)

	return wb
}

func (wb *Wallabot) Start() error {
	go wb.processNotifications()
	wb.bot.Start()
	return nil
}

func (wb *Wallabot) processNotifications() {
	for nt := range wb.Notify {
		// Check if we already sent a notification for this search and item for a lower or same price
		lowerPriceNotified := false
		err := wb.db.User(nt.User.ID, func(u *database.User) error {
			search := u.Searches.Get(nt.Search)
			if search == nil {
				return fmt.Errorf("search '%s' not found", nt.Search)
			}

			notifiedPrice, notified := search.SentItems[nt.Item.ID]
			if notified && notifiedPrice <= nt.Item.Price {
				lowerPriceNotified = true
			}
			return nil
		})
		if err != nil {
			log.WithFields(log.Fields{
				"component": "bot",
			}).Errorf("Error checking previous notifications for '%s': %v", nt.User.Name, err)
			continue
		}

		if lowerPriceNotified {
			log.WithFields(log.Fields{
				"component": "bot",
			}).Debugf("Discarding '%s' for '%s' as previously notified", nt.Item.ID, nt.User.Name)
			continue
		}

		log.WithFields(log.Fields{
			"component": "bot",
		}).Printf("Notifying '%s' about '%s'", nt.User.Name, nt.Item.ID)

		_, err = wb.bot.Send(telebot.ChatID(nt.User.ChatID), nt.Item.Markdown(), &telebot.SendOptions{
			ParseMode: telebot.ModeMarkdownV2,
		})
		if err != nil {
			log.WithFields(log.Fields{
				"component": "bot",
			}).Printf("Error notifying '%s' (chatID %d)", nt.User.Name, nt.User.ChatID)
			continue
		}

		err = wb.db.UserUpdate(nt.User.ID, func(u *database.User) error {
			search := u.Searches.Get(nt.Search)
			if search == nil {
				return fmt.Errorf("search '%s' not found", nt.Search)
			}

			search.SentItems[nt.Item.ID] = nt.Item.Price
			return nil
		})
		if err != nil {
			log.WithFields(log.Fields{
				"component": "bot",
			}).Errorf("internal error updating notification for '%s': %v", nt.Search, err)
		}
	}
}

func (wb *Wallabot) withUser(handler func(message *telebot.Message)) func(message *telebot.Message) {
	return func(m *telebot.Message) {
		u := &database.User{
			ID:       m.Sender.ID,
			Name:     m.Sender.Username,
			ChatID:   m.Chat.ID,
			Searches: database.SavedSearches{},
		}

		err := wb.db.AssertUser(u)
		if err != nil {
			log.WithFields(log.Fields{
				"component": "bot",
			}).Errorf("error asserting user '%d': %v", m.Sender.ID, err)
			return
		}

		handler(m)
	}
}

func (wb *Wallabot) userIsVIP(username string) bool {
	for _, u := range wb.c.VIPUsers {
		if strings.ToLower(u) == strings.ToLower(username) {
			return true
		}
	}

	return false
}

func sendLog(m *telebot.Message, err error) *telebot.Message {
	if err != nil {
		log.WithFields(log.Fields{
			"component": "bot",
		}).Errorf("error sending message: %v", err)
	}

	return m
}
