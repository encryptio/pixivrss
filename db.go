package main

import (
	"log"
	"time"

	r "github.com/dancannon/gorethink"
)

var db *r.Session

var (
	illustrations = r.Table("illustrations")
	thumbnails    = r.Table("thumbnails")
)

func openDB() {
	var err error
	db, err = r.Connect(r.ConnectOpts{
		Address:  config.DBAddress,
		Database: config.DBName,
	})
	if err != nil {
		die("Couldn't connect to rethinkdb at %v: %v", config.DBAddress, err)
	}

	for _, tableName := range []string{"illustrations", "thumbnails"} {
		cur, err := r.Branch(r.TableList().Contains(tableName),
			r.Table(tableName).Wait(),
			r.TableCreate(tableName)).Run(db)
		if err != nil {
			die("Couldn't ensure table %v exists: %v", tableName, err)
		}
		cur.Close()
	}

	cur, err := r.Branch(illustrations.IndexList().Contains("firstSeen"),
		illustrations.IndexWait("firstSeen"),
		illustrations.IndexCreate("firstSeen")).Run(db)
	if err != nil {
		die("Couldn't ensure index on illustrations: %v", err)
	}
	cur.Close()
}

func handleIllust(il Illust) {
	cur, err := illustrations.Get(il.URL).Ne(nil).Run(db)
	if err != nil {
		log.Printf("Couldn't run query: %v", err)
		return
	}
	defer cur.Close()

	var exists bool
	err = cur.One(&exists)
	if err != nil {
		log.Printf("Couldn't run query: %v", err)
		return
	}
	cur.Close()

	if exists {
		return
	}

	_, err = illustrations.Insert(il).RunWrite(db)
	if err != nil {
		log.Printf("Couldn't insert illustration %v: %v", il, err)
		return
	}

	log.Printf("Found new illustration:\n")
	log.Printf("Title: %v\n", il.Title)
	log.Printf("Author: %v\n", il.Author)
	log.Printf("URL: %v\n", il.URL)
	log.Printf("Thumbnail URL: %v\n", il.ThumbnailURL)
}

func recentIllustrations() ([]Illust, error) {
	cur, err := illustrations.OrderBy(r.OrderByOpts{Index: r.Desc("firstSeen")}).Limit(config.RespondCount).Run(db)
	if err != nil {
		return nil, err
	}

	res := make([]Illust, 0, config.RespondCount)
	var il Illust
	for cur.Next(&il) {
		res = append(res, il)
	}
	cur.Close()
	err = cur.Err()

	return res, err
}

func illustrationChangesInto(ch chan<- *Illust) {
	for {
		cur, err := illustrations.Changes(r.ChangesOpts{IncludeInitial: true}).Field("new_val").Filter(r.Row.Ne(nil)).Run(db)
		if err != nil {
			log.Printf("Couldn't start changefeed for illustrations: %v", err)
			time.Sleep(time.Minute)
			continue
		}

		subCh := make(chan *Illust)
		cur.Listen(subCh)
		for il := range subCh {
			ch <- il
		}
		cur.Close()

		err = cur.Err()
		if err != nil {
			log.Printf("Hit end of infinite cursor: %v", err)
		} else {
			log.Printf("Hit end of infinite cursor (but no error)")
		}

		time.Sleep(time.Second)
	}
}

func thumbnailExists(url string) (bool, error) {
	cur, err := thumbnails.Get(url).Ne(nil).Run(db)
	if err != nil {
		return false, err
	}
	var out bool
	err = cur.One(&out)
	cur.Close()
	return out, err
}

func getThumbnail(url string) (*Thumbnail, error) {
	cur, err := thumbnails.Get(url).Run(db)
	if err != nil {
		return nil, err
	}
	var out *Thumbnail
	err = cur.One(&out)
	cur.Close()
	return out, err
}

func insertThumbnail(t *Thumbnail) error {
	_, err := thumbnails.Insert(t).RunWrite(db)
	return err
}
