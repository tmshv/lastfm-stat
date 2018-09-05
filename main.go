package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
)

type LastfmRecentTracksResponse struct {
	RecentTracks struct {
		Track []struct {
			Artist struct {
				Text string `json:"#text"`
				Mbid string `json:"mbid"`
			} `json:"artist"`
			Name       string `json:"name"`
			Streamable string `json:"streamable"`
			Mbid       string `json:"mbid"`
			Album      struct {
				Text string `json:"#text"`
				Mbid string `json:"mbid"`
			} `json:"album"`
			URL   string `json:"url"`
			Image []struct {
				Text string `json:"#text"`
				Size string `json:"size"`
			} `json:"image"`
			Date struct {
				Uts  string `json:"uts"`
				Text string `json:"#text"`
			} `json:"date"`
		} `json:"track"`
		Attr struct {
			User       string `json:"user"`
			Page       string `json:"page"`
			PerPage    string `json:"perPage"`
			TotalPages string `json:"totalPages"`
			Total      string `json:"total"`
		} `json:"@attr"`
	} `json:"recenttracks"`
}

type LastfmContext struct {
	User   string
	ApiKey string
}

type Lastfm struct {
	context              *LastfmContext
	GetRecentTracksLimit int
	LastLoadedPage       int
	TotalPages           int
}

type Record struct {
	Track         string
	TrackMbid     string
	Album         string
	AlbumMbid     string
	Artist        string
	ArtistMbid    string
	Date          string
	DateTimestamp int
}

func (lastfm *Lastfm) hasNextPage() bool {
	if lastfm.LastLoadedPage == 0 {
		return true
	}

	return lastfm.LastLoadedPage < lastfm.TotalPages
}

func (lastfm *Lastfm) loadNext() *[]Record {
	page := lastfm.LastLoadedPage + 1

	return lastfm.loadPage(page)
}

func (lastfm *Lastfm) loadPage(page int) *[]Record {
	res, err := lastfm.context.getRecentTracks(page, lastfm.GetRecentTracksLimit)

	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}

	lastPage, _ := strconv.Atoi(res.RecentTracks.Attr.Page)
	totalPages, _ := strconv.Atoi(res.RecentTracks.Attr.TotalPages)
	lastfm.LastLoadedPage = lastPage
	lastfm.TotalPages = totalPages

	tracks := res.RecentTracks.Track
	records := make([]Record, 0)

	for _, track := range tracks {
		ts, _ := strconv.Atoi(track.Date.Uts)
		stream := track.Streamable == "1"

		if !stream {
			record := &Record{
				DateTimestamp: ts,
				Date:          track.Date.Text,
				Track:         track.Name,
				Album:         track.Album.Text,
				Artist:        track.Artist.Text,
				AlbumMbid:     track.Album.Mbid,
				ArtistMbid:    track.Artist.Mbid,
				TrackMbid:     track.Mbid,
			}

			records = append(records, *record)
		}
	}

	return &records
}

func (ctx *LastfmContext) getRecentTracks(page, limit int) (*LastfmRecentTracksResponse, error) {
	url := ctx.getRecentTracksUrl(page, limit)
	response, err := http.Get(url)
	if err != nil {
		fmt.Printf("%s", err)

		return nil, err
	} else {
		defer response.Body.Close()
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}

		res := LastfmRecentTracksResponse{}
		bytes := []byte(contents)
		json.Unmarshal(bytes, &res)

		return &res, nil
	}
}

func (ctx *LastfmContext) getRecentTracksUrl(page, limit int) string {
	p := strconv.Itoa(page)
	l := strconv.Itoa(limit)
	base := "http://ws.audioscrobbler.com/2.0"
	method := "user.getrecenttracks"
	path := "/?method=" + method + "&user=" + ctx.User + "&api_key=" + ctx.ApiKey + "&page=" + p + "&limit=" + l + "&format=json"

	return base + path
}

func main() {
	context := &LastfmContext{
		User:   "mrpoma",
		ApiKey: "e8162414f5faf07f1958ee934709cc9d",
	}
	lastfm := Lastfm{
		context:              context,
		GetRecentTracksLimit: 200,
	}
	records := make([]Record, 0)

	for lastfm.hasNextPage() {
		pageRecords := *lastfm.loadNext()

		records = append(records, pageRecords...)

		fmt.Println("Page:", lastfm.LastLoadedPage)
		fmt.Println("Total pages:", lastfm.TotalPages)
		fmt.Println("Records found:", len(pageRecords))

		for _, record := range pageRecords {
			fmt.Println(record)
		}

		fmt.Println("")
	}
}
