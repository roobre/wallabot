package main

import (
	"flag"
	"log"
	"os"
	"roob.re/wallabot/telegram"
	"strconv"
)

func main() {
	token := flag.String("token", os.Getenv("WB_TOKEN"), "Telegram bot token")
	dbpath := flag.String("dbpath", os.Getenv("WB_DBPATH"), "Path to database")
	verbose := flag.Bool("verbose", func() bool {
		b, _ := strconv.ParseBool(os.Getenv("WB_VERBOSE"))
		return b
	}(), "Be verbose")
	flag.Parse()

	wb, err := telegram.NewWallabot(telegram.WallabotConfig{
		Token: *token,
		Verbose: *verbose,
		DBPath: *dbpath,
	})
	if err != nil {
		log.Fatalln(err)
	}

	err = wb.Start()
	if err != nil {
		log.Fatalln(err)
	}
}
