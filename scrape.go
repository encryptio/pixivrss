package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"golang.org/x/net/publicsuffix"
)

const (
	userAgent = "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.106 Safari/537.36"
)

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
	log.Printf("Polling for new illustrations\n")
	defer log.Printf("Poll complete\n")

	found := 0
	for _, pageName := range []string{
		"bookmark_new_illust.php",
		"bookmark_new_illust_r18.php",
	} {

		resp, err := client.Get("http://www.pixiv.net/" + pageName)
		if err != nil {
			log.Printf("Couldn't GET new illustrations page: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf("Couldn't GET new illustrations page: got status %v\n", resp.Status)
			return
		}

		if strings.Contains(resp.Request.URL.String(), "return_to") {
			// need to log in.
			log.Printf("Logging into pixiv\n")

			data := "mode=login&return_to=%2F" + pageName + "&skip=1" +
				"&pixiv_id=" + url.QueryEscape(config.Username) +
				"&pass=" + url.QueryEscape(config.Password)
			req, err := http.NewRequest("POST", "https://www.pixiv.net/login.php", strings.NewReader(data))
			if err != nil {
				panic(err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			resp2, err := client.Do(req)
			if err != nil {
				log.Printf("Couldn't POST to login page: %v\n", err)
				return
			}
			defer resp2.Body.Close()

			if resp2.StatusCode < 200 || resp2.StatusCode >= 400 {
				log.Printf("Couldn't POST to login page: got status %v\n", resp2.Status)
				return
			}

			if resp2.Request.URL.Path != "/"+pageName {
				log.Printf("Was not at illustration after attempted login. Bad username/password?\n")
				return
			}

			resp = resp2
		}

		r, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
		if err != nil {
			log.Printf("Couldn't decode response charset: %v\n", err)
			return
		}

		doc, err := goquery.NewDocumentFromReader(r)
		if err != nil {
			log.Printf("Couldn't parse HTML: %v\n", err)
			return
		}

		// look for:
		// <div class="layout-body">
		//     ...
		//     <a href="/member_illust.php?mode=medium&illust_id=NUMERIC">
		//         <h1 class="title" title="TITLE GOES HERE">TITLE GOES HERE</h1>
		//     </a>
		// </div>
		doc.Find("div.layout-body a[href*=member_illust][href*=illust_id] > h1.title").Each(func(i int, h1 *goquery.Selection) {
			found++

			href, _ := h1.Parent().Attr("href")
			u, _ := url.Parse(href)
			if u == nil {
				return
			}
			u = resp.Request.URL.ResolveReference(u)
			urlStr := u.String()

			title := h1.Text()

			thumbSrc, _ := h1.Parent().Parent().Find("img._thumbnail").Attr("src")
			thumbURL, _ := url.Parse(thumbSrc)
			var thumbURLStr string
			if thumbURL != nil {
				thumbURL = resp.Request.URL.ResolveReference(thumbURL)
				thumbURLStr = thumbURL.String()
			}

			author, _ := h1.Parent().Parent().Find("a.user[data-user_name]").Attr("data-user_name")

			handleIllust(Illust{
				URL:          urlStr,
				Title:        title,
				ThumbnailURL: thumbURLStr,
				Author:       author,
				FirstSeen:    time.Now(),
			})
		})
	}

	if found == 0 {
		log.Printf("Didn't find any illustrations on page; scraper is out of date\n")
	}
}

func readFromPixiv() {
	for {
		pollPixiv()

		time.Sleep(time.Duration((0.8 + rand.Float64()*0.4) * float64(time.Minute*time.Duration(config.PollMinutes))))
	}
}
