package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/boltdb/bolt"
)

var thumbnailReadyCh = make(chan struct{}, 1)

func init() {
	go func() {
		for range thumbnailReadyCh {
			getThumbnails()
		}
	}()
}

func pollThumbnailGrabber() {
	select {
	case thumbnailReadyCh <- struct{}{}:
	default:
	}
}

func getThumbnails() {
	thumbsToGet := make(map[uint64]string)
	datafile.View(func(tx *bolt.Tx) error {
		illusts := tx.Bucket(illustrationsBucket)
		thumbs := tx.Bucket(thumbnailsBucket)

		illusts.ForEach(func(k, v []byte) error {
			if thumbs.Get(k) == nil {
				var il Illust
				mustDecodeJSON(v, &il)
				thumbsToGet[binary.BigEndian.Uint64(k)] = il.ThumbnailURL.String()
			}

			return nil
		})

		return nil
	})

	for id, url := range thumbsToGet {
		getThumbnail(id, url)
	}
}

func getThumbnail(id uint64, url string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Referer", "http://www.pixiv.net/bookmark_new_illust.php")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Couldn't GET %v: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Printf("Couldn't GET %v: unexpected status %v\n", url, resp.Status)
		return
	}

	data, err := ioutil.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		fmt.Printf("Couldn't GET %v: %v\n", url, err)
		return
	}

	if len(data) == 1024*1024 {
		fmt.Printf("Couldn't GET %v: response body too large\n", url)
		return
	}

	if len(data) == 0 {
		fmt.Printf("Couldn't GET %v: response was empty\n", url)
		return
	}

	datafile.Update(func(tx *bolt.Tx) error {
		thumbs := tx.Bucket(thumbnailsBucket)

		var idBuf [8]byte
		binary.BigEndian.PutUint64(idBuf[:], id)

		thumbs.Put(idBuf[:], data)

		return nil
	})

	fmt.Printf("Added thumbnail %v from %v\n", id, url)
}
