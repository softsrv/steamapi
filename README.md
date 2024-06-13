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

at the moment, only a subset of APIs are supported

- get Players
- get Friends
- get Games
