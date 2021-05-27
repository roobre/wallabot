package telegram

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/tucnak/telebot.v2"
	"math"
	"roob.re/wallabot/database"
	"roob.re/wallabot/wallapop"
	"strconv"
	"strings"
)

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

	lat, long := user.Location()
	results, err := wb.wp.Search(wallapop.SearchArgs{
		Keywords:  keywords,
		Latitude:  lat,
		Longitude: long,
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
		if (!wb.userIsVIP(m.Sender.Username) && len(u.Searches) >= 5) ||
			len(u.Searches) >= 15 {
			return fmt.Errorf("you have reached the maximum number of searches")
		}

		u.Searches.Set(&database.SavedSearch{
			Keywords: keywords,
			MaxPrice: math.Round(maxPrice),
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

func (wb *Wallabot) HandleLocationText(m *telebot.Message) {
	latLongTxt := strings.Split(m.Payload, ",")
	if len(latLongTxt) != 2 {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("`Usage: /location <latitude,longitude>`\n`Example: /location 41.383333,2.183333`"),
		))
		return
	}

	lat, err := strconv.ParseFloat(latLongTxt[0], 64)
	if err != nil {
		sendLog(wb.bot.Reply(m,
			fmt.Sprintf("I couldn't parse '%s' as a latitude", latLongTxt[0]),
		))
		return
	}

	long, err := strconv.ParseFloat(latLongTxt[1], 64)
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
			vipMessage +
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
