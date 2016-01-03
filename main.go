package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/naoina/toml"
	"golang.org/x/net/publicsuffix"
)

var (
	foundURLsBucket     = []byte("foundURLs")
	illustrationsBucket = []byte("illustrations")
	thumbnailsBucket    = []byte("thumbnails")
)

type Illust struct {
	URL          *url.URL
	Title        string
	ThumbnailURL *url.URL
	Author       string
	FirstSeen    time.Time
}

var config struct {
	Username    string
	Password    string
	Datafile    string
	PollMinutes int
	Listen      string
}

var datafile *bolt.DB

var client http.Client

func init() {
	var err error
	client.Jar, err = cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})
	if err != nil {
		die("Couldn't initialize cookie jar: %v", err)
	}

	client.Timeout = time.Second * 30

	rand.Seed(time.Now().UnixNano())
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
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

func openDatafile() {
	var err error
	datafile, err = bolt.Open(config.Datafile, 0600, nil)
	if err != nil {
		die("Couldn't open datafile: %v", err)
	}

	err = datafile.Update(func(tx *bolt.Tx) error {
		for _, bucket := range [][]byte{foundURLsBucket, thumbnailsBucket, illustrationsBucket} {
			_, err := tx.CreateBucketIfNotExists(bucket)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		die("Couldn't create bucket in datafile: %v", err)
	}
}

func main() {
	loadConfig()
	openDatafile()
	go readFromPixiv()
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/rss.xml", serveRSS)
	http.HandleFunc("/thumbs/", serveThumbs)
	pollThumbnailGrabber()
	die("Couldn't create http server: %v", http.ListenAndServe(config.Listen, nil))
}
