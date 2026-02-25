# CronBeats Go SDK (Ping)

[![Go Reference](https://pkg.go.dev/badge/github.com/cronbeats/cronbeats-go.svg)](https://pkg.go.dev/github.com/cronbeats/cronbeats-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/cronbeats/cronbeats-go)](https://goreportcard.com/report/github.com/cronbeats/cronbeats-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Cron job monitoring and heartbeat monitoring SDK for Go. Monitor scheduled tasks, background jobs, and cron jobs with simple ping telemetry. Get alerts when cron jobs fail, miss their schedule, or run too long.

## Install (local/dev)

```bash
go get github.com/cronbeats/cronbeats-go
```

## Quick Usage

```go
package main

import (
	"log"

	cronbeatsgo "github.com/cronbeats/cronbeats-go"
)

func main() {
	client, err := cronbeatsgo.NewPingClient("abc123de", nil)
	if err != nil {
		log.Fatal(err)
	}

	_, _ = client.Start()
	// ...your work...
	_, _ = client.Success()
}
```

## Real-World Cron Job Example

```go
package main

import (
	"log"

	cronbeatsgo "github.com/cronbeats/cronbeats-go"
)

func runCronTask() error {
	return nil
}

func main() {
	client, err := cronbeatsgo.NewPingClient("abc123de", nil)
	if err != nil {
		log.Fatal(err)
	}

	_, _ = client.Start()

	if err := runCronTask(); err != nil {
		_, _ = client.Fail()
		log.Fatal(err)
	}

	_, _ = client.Success()
}
```

## Progress Updates

```go
seq := 50
_, _ = client.Progress(cronbeatsgo.ProgressOptions{
	Seq:     &seq,
	Message: "Processing batch 50/100",
})
```

## Notes

- SDK uses `POST` for telemetry requests.
- `jobKey` must be exactly 8 Base62 characters.
- Retries happen only for network errors, HTTP `429`, and HTTP `5xx`.
- Default 5s timeout ensures the SDK never blocks your cron job if CronBeats is unreachable.
