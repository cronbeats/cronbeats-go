package main

import (
	"fmt"
	"log"

	cronbeatsgo "github.com/cronbeats/cronbeats-go"
)

func main() {
	client, err := cronbeatsgo.NewPingClient("YCrXzYbV", nil)
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(res.Ok)
	fmt.Println(res.Action)
	fmt.Println(res.JobKey)
}
