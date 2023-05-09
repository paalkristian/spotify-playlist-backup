package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

const (
	authURL        = "https://accounts.spotify.com/authorize"
	tokenURL       = "https://accounts.spotify.com/api/token"
	baseAPIAddress = "https://api.spotify.com"
)

var (
	redirectURL = "http://localhost:8080/callback"
	scopes      = []string{"playlist-read-private", "user-library-read"}
)

type Playlist struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type PlaylistPage struct {
	Items    []Playlist `json:"items"`
	Href     string     `json:"href"`
	Limit    int        `json:"limit"`
	Next     string     `json:"next"`
	Offset   int        `json:"offset"`
	Previous string     `json:"previous"`
	Total    int        `json:"total"`
}

type TracksPage struct {
	Items    []Item `json:"items"`
	Href     string `json:"href"`
	Limit    int    `json:"limit"`
	Next     string `json:"next"`
	Offset   int    `json:"offset"`
	Previous string `json:"previous"`
	Total    int    `json:"total"`
}

type Item struct {
	AddedAt string `json:"added_at"`
	Track   Track  `json:"track"`
}

type Track struct {
	Album        Album       `json:"album"`
	Artists      []Artist    `json:"artists"`
	DiscNumber   int         `json:"disc_number"`
	DurationMs   int         `json:"duration_ms"`
	Explicit     bool        `json:"explicit"`
	ExternalIds  ExternalId  `json:"external_ids"`
	ExternalUrls ExternalUrl `json:"external_urls"`
	Href         string      `json:"href"`
	Id           string      `json:"id"`
	IsLocal      bool        `json:"is_local"`
	Name         string      `json:"name"`
	Popularity   int         `json:"popularity"`
	PreviewUrl   string      `json:"preview_url"`
	TrackNumber  int         `json:"track_number"`
	Type         string      `json:"type"`
	Uri          string      `json:"uri"`
}

type Album struct {
	AlbumGroup           string      `json:"album_group"`
	AlbumType            string      `json:"album_type"`
	Artists              []Artist    `json:"artists"`
	ExternalUrls         ExternalUrl `json:"external_urls"`
	Href                 string      `json:"href"`
	Id                   string      `json:"id"`
	Images               []Image     `json:"images"`
	Name                 string      `json:"name"`
	ReleaseDate          string      `json:"release_date"`
	ReleaseDatePrecision string      `json:"release_date_precision"`
	TotalTracks          int         `json:"total_tracks"`
	Type                 string      `json:"type"`
	Uri                  string      `json:"uri"`
}

type Artist struct {
	ExternalUrls ExternalUrl `json:"external_urls"`
	Href         string      `json:"href"`
	Id           string      `json:"id"`
	Name         string      `json:"name"`
	Type         string      `json:"type"`
	Uri          string      `json:"uri"`
}

type ExternalUrl struct {
	Spotify string `json:"spotify"`
}

type ExternalId struct {
	Isrc string `json:"isrc"`
}

type Image struct {
	Height int    `json:"height"`
	Url    string `json:"url"`
	Width  int    `json:"width"`
}

// Helper functions

func oauthFlow(ctx context.Context, conf *oauth2.Config) *oauth2.Token {
	// Start OAuth flow.
	state := "random-string-for-state-check"

	url := conf.AuthCodeURL(state)

	fmt.Printf("Visit the following URL to authorize the app: \n%v\n", url)

	// Start callback server.
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		code := query.Get("code")
		receivedState := query.Get("state")

		if state != receivedState {
			log.Fatalf("Invalid state received: %s", receivedState)
		}

		token, err := conf.Exchange(ctx, code)
		if err != nil {
			log.Fatalf("Error exchanging authorization code: %v", err)
		}

		saveToken(token)
		fmt.Fprintf(w, "Authorization successful. You can close this window.")
		os.Exit(0)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))

	// The code execution should not reach here.
	return nil
}

func loadToken() (*oauth2.Token, error) {
	file, err := ioutil.ReadFile("token_cache.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read token cache file")
	}

	var token oauth2.Token
	err = json.Unmarshal(file, &token)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal token")
	}

	return &token, nil
}

func saveToken(token *oauth2.Token) {
	data, err := json.Marshal(token)
	if err != nil {
		log.Fatalf("Error marshaling token: %v", err)
	}

	err = ioutil.WriteFile("token_cache.json", data, 0600)
	if err != nil {
		log.Fatalf("Error saving token cache: %v", err)
	}
}

func fetchPlaylists(client *http.Client) ([]Playlist, error) {
	limit := 50
	playlists := make([]Playlist, 0)
	nextPageUrl := fmt.Sprintf("%s/v1/me/playlists?offset=0&limit=%d", baseAPIAddress, limit)

	for nextPageUrl != "" {
		resp, err := client.Get(nextPageUrl)
		if err != nil {
			resp.Body.Close()
			return nil, errors.Wrap(err, "failed to fetch playlists")
		}
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read playlists response")
		}

		var page PlaylistPage
		json.Unmarshal(data, &page)
		playlists = append(playlists, page.Items...)
		fmt.Printf("Fetched %d playlists\n", len(playlists))
		nextPageUrl = page.Next
	}

	return playlists, nil
}

func fetchPlaylistTracks(client *http.Client, playlist Playlist) ([]Item, error) {
	limit := 100
	tracks := make([]Item, 0)
	nextPageUrl := fmt.Sprintf("%s/v1/playlists/%s/tracks?offset=0&limit=%d", baseAPIAddress, playlist.Id, limit)

	for nextPageUrl != "" {
		resp, err := client.Get(nextPageUrl)
		if err != nil {
			resp.Body.Close()
			return nil, errors.Wrapf(err, "failed to fetch tracks for playlist %s", playlist.Name)
		}

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, errors.Wrap(err, "failed to read tracks response")
		}

		var page TracksPage
		json.Unmarshal(data, &page)
		tracks = append(tracks, page.Items...)

		fmt.Printf("Fetched %d tracks for playlist %s. Total tracks: %d\n", len(page.Items), playlist.Name, len(tracks))
		nextPageUrl = page.Next
	}
	return tracks, nil
}

func fetchSavedTracks(client *http.Client) ([]Item, error) {
	limit := 50
	tracks := make([]Item, 0)

	nextPageUrl := fmt.Sprintf("%s/v1/me/tracks?offset=0&limit=%d", baseAPIAddress, limit)

	for {
		resp, err := client.Get(nextPageUrl)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch saved tracks")
		}
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, errors.Wrap(err, "failed to read saved tracks response")
		}
		var savedTracksPage TracksPage
		json.Unmarshal(data, &savedTracksPage)
		tracks = append(tracks, savedTracksPage.Items...)

		if len(savedTracksPage.Items) < limit {
			break
		}
		fmt.Printf("Fetched %d saved tracks\n", len(tracks))
		nextPageUrl = savedTracksPage.Next
		resp.Body.Close()
	}

	return tracks, nil
}

func saveJSONToFile(name string, data interface{}) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling JSON data: %v", err)
	}

	backupFolder := "backups"
	if _, err := os.Stat(backupFolder); os.IsNotExist(err) {
		err = os.Mkdir(backupFolder, 0755)
		if err != nil {
			log.Fatalf("Error creating backups folder: %v", err)
		}
	}

	cleanedFilename := filepath.Clean(name)
	safeFilename := regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(cleanedFilename, "-")

	filename := fmt.Sprintf("%s/%s.json", backupFolder, safeFilename)
	err = ioutil.WriteFile(filename, jsonData, 0644)
	if err != nil {
		log.Fatalf("Error writing JSON data to file: %v", err)
	}
}

func main() {
	// Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	conf := &oauth2.Config{
		ClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		ClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}

	ctx := context.Background()

	// Load cached token or start OAuth flow.
	token, err := loadToken()
	if err != nil {
		token = oauthFlow(ctx, conf)
		saveToken(token)
	}

	client := conf.Client(ctx, token)

	// Fetch playlists.
	playlists, err := fetchPlaylists(client)
	if err != nil {
		log.Fatalf("Error fetching playlists: %v", err)
	}

	// Fetch and save tracks for each playlist.
	for _, p := range playlists {
		tracks, err := fetchPlaylistTracks(client, p)
		if err != nil {
			log.Printf("Error fetching tracks for playlist %s: %v", p.Name, err)
			continue
		}

		saveJSONToFile(p.Name, tracks)
		time.Sleep(2 * time.Second) // Avoid rate limiting. Can probably be tuned
	}

	// Fetch saved tracks.
	savedTracks, err := fetchSavedTracks(client)
	if err != nil {
		log.Fatalf("Error fetching saved tracks: %v", err)
	}

	saveJSONToFile("saved_tracks", savedTracks)
}
