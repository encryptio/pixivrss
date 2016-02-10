package main

import (
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/boltdb/bolt"
)

const rssEntryCount = 50

func serveThumbs(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.URL.Path, "/", 3)
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	id, err := hex.DecodeString(parts[2])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	datafile.View(func(tx *bolt.Tx) error {
		thumbs := tx.Bucket(thumbnailsBucket)

		data := thumbs.Get(id)
		if data == nil {
			http.NotFound(w, r)
			return nil
		}

		w.Header().Set("Content-Type", http.DetectContentType(data))
		w.Write(data)

		return nil
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<!DOCTYPE html>"))
	w.Write([]byte("<html><body>"))

	datafile.View(func(tx *bolt.Tx) error {
		illusts := tx.Bucket(illustrationsBucket)

		illusts.ForEach(func(k, v []byte) error {
			var il Illust
			mustDecodeJSON(v, &il)

			fmt.Fprintf(w, `<div><a rel="noreferrer" href="%s"><img src="thumbs/%s"> %s - %s</a></div>`,
				html.EscapeString(il.URL.String()), hex.EncodeToString(k), html.EscapeString(il.Author), html.EscapeString(il.Title))

			return nil
		})

		return nil
	})

	w.Write([]byte("</body></html>"))
}

func serveRSS(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8" ?>`+"\n")
	fmt.Fprintf(w, `<rss version="2.0">`)
	fmt.Fprintf(w, `<channel>`)
	fmt.Fprintf(w, `<title>Pixiv subscriptions for %v</title>`, html.EscapeString(config.Username))
	fmt.Fprintf(w, `<description>All pixiv illustrations for artists bookmarked by %v</description>`, html.EscapeString(config.Username))
	fmt.Fprintf(w, `<link>http://www.pixiv.net/bookmark_new_illust.php</link>`)
	fmt.Fprintf(w, `<ttl>%v</ttl>`, config.PollMinutes*60)

	datafile.View(func(tx *bolt.Tx) error {
		cur := tx.Bucket(illustrationsBucket).Cursor()

		i := 0
		k, v := cur.Last()
		for k != nil && i < rssEntryCount {
			var il Illust
			mustDecodeJSON(v, &il)

			fmt.Fprintf(w, `<item>`)
			fmt.Fprintf(w, `<title>%s - %s</title>`, html.EscapeString(il.Author), html.EscapeString(il.Title))
			desc := fmt.Sprintf(`<a rel="noreferrer" href="%s"><img src="thumbs/%s"><br>%s - %s</a>`,
				html.EscapeString(il.URL.String()), hex.EncodeToString(k), html.EscapeString(il.Author), html.EscapeString(il.Title))
			fmt.Fprintf(w, `<description>%s</description>`, html.EscapeString(desc))
			fmt.Fprintf(w, `<link>%s</link>`, html.EscapeString(il.URL.String()))
			fmt.Fprintf(w, `</item>`)

			i++
			k, v = cur.Prev()
		}

		return nil
	})

	fmt.Fprintf(w, `</channel></rss>`)
}
