package main

import (
	"fmt"
	"log"
	"net/http"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<h1>Welcome home</h1>"))
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<h1>About page</h1>"))
}

func catchAll(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<h1>Welcome</h1>"))
}

func main() {
	// Create your Multiplexer
	mux := http.NewServeMux()

	// Register Paths to the Multiplexer
	mux.HandleFunc("/home", homeHandler)
	mux.HandleFunc("/about", aboutHandler)
	mux.HandleFunc("/", catchAll)

	// 3. Supply the mux (which satisfies http.Handler) to ListenAndServe()
	fmt.Println("Listening on port 8080...")
	err := http.ListenAndServe(":8080", mux)
	if err != nil {
		log.Fatal(err)
	}
}

