package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const TG_API_BASE = "https://api.telegram.org"
const WP_API_BASE = "https://es.wallapop.com/rest/items"
const WP_SENDLINK_BASE = "https://p.wallapop.com/i"

var db map[uint64]struct {
	// ChatId   uint64
	Location string
	Searches map[string]map[string]interface{}
}

var sent map[string]string

type wpResponse struct {
	Items []*wpItem
}

type wpItem struct {
	ItemId uint64
	Title  string
	Url    string
	Price  string
}

func main() {
	if len(os.Args) > 1 {
		resp, _ := http.Get(tgReq("getUpdates"))
		io.Copy(os.Stdout, resp.Body)

		return
	}

	sent = make(map[string]string)

	dbfile, err := os.Open("db.json")
	if err != nil {
		fmt.Println("Could not open db.json: " + err.Error())
		return
	}
	err = json.NewDecoder(dbfile).Decode(&db)
	if err != nil {
		fmt.Println("Could not decode json database: " + err.Error())
		return
	}

	sentfile, err := os.OpenFile("sent.json", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Could not create sent.json: " + err.Error())
		return
	}

	err = json.NewDecoder(sentfile).Decode(&sent)
	if err != nil {
		fmt.Println("Could decode already sent notifications: " + err.Error())
		return
	}

	for {
		for chatId, user := range db {
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
				err = json.NewDecoder(resp.Body).Decode(&items)
				_ = resp.Body.Close()
				if err != nil {
					log.Println("Error decoding response from wallapop: " + err.Error())
					continue
				}

				for _, item := range items.Items {
					if sent[fmt.Sprintf("%d:%d", chatId, item.ItemId)] == item.Price {
						continue
					}

					log.Printf("Match for %d, \"%s/%d\"", chatId, name, item.ItemId)

					var tgData struct {
						Chat_id uint64 `json:"chat_id"`
						Text    string `json:"text"`
					}
					tgData.Chat_id = chatId
					tgData.Text = name + ", " + item.Price + ":\n" + WP_SENDLINK_BASE + "/" + fmt.Sprintf("%d", item.ItemId)

					js, _ := json.Marshal(&tgData)
					resp, err := http.Post(tgReq("sendMessage"), "application/json", bytes.NewReader(js))
					if err != nil {
						log.Println("Error sending item to tg: " + err.Error())
						continue
					}

					if resp.StatusCode != 200 {
						body, _ := ioutil.ReadAll(resp.Body)
						log.Printf("Error: %d requesting %s: %s", resp.StatusCode, resp.Request.URL.String(), string(body))
						_ = resp.Body.Close()
						continue
					}
					_ = resp.Body.Close()

					sent[fmt.Sprintf("%d:%d", chatId, item.ItemId)] = item.Price
				}

				time.Sleep(2 * time.Second)
			}

			err = sentfile.Truncate(0)
			err = json.NewEncoder(sentfile).Encode(&sent)
			if err != nil {
				log.Println("Could write back sent notifications: " + err.Error())
			}

			time.Sleep(1 * time.Minute)
		}

	}
}

func tgReq(endpoint string) string {
	return fmt.Sprintf("%s/bot%s/%s", TG_API_BASE, TG_BOT_TOKEN, endpoint)
}

func wpReq(params map[string]interface{}) string {
	str := WP_API_BASE + "/?_p=1"
	for k, v := range params {
		str += fmt.Sprintf("&"+k+"=%v", v)
	}

	return str
}
