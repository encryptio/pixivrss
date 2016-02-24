package main

import (
	"encoding/hex"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
)

const rssEntryCount = 50

func serveThumbs(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.URL.EscapedPath(), "/", 3)
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	thumbURL, err := hex.DecodeString(parts[2])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	thumb, err := getThumbnail(string(thumbURL))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Couldn't get thumbnail for %v: %v", string(thumbURL), err)
		return
	}

	if thumb == nil {
		http.NotFound(w, r)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(thumb.Data)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	illusts, err := recentIllustrations()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Couldn't get illustrations: %v", err)
		return
	}

	w.Write([]byte("<!DOCTYPE html>"))
	w.Write([]byte("<html><body>"))

	for _, il := range illusts {
		fmt.Fprintf(w, `<div><a rel="noreferrer" href="%s"><img src="thumbs/%s"> %s - %s</a></div>`,
			html.EscapeString(il.URL), hex.EncodeToString([]byte(il.ThumbnailURL)), html.EscapeString(il.Author), html.EscapeString(il.Title))
	}

	w.Write([]byte("</body></html>"))
}

func serveRSS(w http.ResponseWriter, r *http.Request) {
	illusts, err := recentIllustrations()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Couldn't get illustrations: %v", err)
		return
	}

	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8" ?>`+"\n")
	fmt.Fprintf(w, `<rss version="2.0">`)
	fmt.Fprintf(w, `<channel>`)
	fmt.Fprintf(w, `<title>Pixiv subscriptions for %v</title>`, html.EscapeString(config.Username))
	fmt.Fprintf(w, `<description>All pixiv illustrations for artists bookmarked by %v</description>`, html.EscapeString(config.Username))
	fmt.Fprintf(w, `<link>http://www.pixiv.net/bookmark_new_illust.php</link>`)
	fmt.Fprintf(w, `<ttl>%v</ttl>`, config.PollMinutes*60)

	for _, il := range illusts {
		fmt.Fprintf(w, `<item>`)
		fmt.Fprintf(w, `<title>%s - %s</title>`, html.EscapeString(il.Author), html.EscapeString(il.Title))
		desc := fmt.Sprintf(`<a rel="noreferrer" href="%s"><img src="thumbs/%s"><br>%s - %s</a>`,
			html.EscapeString(il.URL), hex.EncodeToString([]byte(il.ThumbnailURL)), html.EscapeString(il.Author), html.EscapeString(il.Title))
		fmt.Fprintf(w, `<description>%s</description>`, html.EscapeString(desc))
		fmt.Fprintf(w, `<link>%s</link>`, html.EscapeString(il.URL))
		fmt.Fprintf(w, `</item>`)
	}

	fmt.Fprintf(w, `</channel></rss>`)
}
