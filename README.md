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

The v1.0.0 release includes breaking changes: `Friends` now returns `[]Friend`, and the `Games` signature/options have changed. These changes ship under the new major version v1.0.0, not a retag of v0.1.0. Tags and GitHub releases are now published automatically after the main-branch release gate succeeds.

## Release automation

On pushes to `main`, the Release workflow runs build, vet, and tests before its
`release` job can run. That job reads the latest reachable tag and all full commit
messages since it. Breaking headers (`feat!:`, `fix(api)!:`, etc.) or
`BREAKING CHANGE:` / `BREAKING-CHANGE:` footer lines bump major; otherwise `feat`
bumps minor, then `fix` bumps patch. If no significance is recognizable, the
fallback is **minor**, including chore/docs-only and legacy messages. Major and
minor bumps reset the lower components. Tags must be stable `vMAJOR.MINOR.PATCH`
versions; this tooling adds no version file or module dependencies.

Publication is serialized. An already-tagged commit is skipped before computing
another version, and an existing computed tag skips both tag creation and release
publication. `gh release create` creates the missing tag at the workflow's exact
commit and publishes its GitHub release using `GITHUB_TOKEN`. If publication fails
after creating a tag, retries intentionally do not repair that release: maintainers
must inspect and recover it manually rather than bypass the existing-tag guard.

The Commit lint workflow validates every commit in a PR to `main`, not just its
title. Use `type(optional-scope)optional-!: description`, with one of `feat`, `fix`,
`docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, or `revert`.
For example, `feat(steamapi): add method` is valid; `Add method` is not. Repository
administrators must make **Validate PR commits** a required check in the `main`
branch ruleset to block merging; workflow files alone cannot configure that rule.

Both workflows call stdlib-only Go tooling. Its stdin protocol is **NUL-separated
full commit messages**, with an optional final NUL (newlines stay inside messages):

```sh
git log -z --format=%B v1.0.2..HEAD | go run ./cmd/release-tool next-version v1.0.2
git log -z --format=%B origin/main..HEAD | go run ./cmd/release-tool validate-commits
```

`next-version` prints the version; `validate-commits` prints quoted invalid messages
and exits nonzero if any fail. Invalid arguments and unreadable input also fail.

### Deferred live verification

Go tests cover version computation, validation, CLI framing, and a static check of
the release gate. Live Actions evidence remains deferred for CLM-2/5/6/7. In an
authorized disposable GitHub repository, verify these scenarios before treating
those runtime guarantees as proven:

- **CLM-2:** push a deliberate build, vet, or test failure to main; check that the
  release job is skipped and the before/after tag and release lists are identical.
- **CLM-5:** push a valid feature commit; check that its gate succeeds, the expected
  next tag points at the triggering SHA, and a GitHub release exists for that tag.
- **CLM-6:** rerun that successful workflow, then separately pre-create the computed
  tag on another commit and trigger publication; both cases must leave the tag and
  release lists unchanged.
- **CLM-7:** open a PR containing both a valid commit and `change case`; verify
  **Validate PR commits** fails and the configured branch ruleset blocks merging.
  Rewrite the malformed message to `fix: change case` and verify the check succeeds.
