package main

import (
	"flag"
	"log"
	"net/http"
)

var (
	port = flag.String("port", ":9090", "Port to listen to")
)

func main() {
	flag.Parse()

	// API documentation
	http.Handle("/documentation/", http.StripPrefix("/documentation/", http.FileServer(http.Dir("./"))))

	log.Printf("API Documentation server started on port %s\n", *port)

	log.Fatalln(http.ListenAndServe(*port, nil))
}
