package steamapi

import (
	"context"
	"errors"
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
