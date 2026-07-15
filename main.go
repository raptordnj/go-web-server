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
		// Set a custom Server header to identify our web server
		w.Header().Set("Server", "Go-Web-Server")

		path := r.URL.Path
		fullPath := filepath.Join(absStaticDir, filepath.FromSlash(path))
		info, err := os.Stat(fullPath)
		exists := !os.IsNotExist(err)

		// ---------------------------------------------------------
		// Mod_Rewrite logic (similar to Apache's fallback to index.php)
		// If the requested file or directory does NOT exist on disk, 
		// forward the request to /index.php so PHP frameworks (Laravel, 
		// WordPress, etc) can handle the custom route via REQUEST_URI.
		// ---------------------------------------------------------
		if !exists {
			indexPath := filepath.Join(absStaticDir, "index.php")
			if _, errIdx := os.Stat(indexPath); errIdx == nil {
				r.URL.Path = "/index.php"
				phpHandler.ServeHTTP(w, r)
				return
			}
		}

		// Check if the requested path is an existing directory
		if exists && info.IsDir() {
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
