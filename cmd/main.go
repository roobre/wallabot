package main

import (
	"flag"
	"log"
	"os"
	"roob.re/wallabot"
	"roob.re/wallabot/telegram"
	"strconv"
)

func main() {
	token := flag.String("token", os.Getenv("WB_TOKEN"), "Telegram bot token")
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

	wb, err := wallabot.New(wallabot.Config{
		DBPath:         *dbpath,
		Token:          *token,
		WallabotConfig: telegram.WallabotConfig{
			Verbose: *verbose,
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
