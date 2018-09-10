package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"github.com/boltdb/bolt"
	"log"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"strings"
	"sort"
	"time"
)

var apiKey string

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

type Store struct {
	db *bolt.DB
}

type Lastfm struct {
	context              *LastfmContext
	GetRecentTracksLimit int
	TotalPages           int
	lastLoadedPage       int
	lastLoadedRecords    *[]Record
	LoadUntilTimestamp   int
}

type Scan struct {
	RunTimestamp       int
	MaxRecordTimestamp int
	RecordsFound       int
	Username           string
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

type MbidEntity struct {
	Name string `json:"name"`
	Mbid string `json:"mbid"`
}

type RecordRest struct {
	Track     MbidEntity `json:"track"`
	Album     MbidEntity `json:"album"`
	Artist    MbidEntity `json:"artist"`
	Timestamp int        `json:"ts"`
}

type SystemStatus struct {
	Users []string `json:"users"`
}

func hasRecordsOlderThan(records *[]Record, ts int) bool {
	for _, record := range *records {
		if record.DateTimestamp < ts {
			return true
		}
	}

	return false
}

func filterRecordsOlderThen(records *[]Record, ts int) *[]Record {
	newRecords := make([]Record, 0)

	for _, record := range *records {
		if record.DateTimestamp > ts {
			newRecords = append(newRecords, record)
		}
	}

	return &newRecords
}

func (lastfm *Lastfm) scan() *[]Record {
	records := make([]Record, 0)

	for lastfm.hasNextPage() {
		pageRecords := *lastfm.loadNext()

		records = append(records, pageRecords...)
	}

	return filterRecordsOlderThen(&records, lastfm.LoadUntilTimestamp)
}

func (lastfm *Lastfm) hasNextPage() bool {
	if lastfm.lastLoadedPage == 0 {
		return true
	}

	if lastfm.lastLoadedPage > lastfm.TotalPages {
		return false
	}

	return !hasRecordsOlderThan(lastfm.lastLoadedRecords, lastfm.LoadUntilTimestamp)
}

func (lastfm *Lastfm) loadNext() *[]Record {
	page := lastfm.lastLoadedPage + 1

	return lastfm.loadPage(page)
}

func (lastfm *Lastfm) loadPage(page int) *[]Record {
	res, err := lastfm.context.getRecentTracks(page, lastfm.GetRecentTracksLimit)

	if err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}

	tracks := res.RecentTracks.Track
	records := make([]Record, 0)

	for _, track := range tracks {
		ts, _ := strconv.Atoi(track.Date.Uts)

		if ts == 0 {
			continue
		}

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

	lastPage, _ := strconv.Atoi(res.RecentTracks.Attr.Page)
	totalPages, _ := strconv.Atoi(res.RecentTracks.Attr.TotalPages)

	lastfm.lastLoadedPage = lastPage
	lastfm.lastLoadedRecords = &records
	lastfm.TotalPages = totalPages

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

func (store *Store) getUserBucketName(username string) []byte {
	name := fmt.Sprintf("user.%s", username)

	return []byte(name)
}

func (store *Store) getRecordsBucketName(username string) []byte {
	name := fmt.Sprintf("records.%s", username)

	return []byte(name)
}

func (store *Store) GetUsers() *[]string {
	users := make([]string, 0)

	store.db.View(func(tx *bolt.Tx) error {
		tx.ForEach(func(name []byte, bucket *bolt.Bucket) error {
			if strings.HasPrefix(string(name), "user.") {
				val := bucket.Get([]byte("scan"))

				if val != nil {
					var scan Scan
					json.Unmarshal(val, &scan)

					users = append(users, scan.Username)
				}
			}

			return nil
		})

		return nil
	})

	return &users
}

func (store *Store) GetRecords(username string) *[]Record {
	bucketName := store.getUserBucketName(username)

	var records []Record

	store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return nil
		}

		key := []byte("records")
		value := bucket.Get(key)

		if value != nil {
			json.Unmarshal(value, &records)
		}

		return nil
	})

	if records == nil {
		records = make([]Record, 0)
	}

	return &records
}

func (store *Store) UpdateRecords(username string, records *[]Record) error {
	bucketName := store.getUserBucketName(username)

	err := store.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)

		if err != nil {
			return err
		}

		var savedRecords []Record
		key := []byte("records")
		value := bucket.Get(key)

		if value == nil {
			savedRecords = make([]Record, 0)
		} else {
			json.Unmarshal(value, &savedRecords)
		}

		newRecords := append(savedRecords, *records...)
		saveValue, err := json.Marshal(newRecords)

		err = bucket.Put(key, saveValue)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (store *Store) GetLastScan(username string) *Scan {
	key := []byte("scan")

	bucketName := store.getUserBucketName(username)
	var scan Scan

	err := store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)

		if bucket == nil {
			return nil
		}

		val := bucket.Get(key)
		json.Unmarshal(val, &scan)

		return nil
	})

	if err != nil {
		return nil
	}

	return &scan
}

func (store *Store) SetLastScan(username string, scan *Scan) error {
	bucketName := store.getUserBucketName(username)

	// store some data
	err := store.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}

		key := []byte("scan")
		value, err := json.Marshal(scan)

		err = bucket.Put(key, value)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func openDb() (*bolt.DB, error) {
	db, err := bolt.Open("stat.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	return db, nil
}

func getScanInfo(records *[]Record) *Scan {
	maxTs := 0

	for _, record := range *records {
		if record.DateTimestamp > maxTs {
			maxTs = record.DateTimestamp
		}
	}

	return &Scan{
		MaxRecordTimestamp: maxTs,
		RecordsFound:       len(*records),
	}
}

func getSystemStatus() (*SystemStatus, error) {
	db, err := openDb()
	defer db.Close()

	if err != nil {
		return nil, err
	}

	store := Store{
		db: db,
	}

	users := store.GetUsers()
	status := &SystemStatus{
		Users: *users,
	}

	return status, nil
}

func (record *Record) toRestRecord() *RecordRest {
	return &RecordRest{
		Timestamp: record.DateTimestamp,
		Track: MbidEntity{
			Name: record.Track,
			Mbid: record.TrackMbid,
		},
		Album: MbidEntity{
			Name: record.Album,
			Mbid: record.AlbumMbid,
		},
		Artist: MbidEntity{
			Name: record.Artist,
			Mbid: record.ArtistMbid,
		},
	}
}

func getUserRecords(username string) (*[]RecordRest, error) {
	db, err := openDb()
	defer db.Close()

	if err != nil {
		return nil, err
	}

	store := Store{
		db: db,
	}

	records := *store.GetRecords(username)

	sort.Slice(records, func(i, j int) bool {
		return records[i].DateTimestamp > records[j].DateTimestamp
	})

	restRecords := make([]RecordRest, 0)

	for _, record := range records {
		restRecords = append(restRecords, *record.toRestRecord())
	}

	return &restRecords, nil
}

func getUserScan(username string) (*Scan, error) {
	db, err := openDb()
	defer db.Close()

	if err != nil {
		return nil, err
	}

	store := Store{
		db: db,
	}

	return store.GetLastScan(username), nil
}

func runUser(username string) {
	context := &LastfmContext{
		User:   username,
		ApiKey: apiKey,
	}
	db, err := openDb()
	defer db.Close()

	if err != nil {
		panic(err)
	}

	store := Store{
		db: db,
	}

	lastScan := store.GetLastScan(username)

	var until int
	if lastScan == nil {
		until = 1536005795
	} else {
		until = lastScan.MaxRecordTimestamp
	}

	//until = 1536005795

	lastfm := Lastfm{
		context:              context,
		GetRecentTracksLimit: 200,
		LoadUntilTimestamp:   until,
	}

	records := lastfm.scan()
	scan := getScanInfo(records)
	scan.RunTimestamp = int(time.Now().Unix())
	scan.Username = username

	if scan.RecordsFound != 0 {
		store.SetLastScan(username, scan)
		store.UpdateRecords(username, records)
	}
}

func startServer(address string) {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/status", handleStatus)
	e.GET("/user/:username/records", handleUserRecords)
	e.GET("/user/:username/status", handleUserStatus)

	// Start server
	e.Logger.Fatal(e.Start(address))
}

func getErrorMessage(message string) interface{} {
	m := make(map[string]string)
	m["error"] = message

	return m
}

func handleStatus(c echo.Context) error {
	status, err := getSystemStatus()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, getErrorMessage("Cannon get status"))
	}

	return c.JSON(http.StatusOK, status)
}

func handleUserRecords(c echo.Context) error {
	username := c.Param("username")

	records, err := getUserRecords(username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, getErrorMessage("Cannot get user records"))
	}

	return c.JSON(http.StatusOK, records)
}

func handleUserStatus(c echo.Context) error {
	username := c.Param("username")

	scan, err := getUserScan(username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, getErrorMessage("Cannot get user status"))
	}

	out := make(map[string]interface{})
	out["lastScanRecordsFound"] = scan.RecordsFound
	out["lastScanTimestamp"] = scan.RunTimestamp
	out["lastScanMaxRecordTimestamp"] = scan.MaxRecordTimestamp

	return c.JSON(http.StatusOK, out)
}

func runSyncLoop() {
	for {
		status, err := getSystemStatus()
		if err != nil {
			return
		}

		for _, username := range status.Users {
			fmt.Println("Update", username)

			runUser(username)
		}

		time.Sleep(10 * time.Second)
	}
}

func main() {
	apiKey = "e8162414f5faf07f1958ee934709cc9d"

	go runSyncLoop()
	startServer(":8000")
	//go runSyncLoop()
	//go runSyncLoop()

	//username := "mrpoma"
	//runUser(username)
}
