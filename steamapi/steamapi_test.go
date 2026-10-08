package steamapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedGamesWithOneOtherIDReturnsSharedGames(t *testing.T) {
	ctx := context.Background()
	want := []Game{{AppID: 2, Name: "Shared"}}
	client := &Client{categoriesFn: stubCategories(t, map[int][]string{2: {"Multi-player"}}, nil), gamesFn: stubGames(t, map[string][]Game{
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
	client := &Client{categoriesFn: stubCategories(t, map[int][]string{2: {"Multi-player"}, 3: {"Multi-player"}}, nil), gamesFn: stubGames(t, map[string][]Game{
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
	client := &Client{categoriesFn: stubCategories(t, map[int][]string{2: {"Multi-player"}}, nil), gamesFn: stubGames(t, map[string][]Game{
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

func TestSharedGamesFetchesConcurrently(t *testing.T) {
	steamIDs := []string{"caller", "friend1", "friend2"}
	entered := make(chan string, len(steamIDs))
	released := make(chan struct{})
	abort := make(chan struct{})
	defer close(abort)
	want := []Game{{AppID: 1, Name: "Shared"}}
	client := &Client{categoriesFn: stubCategories(t, map[int][]string{1: {"Multi-player"}}, nil), gamesFn: func(_ context.Context, steamID string) ([]Game, error) {
		entered <- steamID
		select {
		case <-released:
			return want, nil
		case <-abort:
			return nil, errors.New("test stopped")
		}
	}}

	var got []Game
	var err error
	done := make(chan struct{})
	go func() {
		got, err = client.SharedGames(context.Background(), steamIDs[0], steamIDs[1:]...)
		close(done)
	}()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	seen := make(map[string]bool)
	for range steamIDs {
		select {
		case steamID := <-entered:
			if seen[steamID] {
				t.Fatalf("fetch entered twice for %q", steamID)
			}
			seen[steamID] = true
		case <-timer.C:
			t.Fatal("not all fetches entered the barrier; SharedGames may be fetching serially")
		}
	}
	for _, steamID := range steamIDs {
		if !seen[steamID] {
			t.Fatalf("fetch never entered for %q", steamID)
		}
	}
	close(released)

	select {
	case <-done:
		if err != nil {
			t.Fatalf("SharedGames() error = %v, want nil", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("SharedGames() = %#v, want %#v", got, want)
		}
	case <-timer.C:
		t.Fatal("SharedGames did not finish after releasing the barrier")
	}
}

func TestSharedGamesWritesResultsByIndexNotCompletionOrder(t *testing.T) {
	tests := []struct {
		name           string
		gamesBySteamID map[string][]Game
		want           []Game
		wantEmptyIDs   []string
	}{
		{
			name: "intersection follows caller order and uses caller metadata",
			gamesBySteamID: map[string][]Game{
				"caller":  {{AppID: 1}, {AppID: 2, Name: "Caller Two"}, {AppID: 3, Name: "Caller Three"}},
				"friend1": {{AppID: 3}, {AppID: 2}, {AppID: 4}},
				"friend2": {{AppID: 3}, {AppID: 2}, {AppID: 5}},
			},
			want: []Game{{AppID: 2, Name: "Caller Two"}, {AppID: 3, Name: "Caller Three"}},
		},
		{
			name: "empty users follow caller order",
			gamesBySteamID: map[string][]Game{
				"caller":  {},
				"friend1": {},
				"friend2": {{AppID: 2}},
			},
			wantEmptyIDs: []string{"caller", "friend1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			friend1Done := make(chan struct{})
			friend2Done := make(chan struct{})
			abort := make(chan struct{})
			defer close(abort)
			client := &Client{categoriesFn: stubCategories(t, map[int][]string{2: {"Multi-player"}, 3: {"Multi-player"}}, nil), gamesFn: func(_ context.Context, steamID string) ([]Game, error) {
				// Hold the caller until both friends have reached their returns,
				// and hold friend1 until friend2 reaches its return.
				switch steamID {
				case "caller":
					select {
					case <-friend1Done:
					case <-abort:
						return nil, errors.New("test stopped")
					}
				case "friend1":
					defer close(friend1Done)
					select {
					case <-friend2Done:
					case <-abort:
						return nil, errors.New("test stopped")
					}
				case "friend2":
					defer close(friend2Done)
				}
				return tt.gamesBySteamID[steamID], nil
			}}

			var got []Game
			var err error
			done := make(chan struct{})
			go func() {
				got, err = client.SharedGames(context.Background(), "caller", "friend1", "friend2")
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("SharedGames did not complete the reverse-order fetches")
			}
			if tt.wantEmptyIDs != nil {
				var noGamesErr *NoGamesError
				if !errors.As(err, &noGamesErr) {
					t.Fatalf("SharedGames() error = %v, want NoGamesError", err)
				}
				if !reflect.DeepEqual(noGamesErr.SteamIDs, tt.wantEmptyIDs) {
					t.Fatalf("NoGamesError.SteamIDs = %#v, want %#v", noGamesErr.SteamIDs, tt.wantEmptyIDs)
				}
			} else if err != nil {
				t.Fatalf("SharedGames() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("SharedGames() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSharedGamesCancelsSiblingsOnError(t *testing.T) {
	transportErr := errors.New("transport failed")
	canceled := make(chan struct{})
	abort := make(chan struct{})
	defer close(abort)
	client := &Client{gamesFn: func(ctx context.Context, steamID string) ([]Game, error) {
		if steamID == "error-friend" {
			return nil, transportErr
		}
		select {
		case <-ctx.Done():
			close(canceled)
			return nil, ctx.Err()
		case <-abort:
			return nil, errors.New("test stopped")
		}
	}}

	var games []Game
	var err error
	done := make(chan struct{})
	go func() {
		games, err = client.SharedGames(context.Background(), "caller", "error-friend")
		close(done)
	}()

	select {
	case <-done:
		if !errors.Is(err, transportErr) || err != transportErr {
			t.Fatalf("SharedGames() error = %v, want exact underlying error %v", err, transportErr)
		}
		if games != nil {
			t.Fatalf("SharedGames() games = %#v, want nil on error", games)
		}
		select {
		case <-canceled:
		default:
			t.Fatal("sibling did not observe context cancellation")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SharedGames did not cancel and wait for the blocking sibling")
	}
}

func TestSharedGamesFiltersOnlyAllOwnersInCallerOrder(t *testing.T) {
	want := []Game{{AppID: 3, Name: "Caller Three", PlaytimeForever: 30}, {AppID: 2, Name: "Caller Two", PlaytimeForever: 20}}
	var calls []int
	categories := stubCategories(t, map[int][]string{
		1: {"Multi-player"},
		2: {"Co-op"},
		3: {"Online PvP"},
	}, nil)
	client := &Client{
		gamesFn: stubGames(t, map[string][]Game{
			"caller":  {{AppID: 1, Name: "Subset Only"}, want[0], want[1]},
			"friend1": {{AppID: 2, Name: "Friend Two"}, {AppID: 1}, {AppID: 3}},
			"friend2": {{AppID: 2}, {AppID: 3, Name: "Friend Three"}},
		}, nil),
		categoriesFn: func(ctx context.Context, appID int) ([]string, error) {
			calls = append(calls, appID)
			return categories(ctx, appID)
		},
	}
	got, err := client.SharedGames(context.Background(), "caller", "friend1", "friend2")
	if err != nil {
		t.Fatalf("SharedGames() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SharedGames() = %#v, want caller-ordered metadata %#v", got, want)
	}
	if !reflect.DeepEqual(calls, []int{3, 2}) {
		t.Fatalf("category calls = %v, want only all-owner apps [3 2]", calls)
	}
}

func TestSharedGamesExcludesAllOwnedNonMultiplayerGames(t *testing.T) {
	games := []Game{{AppID: 1}, {AppID: 2}, {AppID: 3}}
	client := &Client{
		gamesFn: stubGames(t, map[string][]Game{"caller": games, "friend": games}, nil),
		categoriesFn: stubCategories(t, map[int][]string{
			1: {"Steam Achievements", "Controller Support"},
			2: nil,
			3: {"Co-op"},
		}, nil),
	}
	got, err := client.SharedGames(context.Background(), "caller", "friend")
	if err != nil {
		t.Fatalf("SharedGames() error = %v, want nil", err)
	}
	if want := []Game{{AppID: 3}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("SharedGames() = %#v, want only multiplayer game %#v", got, want)
	}
}

func TestSharedGamesMultiplayerCategoryMembership(t *testing.T) {
	tests := []struct {
		name       string
		categories []string
		want       bool
	}{
		{"Multi-player", []string{"Multi-player"}, true},
		{"Co-op", []string{"Co-op"}, true},
		{"Online Co-op", []string{"Online Co-op"}, true},
		{"PvP", []string{"PvP"}, true},
		{"Online PvP", []string{"Online PvP"}, true},
		{"Shared/Split Screen", []string{"Shared/Split Screen"}, true},
		{"Single-player", []string{"Single-player"}, false},
		{"empty", nil, false},
		{"case sensitive", []string{"multi-player"}, false},
		{"no substring matching", []string{"Local Co-op", "Shared/Split Screen Co-op"}, false},
		{"qualifying category after unrelated", []string{"Single-player", "Co-op"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMultiplayer(tt.categories); got != tt.want {
				t.Errorf("isMultiplayer(%v) = %v, want %v", tt.categories, got, tt.want)
			}
			game := Game{AppID: 42, Name: "Shared"}
			client := &Client{
				gamesFn:      stubGames(t, map[string][]Game{"caller": {game}, "friend": {game}}, nil),
				categoriesFn: stubCategories(t, map[int][]string{42: tt.categories}, nil),
			}
			got, err := client.SharedGames(context.Background(), "caller", "friend")
			if err != nil {
				t.Fatalf("SharedGames() error = %v, want nil", err)
			}
			want := []Game{}
			if tt.want {
				want = append(want, game)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("SharedGames() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestSharedGamesAllEmptySkipsCategories(t *testing.T) {
	var calls int
	client := &Client{
		gamesFn: stubGames(t, map[string][]Game{"caller": {}, "friend1": {}, "friend2": {}}, nil),
		categoriesFn: func(context.Context, int) ([]string, error) {
			calls++
			return nil, errors.New("categories must not be fetched")
		},
	}
	got, err := client.SharedGames(context.Background(), "caller", "friend1", "friend2")
	var noGamesErr *NoGamesError
	if !errors.As(err, &noGamesErr) {
		t.Fatalf("SharedGames() error = %v, want NoGamesError", err)
	}
	if want := []string{"caller", "friend1", "friend2"}; !reflect.DeepEqual(noGamesErr.SteamIDs, want) {
		t.Errorf("empty IDs = %v, want %v", noGamesErr.SteamIDs, want)
	}
	if got != nil || calls != 0 {
		t.Fatalf("SharedGames() = %#v, category calls = %d; want nil and zero calls", got, calls)
	}
}

func TestSharedGamesNoIntersectionSkipsCategories(t *testing.T) {
	calls := 0
	client := &Client{
		gamesFn: stubGames(t, map[string][]Game{"caller": {{AppID: 1}}, "friend": {{AppID: 2}}}, nil),
		categoriesFn: func(context.Context, int) ([]string, error) {
			calls++
			return nil, errors.New("categories must not be fetched")
		},
	}
	got, err := client.SharedGames(context.Background(), "caller", "friend")
	if err != nil || len(got) != 0 || calls != 0 {
		t.Fatalf("SharedGames() = %#v, %v; category calls = %d; want empty, nil, zero", got, err, calls)
	}
}

func TestSharedGamesPropagatesCategoryErrorWithoutPartialResult(t *testing.T) {
	categoryErr := errors.New("category lookup failed")
	games := []Game{{AppID: 1}, {AppID: 2}}
	client := &Client{
		gamesFn:      stubGames(t, map[string][]Game{"caller": games, "friend": games}, nil),
		categoriesFn: stubCategories(t, map[int][]string{1: {"Co-op"}}, map[int]error{2: categoryErr}),
	}
	got, err := client.SharedGames(context.Background(), "caller", "friend")
	if err != categoryErr || got != nil {
		t.Fatalf("SharedGames() = %#v, %v; want nil, exact category error", got, err)
	}
}

type categoryTransport func(*http.Request) (*http.Response, error)

func (f categoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSharedGamesDefaultCategoriesUsesStore(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "category context")
	var calls []string
	client := NewClient("must-not-be-sent-to-store")
	client.gamesFn = stubGames(t, map[string][]Game{
		"caller": {{AppID: 42}, {AppID: 43}},
		"friend": {{AppID: 43}, {AppID: 42}},
	}, nil)
	client.client.Transport = categoryTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Scheme != "https" || req.URL.Host != "store.steampowered.com" || req.URL.Path != "/api/appdetails" {
			t.Errorf("unexpected Store request: %s %s", req.Method, req.URL)
		}
		if req.Context().Value(contextKey{}) != "category context" {
			t.Error("Store request lost caller context")
		}
		appID := req.URL.Query().Get("appids")
		if want := map[string][]string{"appids": {appID}}; !reflect.DeepEqual(map[string][]string(req.URL.Query()), want) {
			t.Errorf("unexpected Store query: %v", req.URL.Query())
		}
		calls = append(calls, appID)
		var body string
		switch appID {
		case "42":
			body = `{"42":{"success":true,"data":{"categories":[{"id":9,"description":"Co-op"}]}}}`
		case "43":
			body = `{"43":{"success":true,"data":{"categories":[{"id":2,"description":"Single-player"}]}}}`
		default:
			t.Fatalf("unexpected Store app ID %q", appID)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	got, err := client.SharedGames(ctx, "caller", "friend")
	if err != nil || !reflect.DeepEqual(got, []Game{{AppID: 42}}) {
		t.Fatalf("SharedGames() = %#v, %v; want app 42, nil", got, err)
	}
	if !reflect.DeepEqual(calls, []string{"42", "43"}) {
		t.Fatalf("Store app requests = %v, want [42 43]", calls)
	}
}

func TestCategoriesDecodesDescriptions(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"descriptions", `{"42":{"success":true,"data":{"categories":[{"id":2,"description":"Single-player"},{"id":9,"description":"Co-op"}]}}}`, []string{"Single-player", "Co-op"}},
		{"no categories", `{"42":{"success":true,"data":{}}}`, []string{}},
		{"unavailable", `{"42":{"success":false}}`, nil},
		{"missing app", `{}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("")
			client.client.Transport = categoryTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})
			got, err := client.categories(context.Background(), 42)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("categories() = %#v, %v; want %#v, nil", got, err, tt.want)
			}
		})
	}
}

func TestCategoriesRequestErrors(t *testing.T) {
	t.Run("building request", func(t *testing.T) {
		original := storeBaseURL
		storeBaseURL = "://invalid"
		t.Cleanup(func() { storeBaseURL = original })
		_, err := NewClient("").categories(context.Background(), 42)
		if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
			t.Fatalf("error = %v, want request-building error", err)
		}
	})
	t.Run("performing request", func(t *testing.T) {
		transportErr := errors.New("store unavailable")
		client := NewClient("")
		client.client.Transport = categoryTransport(func(*http.Request) (*http.Response, error) {
			return nil, transportErr
		})
		_, err := client.categories(context.Background(), 42)
		if !errors.Is(err, transportErr) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
			t.Fatalf("error = %v, want wrapped transport error", err)
		}
	})
	t.Run("decoding response", func(t *testing.T) {
		client := NewClient("")
		client.client.Transport = categoryTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("not json"))}, nil
		})
		_, err := client.categories(context.Background(), 42)
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) || !strings.HasPrefix(err.Error(), "steamapi: decoding response:") {
			t.Fatalf("error = %v, want wrapped JSON syntax error", err)
		}
	})
	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := NewClient("")
		client.client.Transport = categoryTransport(func(req *http.Request) (*http.Response, error) {
			if req.Context().Err() != context.Canceled {
				t.Error("Store request did not carry cancellation")
			}
			return nil, req.Context().Err()
		})
		_, err := client.categories(ctx, 42)
		if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
			t.Fatalf("error = %v, want wrapped context cancellation", err)
		}
	})
}

func stubCategories(t *testing.T, categoriesByAppID map[int][]string, errorsByAppID map[int]error) func(context.Context, int) ([]string, error) {
	t.Helper()
	return func(_ context.Context, appID int) ([]string, error) {
		if err := errorsByAppID[appID]; err != nil {
			return nil, err
		}
		categories, ok := categoriesByAppID[appID]
		if !ok {
			t.Fatalf("unexpected categories call for app ID %d", appID)
		}
		return categories, nil
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

	games, err := client.Games(context.Background(), "42", true, true)
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

	_, err := client.Games(context.Background(), "42", true, true)
	if err == nil {
		t.Fatal("expected an error for invalid JSON, got nil")
	}
}

func TestFriends_Success(t *testing.T) {
	mux := http.NewServeMux()
	var requests atomic.Int32
	mux.HandleFunc("/ISteamUser/GetFriendList/v0001/", func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		assertRequest(t, r, "/ISteamUser/GetFriendList/v0001/", map[string]string{
			"steamid": "1", "relationship": "friend",
		})
		writeJSON(t, w, json.RawMessage(`{"friendslist":{"friends":[
			{"steamid":"2","friend_since":100,"relationship":"friend"},
			{"steamid":"3","friend_since":200,"relationship":"friend"}
		]}}`))
	})
	mux.HandleFunc("/ISteamUser/GetPlayerSummaries/v0002", func(w http.ResponseWriter, r *http.Request) {
		// Fail without calling Fatal from the HTTP server's goroutine.
		t.Error("Friends must not request player summaries")
		http.Error(w, "unexpected hydration", http.StatusInternalServerError)
	})
	client := newTestClient(t, mux)
	friends, err := client.Friends(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Friend{
		{SteamID: "2", FriendSince: 100, Relationship: "friend"},
		{SteamID: "3", FriendSince: 200, Relationship: "friend"},
	}
	if !reflect.DeepEqual(friends, want) {
		t.Errorf("Friends() = %#v, want %#v", friends, want)
	}
	if got := requests.Load(); got != 1 {
		t.Errorf("GetFriendList requests = %d, want 1", got)
	}
}

func TestFriends_NoFriends(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ISteamUser/GetFriendList/v0001/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, FriendsResult{})
	})
	mux.HandleFunc("/ISteamUser/GetPlayerSummaries/v0002", func(w http.ResponseWriter, r *http.Request) {
		t.Error("Friends must not request player summaries for an empty list")
		http.Error(w, "unexpected hydration", http.StatusInternalServerError)
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

// assertRequest checks the common helper contract as well as endpoint-specific parameters.
func assertRequest(t *testing.T, r *http.Request, path string, params map[string]string) {
	t.Helper()
	if r.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", r.Method)
	}
	if r.URL.Path != path {
		t.Errorf("path = %q, want %q", r.URL.Path, path)
	}
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	wantQuery := map[string][]string{"key": {"test-api-key"}}
	for key, value := range params {
		wantQuery[key] = []string{value}
	}
	if got := map[string][]string(r.URL.Query()); !reflect.DeepEqual(got, wantQuery) {
		t.Errorf("query = %#v, want %#v", got, wantQuery)
	}
}

func TestGetNewsForApp_Success(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/ISteamNews/GetNewsForApp/v0002", map[string]string{"appid": "440"})
		writeJSON(t, w, json.RawMessage(`{"appnews":{"appid":440,"newsitems":[{
			"gid":"12345678901234567890","title":"Update","url":"https://example.com/news",
			"is_external_url":true,"author":"Valve","contents":"Patch notes","feedlabel":"Updates",
			"date":1700000000,"feedname":"steam_updates","feed_type":1,"appid":440
		}],"count":1}}`))
	}))
	got, err := client.GetNewsForApp(context.Background(), "440")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []NewsItem{{GID: "12345678901234567890", Title: "Update", URL: "https://example.com/news",
		IsExternalURL: true, Author: "Valve", Contents: "Patch notes", FeedLabel: "Updates",
		Date: 1700000000, FeedName: "steam_updates", FeedType: 1, AppID: 440}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetNewsForApp() = %#v, want %#v", got, want)
	}
}

func TestGetGlobalAchievementPercentagesForApp_Success(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/ISteamUserStats/GetGlobalAchievementPercentagesForApp/v0002", map[string]string{"gameid": "440"})
		writeJSON(t, w, json.RawMessage(`{"achievementpercentages":{"achievements":[{"name":"WIN_ONE","percent":12.3}]}}`))
	}))
	got, err := client.GetGlobalAchievementPercentagesForApp(context.Background(), "440")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []GlobalAchievement{{Name: "WIN_ONE", Percent: 12.3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetGlobalAchievementPercentagesForApp() = %#v, want %#v", got, want)
	}
}

func TestPlayerSummaries_EnrichedFields(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/ISteamUser/GetPlayerSummaries/v0002", map[string]string{"steamids": "42"})
		writeJSON(t, w, json.RawMessage(`{"response":{"players":[{
			"steamid":"42","personaname":"Solo","avatar":"small","avatarmedium":"medium","avatarfull":"full",
			"communityvisibilitystate":3,"profilestate":1,"lastlogoff":1700000000,"personastate":2,
			"realname":"Alex","primaryclanid":"103582791429521412","timecreated":1600000000,
			"personastateflags":4,"gameid":"440","gameextrainfo":"Team Fortress 2",
			"loccountrycode":"US","locstatecode":"WA","loccityid":3961,"profileurl":"https://steamcommunity.com/id/solo/"
		}]}}`))
	}))
	want := Player{SteamID: "42", PersonaName: "Solo", AvatarSmall: "small", AvatarMedium: "medium", AvatarFull: "full",
		CommunityVisibilityState: 3, ProfileState: 1, LastLogoff: 1700000000, PersonaState: 2,
		RealName: "Alex", PrimaryClanID: "103582791429521412", TimeCreated: 1600000000,
		PersonaStateFlags: 4, GameID: "440", GameExtraInfo: "Team Fortress 2",
		LocCountryCode: "US", LocStateCode: "WA", LocCityID: 3961, ProfileURL: "https://steamcommunity.com/id/solo/"}
	players, err := client.Players(context.Background(), []string{"42"})
	if err != nil {
		t.Fatalf("Players() error = %v", err)
	}
	if !reflect.DeepEqual(players, []Player{want}) {
		t.Errorf("Players() = %#v, want %#v", players, []Player{want})
	}
	player, err := client.Player(context.Background(), "42")
	if err != nil {
		t.Fatalf("Player() error = %v", err)
	}
	if player != want {
		t.Errorf("Player() = %#v, want %#v", player, want)
	}
}

func TestGetPlayerAchievements_Success(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/ISteamUserStats/GetPlayerAchievements/v0001", map[string]string{"steamid": "42", "appid": "440"})
		writeJSON(t, w, json.RawMessage(`{"playerstats":{"steamID":"42","gameName":"Team Fortress 2","achievements":[
			{"apiname":"WIN_ONE","achieved":1,"unlocktime":1700000000,"name":"First win","description":"Win a game"},
			{"apiname":"WIN_TWO","achieved":0,"unlocktime":0}
		],"success":true}}`))
	}))
	got, err := client.GetPlayerAchievements(context.Background(), "42", "440")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []PlayerAchievement{
		{APIName: "WIN_ONE", Achieved: 1, UnlockTime: 1700000000, Name: "First win", Description: "Win a game"},
		{APIName: "WIN_TWO"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetPlayerAchievements() = %#v, want %#v", got, want)
	}
}

func TestGetUserStatsForGame_Success(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/ISteamUserStats/GetUserStatsForGame/v0002", map[string]string{"steamid": "42", "appid": "440"})
		writeJSON(t, w, json.RawMessage(`{"playerstats":{"steamID":"42","gameName":"Team Fortress 2","stats":[{"name":"wins","value":17}],"achievements":[]}}`))
	}))
	got, err := client.GetUserStatsForGame(context.Background(), "42", "440")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []UserStat{{Name: "wins", Value: 17}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetUserStatsForGame() = %#v, want %#v", got, want)
	}
}

func TestGames_EnrichedFields(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/IPlayerService/GetOwnedGames/v0001/", map[string]string{
			"steamid": "42", "include_appinfo": "1", "include_played_free_games": "1",
		})
		writeJSON(t, w, json.RawMessage(`{"response":{"game_count":1,"games":[{
			"appid":440,"name":"Team Fortress 2","playtime_forever":120,"img_icon_url":"icon","img_logo_url":"logo",
			"playtime_2weeks":30,"has_community_visible_stats":true,"playtime_windows_forever":80,
			"playtime_mac_forever":10,"playtime_linux_forever":30,"rtime_last_played":1700000000
		}]}}`))
	}))
	got, err := client.Games(context.Background(), "42", true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Game{{AppID: 440, Name: "Team Fortress 2", PlaytimeForever: 120, ImgIconURL: "icon", ImgLogoURL: "logo",
		Playtime2Weeks: 30, HasCommunityVisibleStats: true, PlaytimeWindowsForever: 80,
		PlaytimeMacForever: 10, PlaytimeLinuxForever: 30, RTimeLastPlayed: 1700000000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Games() = %#v, want %#v", got, want)
	}
}

func TestGetRecentlyPlayedGames_Success(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/IPlayerService/GetRecentlyPlayedGames/v0001", map[string]string{"steamid": "42"})
		writeJSON(t, w, json.RawMessage(`{"response":{"total_count":1,"games":[{
			"appid":440,"name":"Team Fortress 2","playtime_2weeks":30,"playtime_forever":120,"img_icon_url":"icon","img_logo_url":"logo"
		}]}}`))
	}))
	got, err := client.GetRecentlyPlayedGames(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Game{{AppID: 440, Name: "Team Fortress 2", Playtime2Weeks: 30, PlaytimeForever: 120, ImgIconURL: "icon", ImgLogoURL: "logo"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetRecentlyPlayedGames() = %#v, want %#v", got, want)
	}
}

func TestSharedGames_DefaultFetchUsesOwnedGames(t *testing.T) {
	var requests atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		steamID := r.URL.Query().Get("steamid")
		assertRequest(t, r, "/IPlayerService/GetOwnedGames/v0001/", map[string]string{
			"steamid": steamID, "include_appinfo": "1", "include_played_free_games": "1",
		})
		switch steamID {
		case "caller":
			writeJSON(t, w, json.RawMessage(`{"response":{"games":[{"appid":1,"name":"Only caller"},{"appid":2,"name":"Shared"}]}}`))
		case "friend":
			writeJSON(t, w, json.RawMessage(`{"response":{"games":[{"appid":2,"name":"Shared"},{"appid":3,"name":"Only friend"}]}}`))
		default:
			t.Errorf("unexpected Steam ID: %q", steamID)
			http.Error(w, "unexpected Steam ID", http.StatusBadRequest)
		}
	}))
	client.categoriesFn = stubCategories(t, map[int][]string{2: {"Multi-player"}}, nil)
	got, err := client.SharedGames(context.Background(), "caller", "friend")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Game{{AppID: 2, Name: "Shared"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SharedGames() = %#v, want %#v", got, want)
	}
	if got := requests.Load(); got != 2 {
		t.Errorf("GetOwnedGames requests = %d, want 2", got)
	}
}

func TestGames_FlagsNoExtras(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/IPlayerService/GetOwnedGames/v0001/", map[string]string{
			"steamid": "42", "include_appinfo": "0", "include_played_free_games": "0",
		})
		writeJSON(t, w, json.RawMessage(`{"response":{"games":[]}}`))
	}))
	_, err := client.Games(context.Background(), "42", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGames_FlagsAppInfoOnly(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/IPlayerService/GetOwnedGames/v0001/", map[string]string{
			"steamid": "42", "include_appinfo": "1", "include_played_free_games": "0",
		})
		writeJSON(t, w, json.RawMessage(`{"response":{"games":[]}}`))
	}))
	_, err := client.Games(context.Background(), "42", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGames_FlagsFreeGamesOnly(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/IPlayerService/GetOwnedGames/v0001/", map[string]string{
			"steamid": "42", "include_appinfo": "0", "include_played_free_games": "1",
		})
		writeJSON(t, w, json.RawMessage(`{"response":{"games":[]}}`))
	}))
	_, err := client.Games(context.Background(), "42", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlayers_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.Players(context.Background(), []string{"42"})
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestPlayer_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Player(ctx, "42")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestPlayer_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.Player(context.Background(), "42")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestGames_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Games(ctx, "42", true, true)
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestGames_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.Games(context.Background(), "42", true, true)
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestFriends_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.Friends(ctx, "42")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestFriends_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.Friends(context.Background(), "42")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestGetNewsForApp_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	_, err := client.GetNewsForApp(context.Background(), "440")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.HasPrefix(err.Error(), "steamapi: decoding response:") {
		t.Fatalf("error = %v, want wrapped JSON syntax error", err)
	}
}

func TestGetNewsForApp_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetNewsForApp(ctx, "440")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestGetNewsForApp_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.GetNewsForApp(context.Background(), "440")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestGetGlobalAchievementPercentagesForApp_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	_, err := client.GetGlobalAchievementPercentagesForApp(context.Background(), "440")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.HasPrefix(err.Error(), "steamapi: decoding response:") {
		t.Fatalf("error = %v, want wrapped JSON syntax error", err)
	}
}

func TestGetGlobalAchievementPercentagesForApp_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetGlobalAchievementPercentagesForApp(ctx, "440")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestGetGlobalAchievementPercentagesForApp_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.GetGlobalAchievementPercentagesForApp(context.Background(), "440")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestGetPlayerAchievements_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	_, err := client.GetPlayerAchievements(context.Background(), "42", "440")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.HasPrefix(err.Error(), "steamapi: decoding response:") {
		t.Fatalf("error = %v, want wrapped JSON syntax error", err)
	}
}

func TestGetPlayerAchievements_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetPlayerAchievements(ctx, "42", "440")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestGetPlayerAchievements_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.GetPlayerAchievements(context.Background(), "42", "440")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestGetUserStatsForGame_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	_, err := client.GetUserStatsForGame(context.Background(), "42", "440")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.HasPrefix(err.Error(), "steamapi: decoding response:") {
		t.Fatalf("error = %v, want wrapped JSON syntax error", err)
	}
}

func TestGetUserStatsForGame_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetUserStatsForGame(ctx, "42", "440")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestGetUserStatsForGame_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.GetUserStatsForGame(context.Background(), "42", "440")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestGetRecentlyPlayedGames_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	_, err := client.GetRecentlyPlayedGames(context.Background(), "42")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.HasPrefix(err.Error(), "steamapi: decoding response:") {
		t.Fatalf("error = %v, want wrapped JSON syntax error", err)
	}
}

func TestGetRecentlyPlayedGames_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetRecentlyPlayedGames(ctx, "42")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestGetRecentlyPlayedGames_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.GetRecentlyPlayedGames(context.Background(), "42")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestGetNumberOfCurrentPlayers_Success(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, "/IPlayerService/GetNumberOfCurrentPlayers/v0001", map[string]string{"appid": "440"})
		writeJSON(t, w, json.RawMessage(`{"response":{"player_count":12345,"result":1}}`))
	}))
	got, err := client.GetNumberOfCurrentPlayers(context.Background(), "440")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 12345 {
		t.Errorf("GetNumberOfCurrentPlayers() = %d, want %d", got, 12345)
	}
}

func TestGetNumberOfCurrentPlayers_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	_, err := client.GetNumberOfCurrentPlayers(context.Background(), "440")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.HasPrefix(err.Error(), "steamapi: decoding response:") {
		t.Fatalf("error = %v, want wrapped JSON syntax error", err)
	}
}

func TestGetNumberOfCurrentPlayers_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetNumberOfCurrentPlayers(ctx, "440")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestGetNumberOfCurrentPlayers_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.GetNumberOfCurrentPlayers(context.Background(), "440")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}

func TestSharedGames_DecodeError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	_, err := client.SharedGames(context.Background(), "42")
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) || !strings.HasPrefix(err.Error(), "steamapi: decoding response:") {
		t.Fatalf("error = %v, want wrapped JSON syntax error", err)
	}
}

func TestSharedGames_ContextCanceled(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled request must not reach the server")
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.SharedGames(ctx, "42")
	if !errors.Is(err, context.Canceled) || !strings.HasPrefix(err.Error(), "steamapi: performing request:") {
		t.Fatalf("error = %v, want wrapped context cancellation", err)
	}
}

func TestSharedGames_RequestBuildError(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid request URL must not reach the server")
	}))
	baseURL = "://invalid"
	_, err := client.SharedGames(context.Background(), "42")
	if err == nil || !strings.HasPrefix(err.Error(), "steamapi: building request:") {
		t.Fatalf("error = %v, want request-building error", err)
	}
}
