package main

import (
	"flag"
	"fmt"
	"time"
)

var (
	rate = flag.Float64("rate", 10, "records per second")
	id   = flag.String("id", "generator", "`id` field value")
)

func main() {
	flag.Parse()

	var j int64
	interval := time.Duration(1 / *rate * float64(time.Second))

	for range time.Tick(interval) {
		j++
		fmt.Printf(`{"ts":"%v", "message": "%v", "id":"%v"}`+"\n", time.Now().Format(time.RFC3339Nano), j, *id)
	}
}

