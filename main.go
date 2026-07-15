package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yookoala/gofast"
)

func main() {
	staticDir := "./static"
	// Ensure the static directory path is absolute. PHP-FPM needs absolute paths.
	absStaticDir, err := filepath.Abs(staticDir)
	if err != nil {
		log.Fatal(err)
	}

	// Read FPM connection settings from environment, or try to auto-detect a Unix socket, then default to TCP.
	fpmNetwork := os.Getenv("FPM_NETWORK")
	fpmAddress := os.Getenv("FPM_ADDRESS")
	
	if fpmNetwork == "" || fpmAddress == "" {
		// Try auto-detecting common Unix sockets on Linux
		commonSockets := []string{
			"/run/php/php-fpm.sock",
			"/run/php/php8.5-fpm.sock",
			"/run/php/php8.4-fpm.sock",
		}
		
		foundSocket := false
		for _, sockPath := range commonSockets {
			if _, err := os.Stat(sockPath); err == nil {
				fpmNetwork = "unix"
				fpmAddress = sockPath
				foundSocket = true
				break
			}
		}

		// Fallback to TCP if no Unix socket is found
		if !foundSocket {
			fpmNetwork = "tcp"
			fpmAddress = "127.0.0.1:9000"
		}
	}

	// Create a FastCGI client factory
	connFactory := gofast.SimpleConnFactory(fpmNetwork, fpmAddress)
	clientFactory := gofast.SimpleClientFactory(connFactory)

	// Create a PHP handler using gofast middleware
	phpHandler := gofast.NewHandler(
		gofast.NewPHPFS(absStaticDir)(gofast.BasicSession),
		clientFactory,
	)

	// Standard static file server
	fileServer := http.FileServer(http.Dir(absStaticDir))

	// Main routing handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Map the request URL path to the local filesystem path
		fullPath := filepath.Join(absStaticDir, filepath.FromSlash(path))

		// Check if the requested path is a directory
		info, err := os.Stat(fullPath)
		if err == nil && info.IsDir() {
			// If missing a trailing slash, redirect to ensure relative paths work in the browser
			if !strings.HasSuffix(path, "/") {
				http.Redirect(w, r, path+"/", http.StatusMovedPermanently)
				return
			}

			// List of index files to check, in order of priority
			indexFiles := []string{
				"index.php", "index.html", "index.htm",
				"default.php", "default.html", "default.htm",
			}
			
			for _, indexFile := range indexFiles {
				indexPath := filepath.Join(fullPath, indexFile)
				if _, err := os.Stat(indexPath); err == nil {
					// Found an index file! Update the request path so the downstream handlers can process it
					path += indexFile
					r.URL.Path = path
					break
				}
			}
		}

		// Proxy .php files to PHP-FPM
		if strings.HasSuffix(path, ".php") {
			// Check if file exists to prevent PHP-FPM returning "Primary script unknown"
			if _, err := os.Stat(filepath.Join(absStaticDir, filepath.FromSlash(path))); os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			phpHandler.ServeHTTP(w, r)
			return
		}

		// Serve all other files statically
		fileServer.ServeHTTP(w, r)
	})

	port := ":8080"
	log.Printf("Starting server on http://localhost%s\n", port)
	log.Printf("Serving files from %s directory\n", absStaticDir)
	log.Printf("Proxying PHP requests to FastCGI on %s:%s\n", fpmNetwork, fpmAddress)

	// Start the web server
	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}
