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

## Progress Tracking

Track your job's progress in real-time. CronBeats supports two distinct modes:

### Mode 1: With Percentage (0-100)
Shows a **progress bar** and your status message on the dashboard.

✓ **Use when**: You can calculate meaningful progress (e.g., processed 750 of 1000 records)

```go
// Percentage mode: 0-100 with message
seq := 50
_, _ = client.Progress(cronbeatsgo.ProgressOptions{
	Seq:     &seq,
	Message: "Processing batch 500/1000",
})

seq75 := 75
_, _ = client.Progress(cronbeatsgo.ProgressOptions{
	Seq:     &seq75,
	Message: "Almost done - 750/1000",
})
```

### Mode 2: Message Only
Shows **only your status message** (no percentage bar) on the dashboard.

✓ **Use when**: Progress isn't measurable or you only want to send status updates

```go
// Message-only mode: nil seq, just status updates
_, _ = client.Progress(cronbeatsgo.ProgressOptions{
	Seq:     nil,
	Message: "Connecting to database...",
})

_, _ = client.Progress(cronbeatsgo.ProgressOptions{
	Seq:     nil,
	Message: "Starting data sync...",
})
```

### What you see on the dashboard
- **Mode 1**: Progress bar (0-100%) + your message → "75% - Processing batch 750/1000"
- **Mode 2**: Only your status message → "Connecting to database..."

### Complete Example

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

	// Message-only updates for non-measurable steps
	_, _ = client.Progress(cronbeatsgo.ProgressOptions{
		Seq:     nil,
		Message: "Connecting to database...",
	})

	db := connectToDatabase()

	_, _ = client.Progress(cronbeatsgo.ProgressOptions{
		Seq:     nil,
		Message: "Fetching records...",
	})

	total := db.Count()

	// Percentage updates for measurable progress
	for i := 0; i < total; i++ {
		processRecord(i)

		if i%100 == 0 {
			percent := (i * 100) / total
			_, _ = client.Progress(cronbeatsgo.ProgressOptions{
				Seq:     &percent,
				Message: fmt.Sprintf("Processed %d / %d records", i, total),
			})
		}
	}

	seq100 := 100
	_, _ = client.Progress(cronbeatsgo.ProgressOptions{
		Seq:     &seq100,
		Message: "All records processed",
	})

	_, _ = client.Success()
}
```

## Notes

- SDK uses `POST` for telemetry requests.
- `jobKey` must be exactly 8 Base62 characters.
- Retries happen only for network errors, HTTP `429`, and HTTP `5xx`.
- Default 5s timeout ensures the SDK never blocks your cron job if CronBeats is unreachable.
