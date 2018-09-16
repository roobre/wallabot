package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
	"strings"
)

const TG_API_BASE = "https://api.telegram.org"
const WP_API_BASE = "https://es.wallapop.com/rest/items"
const WP_SENDLINK_BASE = "https://es.wallapop.com/item"

var db []struct {
	ChatId   uint64
	Location string
	Searches []map[string]interface{}
}

var sent map[string]string

type wpResponse struct {
	Items []wpItem
}

type wpItem struct {
	ItemId uint64
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

	sentfile, err := os.Create("sent.json")
	if err != nil {
		fmt.Println("Could not create sent.json: " + err.Error())
		return
	}

	for {
		for _, user := range db {
			latlong := strings.Split(user.Location, ":")
			for _, search := range user.Searches {
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
					if sent[fmt.Sprintf("%d:%d", user.ChatId, item.ItemId)] == item.Price {
						continue
					}

					resp, err := http.Get(tgReq(fmt.Sprintf(
						"sendMessage?chat_id=%d&text=%s",
						user.ChatId,
						url.QueryEscape(WP_SENDLINK_BASE+"/"+item.Url))))

					if err != nil {
						log.Println("Error sending item to tg: " + err.Error())
						continue
					}

					if resp.StatusCode != 200 {
						log.Println("Response: " + resp.Status + " " + resp.Request.URL.String())
						continue
					}

					sent[fmt.Sprintf("%d:%d", user.ChatId, item.ItemId)] = item.Price
				}

				time.Sleep(2 * time.Second)
			}

			sentfile.Truncate(0)
			json.NewEncoder(sentfile).Encode(sent)

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
