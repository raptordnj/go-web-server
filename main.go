package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	// Detect PHP version dynamically by matching the PHP-FPM socket version
	phpVersion := "PHP"
	phpBin := "php"
	
	// Check if the FPM address gives us a hint about the version (like php8.5-fpm.sock)
	resolvedAddress := fpmAddress
	if fpmNetwork == "unix" {
		if resolved, err := filepath.EvalSymlinks(fpmAddress); err == nil {
			resolvedAddress = resolved
		}
	}

	if strings.Contains(resolvedAddress, "8.5") {
		phpBin = "php8.5"
	} else if strings.Contains(resolvedAddress, "8.4") {
		phpBin = "php8.4"
	}

	if out, err := exec.Command(phpBin, "-r", "echo phpversion();").Output(); err == nil {
		re := regexp.MustCompile(`\d+\.\d+\.\d+`)
		if match := re.FindString(string(out)); match != "" {
			phpVersion = "PHP/" + match
		}
	}

	// Main routing handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Set custom headers
		w.Header().Set("Server", "Go-Web-Server")
		w.Header().Set("X-Powered-By", phpVersion)

		// Serve all responses from PHP-FPM (Front Controller pattern)
		// We route everything to /index.php, preserving the original RequestURI for PHP to read
		r.URL.Path = "/index.php"
		phpHandler.ServeHTTP(w, r)
	})

	port := ":8080"
	log.Printf("Starting server on http://localhost%s\n", port)
	log.Printf("Routing ALL requests to %s/index.php\n", absStaticDir)
	log.Printf("Proxying PHP requests to FastCGI on %s:%s\n", fpmNetwork, fpmAddress)

	// Start the web server
	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}
