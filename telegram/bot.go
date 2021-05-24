package telegram

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"
	"math"
	"net/http"
	"roob.re/wallabot/database"
	"roob.re/wallabot/wallapop"
	"strconv"
	"strings"
	"time"
)

type Wallabot struct {
	Notify chan database.Notification

	bot *telebot.Bot
	wp  *wallapop.Client
	db  *database.Database

	c WallabotConfig
}

type WallabotConfig struct {
	Verbose     bool
	Timeout     time.Duration
	QueueLength int
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

	return (&Wallabot{
		bot:    bot,
		wp:     wp,
		db:     db,
		c:      c.WithDefaults(),
		Notify: make(chan database.Notification, c.QueueLength),
	}).withHandlers(), nil
}

func (wb *Wallabot) withHandlers() *Wallabot {
	wb.bot.Handle("/search", wb.withUser(wb.HandleSearch))
	wb.bot.Handle("/list", wb.withUser(wb.HandleSavedSearches))
	wb.bot.Handle("/new", wb.withUser(wb.HandleNewSearch))
	wb.bot.Handle("/delete", wb.withUser(wb.HandleDeleteSearch))
	wb.bot.Handle("/radius", wb.withUser(wb.HandleRadius))
	wb.bot.Handle("/me", wb.withUser(wb.HandleMe))
	wb.bot.Handle(telebot.OnLocation, wb.withUser(wb.HandleLocation))
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
		{
			Text:        "/radius",
			Description: "Show preferred search radius",
		},
		{
			Text:        "/me",
			Description: "Show info about the current user",
		},
	})

	return wb
}

func (wb *Wallabot) Start() error {
	go wb.processNotifications()
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

	var user *database.User
	err := wb.db.User(m.Sender.ID, func(u *database.User) error {
		user = u
		return nil
	})
	if err != nil {
		log.WithFields(log.Fields{
			"component": "bot",
		}).Errorf("Could not get user '%s' (%d) from database: %v", m.Sender.Username, m.Sender.ID, err)

		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Error getting you from the database: %v", err),
		))

		return
	}

	results, err := wb.wp.Search(wallapop.SearchArgs{
		Keywords:  keywords,
		Latitude:  user.Lat,
		Longitude: user.Long,
	})
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Error processing your search: %v", err),
		))

		return
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
			MaxPrice: math.Round(maxPrice),
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
		msg += fmt.Sprintf("- `%s` (%.2f 📈, %d 🔔)\n", ss.Keywords, ss.MaxPrice, len(ss.SentItems))
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

func (wb *Wallabot) HandleLocation(m *telebot.Message) {
	if m.Location == nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Your message does not have a valid location"),
		))
		return
	}

	err := wb.db.UserUpdate(m.Sender.ID, func(u *database.User) error {
		u.Lat = float64(m.Location.Lat)
		u.Long = float64(m.Location.Lng)
		return nil
	})
	if err != nil {
		log.WithFields(log.Fields{
			"component": "bot",
		}).Errorf("Saving location for %v", err)

		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("error saving your location: %v", err),
		))
		return
	}

	sendLog(wb.bot.Reply(m,
		fmt.Sprintf("I've set your location to %.8f,%.8f", m.Location.Lat, m.Location.Lng),
	))
}

func (wb *Wallabot) HandleRadius(m *telebot.Message) {
	radius, err := strconv.Atoi(m.Payload)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			"`Usage: /radius <num kilometers>",
		))
		return
	}

	err = wb.db.UserUpdate(m.Sender.ID, func(u *database.User) error {
		u.RadiusKm = radius
		return nil
	})
	if err != nil {
		log.WithFields(log.Fields{
			"component": "bot",
		}).Errorf("Saving radius for %v", err)

		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("error saving your search radius: %v", err),
		))
		return
	}

	sendLog(wb.bot.Reply(m,
		fmt.Sprintf("I've set your preferred search radius to %d Km", radius),
	))
}

func (wb *Wallabot) HandleMe(m *telebot.Message) {
	var user *database.User
	err := wb.db.User(m.Sender.ID, func(u *database.User) error {
		user = u
		return nil
	})
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Could not get your entry from database: %v", err),
		))
		return
	}

	sendLog(wb.bot.Reply(m,
		fmt.Sprintf("👤: %s\n"+
			"📍: %.8f, %.8f (+%dKm)\n"+
			"You can send me your location fo configure it, and use /radius to set your desired search radius",
			user.Name,
			user.Lat, user.Long, user.RadiusKm,
		),
	))
}

func (wb *Wallabot) HandleHelp(m *telebot.Message) {
	sendLog(wb.bot.Reply(m, "Oopsie woopsie, I did not get that command :("))
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

func sendLog(m *telebot.Message, err error) *telebot.Message {
	if err != nil {
		log.WithFields(log.Fields{
			"component": "bot",
		}).Errorf("error sending message: %v", err)
	}

	return m
}
