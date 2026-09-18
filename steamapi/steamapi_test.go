package steamapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
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

	client := NewClient("test-key")
	client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		steamID := req.URL.Query().Get("steamid")
		callsBySteamID[steamID]++
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

func TestSharedGamesHandlesSingleEmptyAndErrors(t *testing.T) {
	calls := 0
	client := NewClient("test-key")
	client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
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
