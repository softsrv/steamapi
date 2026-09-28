# steamapi

steamapi is a Go client library for interacting with the [Steam Web API](https://developer.valvesoftware.com/wiki/Steam_Web_API)

requires Go version 1.22 or later

## Installation

go get github.com/softsrv/steamapi

## Usage

```
import "github.com/softsrv/steamapi
```

create a new client with your [Steam web API key](https://steamcommunity.com/dev)

```
 steamClient := steamapi.NewClient(os.Getenv("STEAM_API_KEY"))
```

## Supported Requests

steamapi v1.0.0 supports a subset of the Steam Web API:

- `Players` — batch GetPlayerSummaries for multiple steamIDs, returning `[]Player` with the full player field set, including realname, timecreated, lastlogoff, personastate, loccountrycode, gameid, and primaryclanid.
- `Player` — GetPlayerSummaries convenience method for one steamID, returning a `Player`.
- `Friends` — GetFriendList for a steamID, returning the raw `[]Friend` list (steamid, relationship, friend_since). It no longer hydrates friends into player summaries; to get names or avatars, call `Players` with the returned steamIDs.
- `Games` — GetOwnedGames for a steamID, returning `[]Game` with enriched fields including playtime_2weeks. Callers control the include_appinfo and include_played_free_games options.
- `SharedGames` — intersection of games owned by multiple users.
- `GetNewsForApp` — ISteamNews/GetNewsForApp news items for an appid.
- `GetGlobalAchievementPercentagesForApp` — ISteamUserStats/GetGlobalAchievementPercentagesForApp global achievement percentages for an appid.
- `GetPlayerAchievements` — ISteamUserStats/GetPlayerAchievements achievements for a player and game.
- `GetUserStatsForGame` — ISteamUserStats/GetUserStatsForGame stats for a player and game.
- `GetRecentlyPlayedGames` — IPlayerService/GetRecentlyPlayedGames recently played games for a player.

The v1.0.0 release includes breaking changes: `Friends` now returns `[]Friend`, and the `Games` signature/options have changed. These changes ship under the new major version v1.0.0, not a retag of v0.1.0. Cutting the actual git tag and publishing the GitHub release are post-merge human steps.
