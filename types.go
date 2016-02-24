package main

import "time"

type Illust struct {
	URL          string    `gorethink:"id"`
	Title        string    `gorethink:"title"`
	ThumbnailURL string    `gorethink:"thumbnailURL"`
	Author       string    `gorethink:"author"`
	FirstSeen    time.Time `gorethink:"firstSeen"`
}

type Thumbnail struct {
	URL  string `gorethink:"id"`
	Data []byte `gorethink:"data"`
}
