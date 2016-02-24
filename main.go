package main

import (
	"net/http"
	"os"

	"github.com/naoina/toml"
)

var config struct {
	// pixiv options
	Username    string
	Password    string
	PollMinutes int

	// local options
	Listen       string
	RespondCount int

	// database options
	DBAddress string `toml:"db-address"`
	DBName    string `toml:"db-name"`
}

func init() {
	config.PollMinutes = 15
	config.Listen = ":8080"
	config.RespondCount = 50
	config.DBAddress = "localhost:28015"
	config.DBName = "pixivrss"
}

func loadConfig() {
	if len(os.Args) != 2 {
		die("Usage: %s config-file.toml", os.Args[0])
	}

	configFile := os.Args[1]

	fh, err := os.Open(configFile)
	if err != nil {
		die("Couldn't open %v: %v", configFile, err)
	}
	defer fh.Close()

	err = toml.NewDecoder(fh).Decode(&config)
	if err != nil {
		die("Couldn't load config from %v: %v", configFile, err)
	}
}

func main() {
	loadConfig()
	openDB()
	go getThumbnails()
	go readFromPixiv()
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/rss.xml", serveRSS)
	http.HandleFunc("/thumbs/", serveThumbs)
	die("Couldn't create http server: %v", http.ListenAndServe(config.Listen, nil))
}
