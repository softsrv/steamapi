package steamapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSharedGamesIntersectsByAppIDAndPreservesFirstGameData(t *testing.T) {
	responsesBySteamID := map[string]string{
		"one":   `{"response":{"games":[{"appid":10,"name":"First Name","playtime_forever":12,"img_icon_url":"first-icon","img_logo_url":"first-logo"},{"appid":20,"name":"Only First","playtime_forever":20,"img_icon_url":"only-icon","img_logo_url":"only-logo"}]}}`,
		"two":   `{"response":{"games":[{"appid":10,"name":"Second Name","playtime_forever":30,"img_icon_url":"second-icon","img_logo_url":"second-logo"},{"appid":30,"name":"Only Second"}]}}`,
		"three": `{"response":{"games":[{"appid":10,"name":"Third Name","playtime_forever":40,"img_icon_url":"third-icon","img_logo_url":"third-logo"}]}}`,
	}
	callsBySteamID := map[string]int{}
	var callsMu sync.Mutex

	client := NewClient("test-key")
	client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		steamID := req.URL.Query().Get("steamid")
		callsMu.Lock()
		callsBySteamID[steamID]++
		callsMu.Unlock()
		body, ok := responsesBySteamID[steamID]
		if !ok {
			t.Fatalf("unexpected steamID %q", steamID)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	games, err := client.SharedGames(context.Background(), []string{"one", "two", "three"})
	if err != nil {
		t.Fatalf("SharedGames returned error: %v", err)
	}

	if len(games) != 1 {
		t.Fatalf("expected 1 shared game, got %d: %#v", len(games), games)
	}
	if games[0].AppID != 10 {
		t.Fatalf("expected shared AppID 10, got %d", games[0].AppID)
	}
	if games[0].Name != "First Name" || games[0].ImgIconURL != "first-icon" || games[0].ImgLogoURL != "first-logo" || games[0].PlaytimeForever != 12 {
		t.Fatalf("expected first steamID's full Game data, got %#v", games[0])
	}
	for _, steamID := range []string{"one", "two", "three"} {
		if callsBySteamID[steamID] != 1 {
			t.Fatalf("expected Games to be called once for %s, got %d", steamID, callsBySteamID[steamID])
		}
	}
}

func TestSharedGamesConcurrentFetch(t *testing.T) {
	responsesBySteamID := map[string]string{
		"one":   `{"response":{"games":[{"appid":10,"name":"First Shared","playtime_forever":12,"img_icon_url":"first-shared-icon","img_logo_url":"first-shared-logo"},{"appid":20,"name":"First Only","playtime_forever":20,"img_icon_url":"first-only-icon","img_logo_url":"first-only-logo"},{"appid":30,"name":"Second Shared","playtime_forever":30,"img_icon_url":"second-shared-icon","img_logo_url":"second-shared-logo"}]}}`,
		"two":   `{"response":{"games":[{"appid":30,"name":"Two Second Shared"},{"appid":10,"name":"Two First Shared"},{"appid":40,"name":"Two Only"}]}}`,
		"three": `{"response":{"games":[{"appid":10,"name":"Three First Shared"},{"appid":30,"name":"Three Second Shared"}]}}`,
	}
	started := make(chan string, len(responsesBySteamID))
	release := make(chan struct{})
	var once sync.Once

	client := NewClient("test-key")
	client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		steamID := req.URL.Query().Get("steamid")
		body, ok := responsesBySteamID[steamID]
		if !ok {
			t.Fatalf("unexpected steamID %q", steamID)
		}

		started <- steamID
		<-release

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	done := make(chan struct{})
	var games []Game
	var err error
	go func() {
		defer close(done)
		games, err = client.SharedGames(context.Background(), []string{"one", "two", "three"})
	}()

	seen := map[string]bool{}
	for len(seen) < len(responsesBySteamID) {
		select {
		case steamID := <-started:
			seen[steamID] = true
			if len(seen) == len(responsesBySteamID) {
				once.Do(func() { close(release) })
			}
		case <-time.After(time.Second):
			once.Do(func() { close(release) })
			t.Fatalf("timed out waiting for concurrent SharedGames fetches; saw %v", seen)
		}
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for SharedGames to return")
	}

	if err != nil {
		t.Fatalf("SharedGames returned error: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("expected 2 shared games, got %d: %#v", len(games), games)
	}
	if games[0].AppID != 10 || games[1].AppID != 30 {
		t.Fatalf("expected first steamID order [10 30], got %#v", games)
	}
	if games[0].Name != "First Shared" || games[0].ImgIconURL != "first-shared-icon" || games[0].ImgLogoURL != "first-shared-logo" || games[0].PlaytimeForever != 12 {
		t.Fatalf("expected first shared game to retain first steamID's full Game data, got %#v", games[0])
	}
	if games[1].Name != "Second Shared" || games[1].ImgIconURL != "second-shared-icon" || games[1].ImgLogoURL != "second-shared-logo" || games[1].PlaytimeForever != 30 {
		t.Fatalf("expected second shared game to retain first steamID's full Game data, got %#v", games[1])
	}

	errClient := NewClient("test-key")
	errClient.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("steamid") == "two" {
			return nil, fmt.Errorf("games failed")
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(responsesBySteamID["one"])),
			Header:     make(http.Header),
		}, nil
	})}

	games, err = errClient.SharedGames(context.Background(), []string{"one", "two", "three"})
	if err == nil {
		t.Fatalf("expected Games error to be returned")
	}
	if games != nil {
		t.Fatalf("expected nil games on Games error, got %#v", games)
	}
}

func TestSharedGamesHandlesSingleEmptyAndErrors(t *testing.T) {
	calls := 0
	var callsMu sync.Mutex
	client := NewClient("test-key")
	client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callsMu.Lock()
		calls++
		callsMu.Unlock()
		if req.URL.Query().Get("steamid") == "error" {
			return nil, fmt.Errorf("games failed")
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"response":{"games":[{"appid":10,"name":"Solo","img_icon_url":"solo-icon"}]}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	games, err := client.SharedGames(context.Background(), []string{"solo"})
	if err != nil {
		t.Fatalf("SharedGames returned error for one steamID: %v", err)
	}
	if len(games) != 1 || games[0].AppID != 10 || games[0].Name != "Solo" || games[0].ImgIconURL != "solo-icon" {
		t.Fatalf("expected single steamID's full list, got %#v", games)
	}
	if calls != 1 {
		t.Fatalf("expected one Games call for one steamID, got %d", calls)
	}

	games, err = client.SharedGames(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error for no steamIDs")
	}
	if games != nil {
		t.Fatalf("expected nil games for no steamIDs, got %#v", games)
	}

	games, err = client.SharedGames(context.Background(), []string{"solo", "error"})
	if err == nil {
		t.Fatalf("expected Games error to be returned")
	}
	if games != nil {
		t.Fatalf("expected nil games on Games error, got %#v", games)
	}
}
