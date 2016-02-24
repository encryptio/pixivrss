package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

func getThumbnails() {
	changes := make(chan *Illust)
	go illustrationChangesInto(changes)
	for il := range changes {
		if il.ThumbnailURL == "" {
			continue
		}

		has, err := thumbnailExists(il.ThumbnailURL)
		if has || err != nil {
			continue
		}

		data, err := downloadThumbnail(il.ThumbnailURL)
		if err != nil {
			log.Printf("Couldn't download thumbnail: %v", err)
			continue
		}

		err = insertThumbnail(&Thumbnail{
			URL:  il.ThumbnailURL,
			Data: data,
		})
		if err != nil {
			log.Printf("Couldn't insert thumbnail: %v", err)
			continue
		}

		fmt.Printf("Added thumbnail %v\n", il.ThumbnailURL)
	}
}

func downloadThumbnail(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Referer", "http://www.pixiv.net/bookmark_new_illust.php")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Couldn't GET %v: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Couldn't GET %v: unexpected status %v", url, resp.Status)
	}

	data, err := ioutil.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("Couldn't GET %v: %v", url, err)
	}

	if len(data) == 1024*1024 {
		return nil, fmt.Errorf("Couldn't GET %v: response body too large", url)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("Couldn't GET %v: response was empty", url)
	}

	return data, nil
}
