package main

import (
	"encoding/json"
	"fmt"
	tb "gopkg.in/tucnak/telebot.v2"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const TG_API_BASE = "https://api.telegram.org"
const WP_API_BASE = "https://es.wallapop.com/rest/items"
const WP_SENDLINK_BASE = "https://p.wallapop.com/i"

type wpResponse struct {
	Items []*wpItem
}

type wpItem struct {
	ItemId    uint64
	Title     string
	Url       string
	Price     string
	SalePrice uint
}

var db Database
var tg *tb.Bot

func main() {
	var err error

	db, err = openJsonDB("db.json")
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
		return
	}

	tg, err = tb.NewBot(tb.Settings{
		Token:  TG_BOT_TOKEN,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second, LastUpdateID: db.Config().lastUpdate},
	})

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	if len(os.Args) > 1 {
		upd := <-tg.Updates
		fmt.Println(upd)
		return
	}

	tg.Handle("/start", tgStart)
	tg.Handle("/location", tgLocation)
	tg.Handle("/searches", tgSearches)
	tg.Handle("/search", tgSearch)

	tg.Handle("/help", func(m *tb.Message) {
		tg.Send(m.Sender, "/location: Set location (/location 40.23981,1.39681")
	})

	go wallapoll()

	tg.Start()
}

func tgStart(m *tb.Message) {
	log.Printf("New user %d\n", m.Chat.ID)
	db.Lock()
	db.Users()[m.Chat.ID] = &user{
		Location: "0.000,0.000",
		Searches: make(map[string]map[string]interface{}),
		Sent:     make(map[uint64]uint),
	}
	db.Unlock()
	db.Save()

	tg.Reply(m, "Wellcome to wallabot (private beta)! I have created an account for you.")
}

func tgLocation(m *tb.Message) {
	rx := regexp.MustCompile(`^\d+\.\d+,\d+\.\d+$`)
	if !rx.MatchString(m.Payload) {
		tg.Reply(m, "Usage: /location lat,long (/location 41.3658,1.2568)")
		return
	}

	db.Lock()
	db.Users()[m.Chat.ID].Location = m.Payload
	db.Unlock()

	// TODO: Answer with venue
	tg.Reply(m, "Location set to " + m.Payload)
}

func tgSearches(m *tb.Message) {
	db.RLock()
	user := db.Users()[m.Chat.ID].Copy()
	db.RUnlock()

	if user == nil {
		tg.Reply(m, "You need to create an account first! Please, type /start.")
		return
	}

	if len(user.Searches) == 0 {
		tg.Reply(m, "You have no registered searches. You can create a new one using /search.")
		return
	}

	msg := "Your searches:\n"
	for name, params := range user.Searches {
		msg += name + ":\n"
		for name, value := range params {
			msg += fmt.Sprintf("  %s: %v\n", name, value)
		}
		msg += "\n"
	}

	tg.Reply(m, msg)
}

func tgSearch(m *tb.Message) {
	if len(m.Payload) == 0 {
		tg.Reply(m, "Usage: " + m.Text + " searchname")
		return
	}
}

func wpReq(params map[string]interface{}) string {
	str := WP_API_BASE + "/?_p=1"
	for k, v := range params {
		str += fmt.Sprintf("&"+k+"=%v", v)
	}

	return str
}

func wallapoll() {
	for {
		users := make(map[int64]*user)
		db.RLock()
		for k, v := range db.Users() {
			users[k] = v.Copy()
		}
		db.RUnlock()

		for chatId, user := range users {
			latlong := strings.Split(user.Location, ",")
			for name, search := range user.Searches {
				search["latitude"] = latlong[0]
				search["longitude"] = latlong[1]

				resp, err := http.Get(wpReq(search))
				if err != nil {
					log.Println("Error while requesting from wallapop, sleeping 10s: " + err.Error())
					time.Sleep(10 * time.Second)
				}

				items := wpResponse{}
				json.NewDecoder(resp.Body).Decode(&items)

				for _, item := range items.Items {
					db.RLock()
					prevPrice := db.Users()[chatId].Sent[item.ItemId]
					db.RUnlock()
					if prevPrice == item.SalePrice {
						continue
					}

					log.Printf("Match for %d, \"%s/%d\"", chatId, name, item.ItemId)

					//var tgData struct {
					//	Chat_id uint64 `json:"chat_id"`
					//	Text    string `json:"text"`
					//}
					//tgData.Chat_id = chatId
					//tgData.Text = name + ", " + item.Price + ":\n" + WP_SENDLINK_BASE + "/" + fmt.Sprintf("%d", item.ItemId)
					//
					//js, _ := json.Marshal(&tgData)
					//resp, err := http.Post(tgReq("sendMessage"), "application/json", bytes.NewReader(js))
					//if err != nil {
					//	log.Println("Error sending item to tg: " + err.Error())
					//	continue
					//}
					//
					//if resp.StatusCode != 200 {
					//	log.Println("Response: " + resp.Status + " " + resp.Request.URL.String())
					//	continue
					//}

					db.Lock()
					db.Users()[chatId].Sent[item.ItemId] = item.SalePrice
					db.Unlock()
				}

				time.Sleep(500 * time.Millisecond)
			}

			db.Save()
			time.Sleep(3 * time.Minute)
		}
	}
}
