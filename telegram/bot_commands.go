package telegram

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"
	"roob.re/wallabot/database"
	searchcmd "roob.re/wallabot/telegram/search"
)

func (wb *Wallabot) HandleSearch(m *telebot.Message) {
	const maxResults = 10

	search, err := searchcmd.New(m.Payload)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Error %v", err),
		))
		return
	}

	if search.Keywords == "" || search.MaxPrice == 0 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("`Usage: %s <price=100> [radius=100] [strict=false] [nozero=false] search string...`", "/search"),
		))
		return
	}

	var user *database.User
	err = wb.db.User(m.Sender.ID, func(u *database.User) error {
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

	if search.RadiusKm == 0 {
		search.RadiusKm = user.RadiusKm
	}

	args := search.Args()

	lat, long := user.Location()
	args.Latitude = lat
	args.Longitude = long

	results, err := wb.wp.Search(args)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Error processing your search: %v", err),
		))

		return
	}

	if len(results) == 0 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Could not find any results for '%s'", search),
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
	search, err := searchcmd.New(m.Payload)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("Error %v", err),
		))
		return
	}

	if search.Keywords == "" || search.MaxPrice == 0 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("`Usage: %s <price=100> [radius=100] [strict=false] [nozero=false] search string...`", "/search"),
		))
		return
	}

	err = wb.db.UserUpdate(m.Sender.ID, func(u *database.User) error {
		if (!wb.userIsVIP(m.Sender.Username) && len(u.Searches) >= 5) ||
			len(u.Searches) >= 15 {
			return fmt.Errorf("you have reached the maximum number of searches")
		}

		u.Searches.Set(&database.SavedSearch{
			Search: search,
		})

		return nil
	})
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("error creating saved search: %v", err),
		))

		return
	}

	sendLog(wb.bot.Reply(m,
		fmt.Sprintf("Created new saved search `%s`", search.Keywords),
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
		ss.LegacyFill()
		msg += fmt.Sprintf("%s\n", ss.Emojify())
	}
	sendLog(wb.bot.Reply(m, strings.TrimSpace(msg)))
}

func (wb *Wallabot) HandleDeleteSearch(m *telebot.Message) {
	keywords := strings.TrimSpace(m.Payload)

	if len(keywords) == 0 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("`Usage: %s <search>`", "/delete"),
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

func (wb *Wallabot) HandleLocationText(m *telebot.Message) {
	latLongTxt := strings.Split(m.Payload, ",")
	if len(latLongTxt) != 2 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("`Usage: /location <latitude,longitude>`\n`Example: /location 41.383333,2.183333`"),
		))
		return
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(latLongTxt[0]), 64)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("I couldn't parse '%s' as a latitude", latLongTxt[0]),
		))
		return
	}

	long, err := strconv.ParseFloat(strings.TrimSpace(latLongTxt[1]), 64)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("I couldn't parse '%s' as a longitude", latLongTxt[1]),
		))
		return
	}

	err = wb.db.UserUpdate(m.Sender.ID, func(u *database.User) error {
		u.Lat = lat
		u.Long = long
		return nil
	})
	if err != nil {
		log.WithFields(log.Fields{
			"component": "bot",
		}).Errorf("Saving location ('%s') for user %d: %v", m.Payload, m.Sender.ID, err)

		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("error saving your location: %v", err),
		))
		return
	}

	sendLog(wb.bot.Reply(m,
		fmt.Sprintf("I've set your location to [%f, %f]", lat, long),
	))
}

func (wb *Wallabot) HandleRadius(m *telebot.Message) {
	radius, err := strconv.Atoi(m.Payload)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			"`Usage: /radius <num kilometers>`",
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

	var vipMessage string
	if wb.userIsVIP(m.Sender.Username) {
		vipMessage = "🥇 You are a VIP user\n"
	}

	sendLog(wb.bot.Reply(m,
		fmt.Sprintf("👤 %s\n"+
			"📍 %.8f, %.8f (+%dKm)\n"+
			vipMessage+
			"You can send me your location fo configure it, and use /radius to set your desired search radius",
			user.Name,
			user.Lat, user.Long, user.RadiusKm,
		),
	))
}

func (wb *Wallabot) HandleHelp(m *telebot.Message) {
	supportedStr := ""
	for _, cmd := range wb.commands {
		supportedStr += fmt.Sprintf("%s\n`  `%s\n", cmd.command, cmd.description)
	}

	sendLog(wb.bot.Reply(m,
		"Oopsie woopsie, I did not get that command :(\n"+
			"I currently support the following ones:\n\n"+
			supportedStr+
			"\nAdditionally you can send me a location directly to easily update your preferred location",
	))
}

func (wb *Wallabot) HandleStart(m *telebot.Message) {
	sendLog(wb.bot.Reply(m, "Welcome! I'm a bot that can help you to search and monitor items in Wallapop.\n"+
		"Want to know what I can do? Throw me a /help command!"))
}
