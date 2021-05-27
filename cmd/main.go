package main

import (
	"flag"
	"log"
	"os"
	"roob.re/wallabot"
	"roob.re/wallabot/telegram"
	"strconv"
	"strings"
)

func main() {
	token := flag.String("token", os.Getenv("WB_TOKEN"), "Telegram bot token")
	vipUsers := flag.String("vips", os.Getenv("WB_VIPS"), "Comma-separated list of VIP usernames")
	dbpath := flag.String("dbpath", func() string {
		env := os.Getenv("WB_DBPATH")
		if env == "" {
			env = "./data"
		}
		return env
	}(), "Path to database")
	verbose := flag.Bool("verbose", func() bool {
		b, _ := strconv.ParseBool(os.Getenv("WB_VERBOSE"))
		return b
	}(), "Be verbose")
	flag.Parse()

	var vipUserList []string
	for _, vip := range strings.Split(*vipUsers, ",") {
		vip = strings.TrimSpace(vip)
		if len(vip) > 0 {
			vipUserList = append(vipUserList, vip)
		}
	}

	wb, err := wallabot.New(wallabot.Config{
		DBPath: *dbpath,
		Token:  *token,
		WallabotConfig: telegram.WallabotConfig{
			Verbose: *verbose,
			VIPUsers: vipUserList,
		},
	})

	if err != nil {
		log.Fatalln(err)
	}

	err = wb.Start()
	if err != nil {
		log.Fatalln(err)
	}
}
