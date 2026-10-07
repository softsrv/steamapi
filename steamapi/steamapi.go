// Package steamapi implements a client over some of steam's webapis
package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// baseURL is a var (not const) so tests can point it at a mock server.
var baseURL = "https://api.steampowered.com"

// storeBaseURL is separate from the Web API host and can be replaced in tests.
var storeBaseURL = "https://store.steampowered.com"

const userService = "ISteamUser"
const playerService = "IPlayerService"
const newsService = "ISteamNews"
const userStatsService = "ISteamUserStats"

// Player contains details about the Steam User
type Player struct {
	SteamID                  string `json:"steamid"`
	PersonaName              string `json:"personaname"`
	AvatarSmall              string `json:"avatar"`
	AvatarMedium             string `json:"avatarmedium"`
	AvatarFull               string `json:"avatarfull"`
	CommunityVisibilityState int    `json:"communityvisibilitystate"`
	ProfileState             int    `json:"profilestate"`
	LastLogoff               int    `json:"lastlogoff"`
	PersonaState             int    `json:"personastate"`
	RealName                 string `json:"realname"`
	PrimaryClanID            string `json:"primaryclanid"`
	TimeCreated              int    `json:"timecreated"`
	PersonaStateFlags        int    `json:"personastateflags"`
	GameID                   string `json:"gameid"`
	GameExtraInfo            string `json:"gameextrainfo"`
	LocCountryCode           string `json:"loccountrycode"`
	LocStateCode             string `json:"locstatecode"`
	LocCityID                int    `json:"loccityid"`
	ProfileURL               string `json:"profileurl"`
}

// PlayersList contains a slice of Player objects.
type PlayersList struct {
	Players []Player `json:"players"`
}

// PlayersResult contains a "response" object with relevant data
type PlayersResult struct {
	Response PlayersList `json:"response"`
}

// Game contains details about a Steam game
type Game struct {
	AppID                    int    `json:"appid"`
	Name                     string `json:"name"`
	PlaytimeForever          int    `json:"playtime_forever"`
	ImgIconURL               string `json:"img_icon_url"`
	ImgLogoURL               string `json:"img_logo_url"`
	Playtime2Weeks           int    `json:"playtime_2weeks"`
	HasCommunityVisibleStats bool   `json:"has_community_visible_stats"`
	PlaytimeWindowsForever   int    `json:"playtime_windows_forever"`
	PlaytimeMacForever       int    `json:"playtime_mac_forever"`
	PlaytimeLinuxForever     int    `json:"playtime_linux_forever"`
	RTimeLastPlayed          int    `json:"rtime_last_played"`
}

// GamesList contains a slice of Game objects
type GamesList struct {
	Games []Game `json:"games"`
}

// GamesResult contains a "response" object with relevant data
type GamesResult struct {
	Response GamesList `json:"response"`
}

// A Friend is a reference to a Player who is friends with a particular user
type Friend struct {
	SteamID      string `json:"steamid"`
	FriendSince  int    `json:"friend_since"`
	Relationship string `json:"relationship"`
}

// FriendsList contains an array of Friend objects
type FriendsList struct {
	Friends []Friend `json:"friends"`
}

// FriendsResult contains a "friendslist" object with relevant data
type FriendsResult struct {
	FriendsList FriendsList `json:"friendslist"`
}

// Client is the type that owns methods for fetching steam data
type Client struct {
	client       *http.Client
	apiKey       string
	gamesFn      func(ctx context.Context, steamID string) ([]Game, error)
	categoriesFn func(ctx context.Context, appID int) ([]string, error)
}

// NoGamesError is returned when one or more users have no owned games.
type NoGamesError struct {
	SteamIDs []string
}

// Error returns a descriptive error string containing the empty users' Steam IDs.
func (e *NoGamesError) Error() string {
	if e == nil {
		return "no games"
	}
	return fmt.Sprintf("no games found for Steam IDs: %s", strings.Join(e.SteamIDs, ", "))
}

// NewClient returns a client struct configured with the provided Steam web API Key
func NewClient(apiKey string) *Client {
	return &Client{
		client: &http.Client{},
		apiKey: apiKey,
	}
}

// doGet builds, sends, and decodes every Steam API request.
func doGet[T any](ctx context.Context, s *Client, service, endpoint string, params url.Values) (T, error) {
	var zero T
	params.Set("key", s.apiKey)
	reqURL := fmt.Sprintf("%s/%s/%s?%s", baseURL, service, endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return zero, fmt.Errorf("steamapi: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return zero, fmt.Errorf("steamapi: performing request: %w", err)
	}
	defer res.Body.Close()
	var out T
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return zero, fmt.Errorf("steamapi: decoding response: %w", err)
	}
	return out, nil
}

// Players accepts one or more steamIDs and returns a slice of Player.
func (s *Client) Players(ctx context.Context, steamIDs []string) ([]Player, error) {
	result, err := doGet[PlayersResult](ctx, s, userService, "GetPlayerSummaries/v0002", url.Values{
		"steamids": {strings.Join(steamIDs, ",")},
	})
	return result.Response.Players, err
}

// Player accepts one steamID and returns that player's object.
func (s *Client) Player(ctx context.Context, steamID string) (Player, error) {
	players, err := s.Players(ctx, []string{steamID})
	if err != nil {
		return Player{}, err
	}
	if len(players) == 0 {
		return Player{}, fmt.Errorf("no player found for steamid %s", steamID)
	}
	return players[0], nil
}

// Games returns owned games with caller-controlled app info and free-game inclusion.
func (s *Client) Games(ctx context.Context, steamID string, includeAppInfo, includePlayedFreeGames bool) ([]Game, error) {
	appInfo, freeGames := "0", "0"
	if includeAppInfo {
		appInfo = "1"
	}
	if includePlayedFreeGames {
		freeGames = "1"
	}
	result, err := doGet[GamesResult](ctx, s, playerService, "GetOwnedGames/v0001/", url.Values{
		"steamid":                   {steamID},
		"include_appinfo":           {appInfo},
		"include_played_free_games": {freeGames},
	})
	return result.Response.Games, err
}

type appCategory struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

type appDetailsData struct {
	Categories []appCategory `json:"categories"`
}

type appDetailsResult struct {
	Success bool           `json:"success"`
	Data    appDetailsData `json:"data"`
}

// categories fetches category descriptions from the Store's app-ID-keyed envelope.
func (s *Client) categories(ctx context.Context, appID int) ([]string, error) {
	reqURL := fmt.Sprintf("%s/api/appdetails?appids=%d", storeBaseURL, appID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("steamapi: building request: %w", err)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steamapi: performing request: %w", err)
	}
	defer res.Body.Close()
	var out map[string]appDetailsResult
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("steamapi: decoding response: %w", err)
	}
	details := out[fmt.Sprint(appID)]
	if !details.Success {
		return nil, nil
	}
	categories := make([]string, 0, len(details.Data.Categories))
	for _, category := range details.Data.Categories {
		categories = append(categories, category.Description)
	}
	return categories, nil
}

func isMultiplayer(categories []string) bool {
	multiplayerCategories := map[string]struct{}{
		"Multi-player":        {},
		"Co-op":               {},
		"Online Co-op":        {},
		"PvP":                 {},
		"Online PvP":          {},
		"Shared/Split Screen": {},
	}
	for _, category := range categories {
		if _, ok := multiplayerCategories[category]; ok {
			return true
		}
	}
	return false
}

// SharedGames returns multiplayer-family games owned by every provided Steam ID.
func (s *Client) SharedGames(ctx context.Context, callerID string, otherIDs ...string) ([]Game, error) {
	steamIDs := append([]string{callerID}, otherIDs...)

	gamesByUser := make([][]Game, len(steamIDs))
	fetchGames := s.gamesFn
	if fetchGames == nil {
		fetchGames = func(ctx context.Context, steamID string) ([]Game, error) {
			return s.Games(ctx, steamID, true, true)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error
	for i, steamID := range steamIDs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			games, err := fetchGames(ctx, steamID)
			if err != nil {
				errOnce.Do(func() {
					firstErr = err
					cancel()
				})
				return
			}
			gamesByUser[i] = games
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	var emptySteamIDs []string
	for i, games := range gamesByUser {
		if len(games) == 0 {
			emptySteamIDs = append(emptySteamIDs, steamIDs[i])
		}
	}
	if len(emptySteamIDs) > 0 {
		return nil, &NoGamesError{SteamIDs: emptySteamIDs}
	}

	ownedCounts := make(map[int]int)
	for _, games := range gamesByUser {
		seen := make(map[int]bool)
		for _, game := range games {
			if seen[game.AppID] {
				continue
			}
			seen[game.AppID] = true
			ownedCounts[game.AppID]++
		}
	}

	sharedGames := make([]Game, 0)
	for _, game := range gamesByUser[0] {
		if ownedCounts[game.AppID] == len(gamesByUser) {
			sharedGames = append(sharedGames, game)
		}
	}

	fetchCategories := s.categoriesFn
	if fetchCategories == nil {
		fetchCategories = s.categories
	}
	multiplayerGames := make([]Game, 0, len(sharedGames))
	for _, game := range sharedGames {
		categories, err := fetchCategories(ctx, game.AppID)
		if err != nil {
			return nil, err
		}
		if isMultiplayer(categories) {
			multiplayerGames = append(multiplayerGames, game)
		}
	}
	return multiplayerGames, nil
}

// Friends returns the raw friend list without fetching player summaries.
func (s *Client) Friends(ctx context.Context, steamID string) ([]Friend, error) {
	result, err := doGet[FriendsResult](ctx, s, userService, "GetFriendList/v0001/", url.Values{
		"steamid":      {steamID},
		"relationship": {"friend"},
	})
	return result.FriendsList.Friends, err
}

// NewsItem contains a Steam news article for an app.
type NewsItem struct {
	GID           string `json:"gid"`
	Title         string `json:"title"`
	URL           string `json:"url"`
	IsExternalURL bool   `json:"is_external_url"`
	Author        string `json:"author"`
	Contents      string `json:"contents"`
	FeedLabel     string `json:"feedlabel"`
	Date          int    `json:"date"`
	FeedName      string `json:"feedname"`
	FeedType      int    `json:"feed_type"`
	AppID         int    `json:"appid"`
}

// AppNews contains the news items and their app's metadata.
type AppNews struct {
	AppID     int        `json:"appid"`
	NewsItems []NewsItem `json:"newsitems"`
	Count     int        `json:"count"`
}

// NewsResult contains the GetNewsForApp response envelope.
type NewsResult struct {
	AppNews AppNews `json:"appnews"`
}

// GlobalAchievement contains an achievement's global completion percentage.
type GlobalAchievement struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

// GlobalAchievementsList contains global achievement percentages.
type GlobalAchievementsList struct {
	Achievements []GlobalAchievement `json:"achievements"`
}

// GlobalAchievementsResult contains the global achievement response envelope.
type GlobalAchievementsResult struct {
	AchievementPercentages GlobalAchievementsList `json:"achievementpercentages"`
}

// PlayerAchievement contains a player's achievement and optional localized text.
type PlayerAchievement struct {
	APIName     string `json:"apiname"`
	Achieved    int    `json:"achieved"`
	UnlockTime  int    `json:"unlocktime"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// UserStat contains a player's named integer statistic.
type UserStat struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// PlayerStats contains a player's statistics and achievements for one game.
type PlayerStats struct {
	SteamID      string              `json:"steamID"`
	GameName     string              `json:"gameName"`
	Stats        []UserStat          `json:"stats"`
	Achievements []PlayerAchievement `json:"achievements"`
	Success      bool                `json:"success"`
}

// PlayerStatsResult contains the player statistics response envelope.
type PlayerStatsResult struct {
	PlayerStats PlayerStats `json:"playerstats"`
}

// GetNewsForApp returns the news items for an app.
func (s *Client) GetNewsForApp(ctx context.Context, appid string) ([]NewsItem, error) {
	result, err := doGet[NewsResult](ctx, s, newsService, "GetNewsForApp/v0002", url.Values{
		"appid": {appid},
	})
	return result.AppNews.NewsItems, err
}

// GetGlobalAchievementPercentagesForApp returns global achievement completion percentages.
func (s *Client) GetGlobalAchievementPercentagesForApp(ctx context.Context, appid string) ([]GlobalAchievement, error) {
	result, err := doGet[GlobalAchievementsResult](ctx, s, userStatsService, "GetGlobalAchievementPercentagesForApp/v0002", url.Values{
		"gameid": {appid},
	})
	return result.AchievementPercentages.Achievements, err
}

// GetPlayerAchievements returns a player's achievements for an app.
func (s *Client) GetPlayerAchievements(ctx context.Context, steamID, appid string) ([]PlayerAchievement, error) {
	result, err := doGet[PlayerStatsResult](ctx, s, userStatsService, "GetPlayerAchievements/v0001", url.Values{
		"steamid": {steamID},
		"appid":   {appid},
	})
	return result.PlayerStats.Achievements, err
}

// GetUserStatsForGame returns a player's statistics for an app.
func (s *Client) GetUserStatsForGame(ctx context.Context, steamID, appid string) ([]UserStat, error) {
	result, err := doGet[PlayerStatsResult](ctx, s, userStatsService, "GetUserStatsForGame/v0002", url.Values{
		"steamid": {steamID},
		"appid":   {appid},
	})
	return result.PlayerStats.Stats, err
}

// GetRecentlyPlayedGames returns a player's recently played games.
func (s *Client) GetRecentlyPlayedGames(ctx context.Context, steamID string) ([]Game, error) {
	result, err := doGet[GamesResult](ctx, s, playerService, "GetRecentlyPlayedGames/v0001", url.Values{
		"steamid": {steamID},
	})
	return result.Response.Games, err
}
