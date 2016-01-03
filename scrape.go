package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/boltdb/bolt"
	"golang.org/x/net/html/charset"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.106 Safari/537.36"
)

func mustEncodeJSON(i interface{}) []byte {
	data, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return data
}

func mustDecodeJSON(data []byte, i interface{}) {
	err := json.Unmarshal(data, i)
	if err != nil {
		panic(err)
	}
}

func pollPixiv() {
	fmt.Printf("Polling for new illustrations\n")
	defer fmt.Printf("Poll complete\n")

	resp, err := client.Get("http://www.pixiv.net/bookmark_new_illust.php")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't GET new illustrations page: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "Couldn't GET new illustrations page: got status %v\n", resp.Status)
		return
	}

	if strings.Contains(resp.Request.URL.String(), "return_to") {
		// need to log in.
		fmt.Printf("Logging into pixiv\n")

		data := "mode=login&return_to=%2Fbookmark_new_illust.php&skip=1" +
			"&pixiv_id=" + url.QueryEscape(config.Username) +
			"&pass=" + url.QueryEscape(config.Password)
		req, err := http.NewRequest("POST", "https://www.secure.pixiv.net/login.php", strings.NewReader(data))
		if err != nil {
			panic(err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp2, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Couldn't POST to login page: %v\n", err)
			return
		}
		defer resp2.Body.Close()

		if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
			fmt.Fprintf(os.Stderr, "Couldn't POST to login page: got status %v\n", resp.Status)
			return
		}

		if resp2.Request.URL.Path != "/bookmark_new_illust.php" {
			fmt.Fprintf(os.Stderr, "Was not at illustration after attempted login. Bad username/password?\n")
			return
		}

		resp = resp2
	}

	r, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't decode response charset: %v\n", err)
		return
	}

	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't parse HTML: %v\n", err)
		return
	}

	// look for:
	// <div class="layout-body">
	//     ...
	//     <a href="/member_illust.php?mode=medium&illust_id=NUMERIC">
	//         <h1 class="title" title="TITLE GOES HERE">TITLE GOES HERE</h1>
	//     </a>
	// </div>
	found := 0
	doc.Find("div.layout-body a[href*=member_illust][href*=illust_id] > h1.title").Each(func(i int, h1 *goquery.Selection) {
		found++

		href, _ := h1.Parent().Attr("href")
		u, _ := url.Parse(href)
		if u == nil {
			return
		}
		u = resp.Request.URL.ResolveReference(u)

		title := h1.Text()

		thumbSrc, _ := h1.Parent().Parent().Find("img._thumbnail").Attr("src")
		thumbURL, _ := url.Parse(thumbSrc)
		if thumbURL != nil {
			thumbURL = resp.Request.URL.ResolveReference(thumbURL)
		}

		author, _ := h1.Parent().Parent().Find("a.user[data-user_name]").Attr("data-user_name")

		handleIllust(Illust{
			URL:          u,
			Title:        title,
			ThumbnailURL: thumbURL,
			Author:       author,
			FirstSeen:    time.Now(),
		})
	})

	if found == 0 {
		fmt.Fprintf(os.Stderr, "Didn't find any illustrations on page; scraper is out of date\n")
	}
}

func handleIllust(il Illust) {
	err := datafile.Update(func(tx *bolt.Tx) error {
		foundURLs := tx.Bucket(foundURLsBucket)
		illusts := tx.Bucket(illustrationsBucket)

		if foundURLs.Get([]byte(il.URL.String())) != nil {
			return nil
		}

		foundURLs.Put([]byte(il.URL.String()), []byte{})

		fmt.Printf("Found new illustration:\n")
		fmt.Printf("Title: %v\n", il.Title)
		fmt.Printf("Author: %v\n", il.Author)
		fmt.Printf("URL: %v\n", il.URL)
		fmt.Printf("Thumbnail URL: %v\n", il.ThumbnailURL)

		id, err := illusts.NextSequence()
		if err != nil {
			return err
		}
		var idBuf [8]byte
		binary.BigEndian.PutUint64(idBuf[:], id)

		illusts.Put(idBuf[:], mustEncodeJSON(il))

		return nil
	})
	if err != nil {
		die("Couldn't update illustration in DB: %v\n", err)
	}

	pollThumbnailGrabber()
}

func readFromPixiv() {
	for {
		pollPixiv()

		time.Sleep(time.Duration((0.8 + rand.Float64()*0.4) * float64(time.Minute*time.Duration(config.PollMinutes))))
	}
}
