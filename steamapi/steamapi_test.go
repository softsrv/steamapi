package steamapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSharedGamesWithOneOtherIDReturnsSharedGames(t *testing.T) {
	ctx := context.Background()
	want := []Game{{AppID: 2, Name: "Shared"}}
	client := &Client{gamesFn: stubGames(t, map[string][]Game{
		"caller": {
			{AppID: 1, Name: "Caller Only"},
			{AppID: 2, Name: "Shared"},
		},
		"friend": {
			{AppID: 2, Name: "Shared"},
			{AppID: 3, Name: "Friend Only"},
		},
	}, nil)}

	got, err := client.SharedGames(ctx, "caller", "friend")
	if err != nil {
		t.Fatalf("SharedGames() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SharedGames() = %#v, want %#v", got, want)
	}
}

func TestSharedGamesReturnsUnderlyingErrorBeforeNoGamesError(t *testing.T) {
	ctx := context.Background()
	transportErr := errors.New("transport failed")
	client := &Client{gamesFn: stubGames(t, map[string][]Game{
		"caller": {
			{AppID: 1, Name: "Caller Game"},
		},
		"empty-friend": {},
		"error-friend": {
			{AppID: 2, Name: "Unreached Game"},
		},
	}, map[string]error{
		"error-friend": transportErr,
	})}

	games, err := client.SharedGames(ctx, "caller", "empty-friend", "error-friend")
	if !errors.Is(err, transportErr) {
		t.Fatalf("SharedGames() error = %v, want underlying transport error %v", err, transportErr)
	}
	if err != transportErr {
		t.Fatalf("SharedGames() error identity = %p, want %p", err, transportErr)
	}
	var noGamesErr *NoGamesError
	if errors.As(err, &noGamesErr) {
		t.Fatalf("SharedGames() error matched NoGamesError = %#v, want plain transport error", noGamesErr)
	}
	if games != nil {
		t.Fatalf("SharedGames() games = %#v, want nil on error", games)
	}

	allSucceedClient := &Client{gamesFn: stubGames(t, map[string][]Game{
		"caller": {
			{AppID: 1, Name: "Caller Game"},
		},
		"empty-friend": {},
		"error-friend": {
			{AppID: 2, Name: "Now Successful"},
		},
	}, nil)}
	_, err = allSucceedClient.SharedGames(ctx, "caller", "empty-friend", "error-friend")
	if errors.Is(err, transportErr) {
		t.Fatalf("SharedGames() all-succeed error matched transportErr, want different branch")
	}
	noGamesErr = nil
	if !errors.As(err, &noGamesErr) {
		t.Fatalf("SharedGames() all-succeed error = %v, want NoGamesError", err)
	}
}

func TestSharedGamesAllSucceedOneEmptyYieldsNoGamesError(t *testing.T) {
	ctx := context.Background()
	client := &Client{gamesFn: stubGames(t, map[string][]Game{
		"caller": {
			{AppID: 1, Name: "Caller Game"},
		},
		"empty-friend": {},
	}, nil)}

	games, err := client.SharedGames(ctx, "caller", "empty-friend")
	if games != nil {
		t.Fatalf("SharedGames() games = %#v, want nil", games)
	}
	var noGamesErr *NoGamesError
	if !errors.As(err, &noGamesErr) {
		t.Fatalf("SharedGames() error = %v, want NoGamesError", err)
	}
	wantSteamIDs := []string{"empty-friend"}
	if !reflect.DeepEqual(noGamesErr.SteamIDs, wantSteamIDs) {
		t.Fatalf("NoGamesError.SteamIDs = %#v, want %#v", noGamesErr.SteamIDs, wantSteamIDs)
	}
}

func TestNoGamesErrorCarriesSteamIDsAndSupportsErrorsAs(t *testing.T) {
	wantSteamIDs := []string{"caller", "friend"}
	err := error(&NoGamesError{SteamIDs: wantSteamIDs})
	if err.Error() == "" {
		t.Fatal("NoGamesError.Error() = empty string, want descriptive error")
	}

	var noGamesErr *NoGamesError
	if !errors.As(err, &noGamesErr) {
		t.Fatalf("errors.As(%T) = false, want true", err)
	}
	if !reflect.DeepEqual(noGamesErr.SteamIDs, wantSteamIDs) {
		t.Fatalf("NoGamesError.SteamIDs = %#v, want %#v", noGamesErr.SteamIDs, wantSteamIDs)
	}
}

func TestSharedGamesNoGamesErrorCarriesExactlyEmptyUsersIncludingCaller(t *testing.T) {
	ctx := context.Background()
	client := &Client{gamesFn: stubGames(t, map[string][]Game{
		"caller": {},
		"friend1": {
			{AppID: 1, Name: "Friend One Game"},
		},
		"friend2": {},
	}, nil)}

	_, err := client.SharedGames(ctx, "caller", "friend1", "friend2")
	var noGamesErr *NoGamesError
	if !errors.As(err, &noGamesErr) {
		t.Fatalf("SharedGames() error = %v, want NoGamesError", err)
	}
	wantSteamIDs := []string{"caller", "friend2"}
	if !reflect.DeepEqual(noGamesErr.SteamIDs, wantSteamIDs) {
		t.Fatalf("NoGamesError.SteamIDs = %#v, want exactly %#v", noGamesErr.SteamIDs, wantSteamIDs)
	}
}

func TestSharedGamesReturnsStrictIntersectionInCallerOrder(t *testing.T) {
	ctx := context.Background()
	client := &Client{gamesFn: stubGames(t, map[string][]Game{
		"caller": {
			{AppID: 1, Name: "A"},
			{AppID: 2, Name: "B"},
			{AppID: 3, Name: "C"},
		},
		"friend1": {
			{AppID: 2, Name: "B"},
			{AppID: 3, Name: "C"},
			{AppID: 4, Name: "D"},
		},
		"friend2": {
			{AppID: 3, Name: "C"},
			{AppID: 2, Name: "B"},
			{AppID: 5, Name: "E"},
		},
	}, nil)}

	got, err := client.SharedGames(ctx, "caller", "friend1", "friend2")
	if err != nil {
		t.Fatalf("SharedGames() error = %v, want nil", err)
	}
	want := []Game{{AppID: 2, Name: "B"}, {AppID: 3, Name: "C"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SharedGames() = %#v, want strict intersection %#v", got, want)
	}
}

func TestSharedGamesExcludesGamesOwnedByOnlySubset(t *testing.T) {
	ctx := context.Background()
	client := &Client{gamesFn: stubGames(t, map[string][]Game{
		"caller": {
			{AppID: 1, Name: "Caller And One Friend"},
			{AppID: 2, Name: "Owned By All"},
		},
		"friend1": {
			{AppID: 1, Name: "Caller And One Friend"},
			{AppID: 2, Name: "Owned By All"},
		},
		"friend2": {
			{AppID: 2, Name: "Owned By All"},
		},
	}, nil)}

	got, err := client.SharedGames(ctx, "caller", "friend1", "friend2")
	if err != nil {
		t.Fatalf("SharedGames() error = %v, want nil", err)
	}
	want := []Game{{AppID: 2, Name: "Owned By All"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SharedGames() = %#v, want %#v without subset-only game", got, want)
	}
	for _, game := range got {
		if game.AppID == 1 {
			t.Fatalf("SharedGames() included subset-only game %#v", game)
		}
	}
}

func stubGames(t *testing.T, gamesBySteamID map[string][]Game, errorsBySteamID map[string]error) func(context.Context, string) ([]Game, error) {
	t.Helper()
	return func(_ context.Context, steamID string) ([]Game, error) {
		if err := errorsBySteamID[steamID]; err != nil {
			return nil, err
		}
		games, ok := gamesBySteamID[steamID]
		if !ok {
			t.Fatalf("unexpected Games call for Steam ID %q", steamID)
		}
		return games, nil
	}
}

// newTestClient starts a mock server, points the package baseURL at it for
// the duration of the test, and returns a Client configured to use it.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	original := baseURL
	baseURL = server.URL
	t.Cleanup(func() { baseURL = original })

	return NewClient("test-api-key")
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to encode test response: %v", err)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("my-key")
	if c.apiKey != "my-key" {
		t.Errorf("expected apiKey %q, got %q", "my-key", c.apiKey)
	}
	if c.client == nil {
		t.Error("expected http client to be initialized")
	}
}

func TestPlayers_Success(t *testing.T) {
	var gotPath, gotSteamIDs, gotKey string

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSteamIDs = r.URL.Query().Get("steamids")
		gotKey = r.URL.Query().Get("key")

		writeJSON(t, w, PlayersResult{
			Response: PlayersList{
				Players: []Player{
					{SteamID: "1", PersonaName: "Alice"},
					{SteamID: "2", PersonaName: "Bob"},
				},
			},
		})
	}))

	players, err := client.Players(context.Background(), []string{"1", "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/ISteamUser/GetPlayerSummaries/v0002" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotSteamIDs != "1,2" {
		t.Errorf("expected steamids=1,2, got %q", gotSteamIDs)
	}
	if gotKey != "test-api-key" {
		t.Errorf("expected key=test-api-key, got %q", gotKey)
	}
	if len(players) != 2 || players[0].PersonaName != "Alice" || players[1].PersonaName != "Bob" {
		t.Errorf("unexpected players: %+v", players)
	}
}

func TestPlayers_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))

	_, err := client.Players(context.Background(), []string{"1"})
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestPlayers_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, PlayersResult{})
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Players(ctx, []string{"1"})
	if err == nil {
		t.Fatal("expected an error for a canceled context, got nil")
	}
}

func TestPlayer_Success(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("steamids"); got != "42" {
			t.Errorf("expected steamids=42, got %q", got)
		}
		writeJSON(t, w, PlayersResult{
			Response: PlayersList{
				Players: []Player{{SteamID: "42", PersonaName: "Solo"}},
			},
		})
	}))

	player, err := client.Player(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.SteamID != "42" || player.PersonaName != "Solo" {
		t.Errorf("unexpected player: %+v", player)
	}
}

func TestPlayer_NotFound(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, PlayersResult{})
	}))

	_, err := client.Player(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected an error when no player is returned, got nil")
	}
}

func TestPlayer_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))

	_, err := client.Player(context.Background(), "42")
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestGames_Success(t *testing.T) {
	var gotPath string
	var gotQuery map[string]string

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = map[string]string{
			"steamid":                   r.URL.Query().Get("steamid"),
			"key":                       r.URL.Query().Get("key"),
			"include_appinfo":           r.URL.Query().Get("include_appinfo"),
			"include_played_free_games": r.URL.Query().Get("include_played_free_games"),
		}

		writeJSON(t, w, GamesResult{
			Response: GamesList{
				Games: []Game{
					{AppID: 440, Name: "Team Fortress 2", PlaytimeForever: 120},
				},
			},
		})
	}))

	games, err := client.Games(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/IPlayerService/GetOwnedGames/v0001/" {
		t.Errorf("unexpected request path: %s", gotPath)
	}
	if gotQuery["steamid"] != "42" {
		t.Errorf("expected steamid=42, got %q", gotQuery["steamid"])
	}
	if gotQuery["include_appinfo"] != "1" || gotQuery["include_played_free_games"] != "1" {
		t.Errorf("unexpected query flags: %+v", gotQuery)
	}
	if len(games) != 1 || games[0].Name != "Team Fortress 2" {
		t.Errorf("unexpected games: %+v", games)
	}
}

func TestGames_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))

	_, err := client.Games(context.Background(), "42")
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestFriends_Success(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ISteamUser/GetFriendList/v0001/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("steamid"); got != "1" {
			t.Errorf("expected steamid=1, got %q", got)
		}
		writeJSON(t, w, FriendsResult{
			FriendsList: FriendsList{
				Friends: []Friend{
					{SteamID: "2", FriendSince: 100},
					{SteamID: "3", FriendSince: 200},
				},
			},
		})
	})

	mux.HandleFunc("/ISteamUser/GetPlayerSummaries/v0002", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("steamids"); got != "2,3" {
			t.Errorf("expected steamids=2,3, got %q", got)
		}
		writeJSON(t, w, PlayersResult{
			Response: PlayersList{
				Players: []Player{
					{SteamID: "2", PersonaName: "Carol"},
					{SteamID: "3", PersonaName: "Dave"},
				},
			},
		})
	})

	client := newTestClient(t, mux)

	friends, err := client.Friends(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(friends) != 2 || friends[0].PersonaName != "Carol" || friends[1].PersonaName != "Dave" {
		t.Errorf("unexpected friends: %+v", friends)
	}
}

func TestFriends_NoFriends(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/ISteamUser/GetFriendList/v0001/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, FriendsResult{})
	})

	mux.HandleFunc("/ISteamUser/GetPlayerSummaries/v0002", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("steamids"); got != "" {
			t.Errorf("expected empty steamids, got %q", got)
		}
		writeJSON(t, w, PlayersResult{})
	})

	client := newTestClient(t, mux)

	friends, err := client.Friends(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(friends) != 0 {
		t.Errorf("expected no friends, got %+v", friends)
	}
}

func TestFriends_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))

	_, err := client.Friends(context.Background(), "1")
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}
