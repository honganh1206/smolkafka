package main

import (
	"log"

	"github.com/honganh1206/smolkafka/internal/server"
)

func main() {
	srv := server.NewHTTPServer(":8080")
	log.Fatal(srv.ListenAndServe())
}
