package main

import (
	"log"
	"net/http"
)

func main() {
	// Directory to serve static files from.
	// In this case, a directory named "static" in the same folder as the executable.
	staticDir := "./static"

	// Create a file server handler to serve the directory
	fileServer := http.FileServer(http.Dir(staticDir))

	// Register the file server to handle all requests to the root ("/") and sub-paths.
	// We use http.Handle to map the root path to the file server.
	http.Handle("/", fileServer)

	// Alternatively, if you wanted to serve files under a specific path like /assets/
	// you would use http.StripPrefix:
	// http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))

	port := ":8080"
	log.Printf("Starting server on http://localhost%s\n", port)
	log.Printf("Serving static files from %s directory\n", staticDir)
	
	// Start the web server
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}
