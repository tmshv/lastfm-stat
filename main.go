package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
)

var storageDir string

type Size struct {
	width  float64
	height float64
}

type LastfmContext struct {
	User   string
	ApiKey string
	Limit  int
}

type LastfmResponse struct {
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

func (ctx *LastfmContext) getUrl(page int) string {
	p := strconv.Itoa(page)
	l := strconv.Itoa(ctx.Limit)
	base := "http://ws.audioscrobbler.com/2.0"
	method := "user.getrecenttracks"
	path := "/?method=" + method + "&user=" + ctx.User + "&api_key=" + ctx.ApiKey + "&page=" + p + "&limit=" + l + "&format=json"

	return base + path
}

func lastfmRequest(url string) (*LastfmResponse, error) {
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

		res := LastfmResponse{}
		bytes := []byte(contents)
		json.Unmarshal(bytes, &res)

		return &res, nil
		// fmt.Printf("%s\n", string(contents))
	}
}

func makeRequest(lastfm *LastfmContext, page int, ch chan<- string) {
	url := lastfm.getUrl(page)
	res, err := lastfmRequest(url)

	if err != nil {
		fmt.Printf("%s", err)
		// 	os.Exit(1)
	}

	tracks := res.RecentTracks.Track
	for _, track := range tracks {
		out := fmt.Sprintf("%s000,%s,%s,%s", track.Date.Uts, track.Artist.Text, track.Name, track.Album.Text)

		ch <- out
	}
}

func main() {
	lastfm := LastfmContext{
		User:   "mrpoma",
		ApiKey: "e8162414f5faf07f1958ee934709cc9d",
		Limit:  200,
	}
	ch := make(chan string)
	total := 299

	for i := 1; i < total+1; i++ {
		go makeRequest(&lastfm, i, ch)
	}

	length := total * lastfm.Limit
	// for {
	for i := 0; i < length; i++ {
		row, more := <-ch
		if more {
			fmt.Println(row)
		} else {
			return
		}
	}

	// res, err := lastfmRequest(url)

	// if err != nil {
	// 	fmt.Printf("%s", err)
	// 	os.Exit(1)
	// }

	// fmt.Printf("%s\n", res.RecentTracks.Attr.User)
	// fmt.Printf("%s\n", res.RecentTracks.Attr.TotalPages)

	// response, err := http.Get(url)
	// if err != nil {
	// 	fmt.Printf("%s", err)
	// 	os.Exit(1)
	// } else {
	// 	defer response.Body.Close()
	// 	contents, err := ioutil.ReadAll(response.Body)
	// 	if err != nil {
	// 		fmt.Printf("%s", err)
	// 		os.Exit(1)
	// 	}
	// 	fmt.Printf("%s\n", string(contents))
	// }
}

func save(file *multipart.FileHeader, path string) error {
	// Source
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// Destination
	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy
	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	return nil
}
