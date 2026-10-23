package main

import (
	"net/http"
)

// Dummy Server Struct. Made just for the ServeHTTP() method
type Server struct {
}

// Function definition that satisfies http.Handler interface. 
func (d Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// r = Request struct. HTTP Request information gets stored here
	// w = Writer interfafce. Write HTTP response.
	// Server Logic:
	w.Write([]byte("<h1>Hi,</h1>"))
}

func main() {
	// Build the Server
	handler := Server{}

	// 1. Builds the TCP sockets for the clients
	// 2. Parse HTTP request and responses
	// 3. Passes the Reader and Writer interface
	// 4. takes a http.Handler interface as input
	http.ListenAndServe(":8080", handler)
}
