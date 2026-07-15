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
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port      string `yaml:"port"`
		StaticDir string `yaml:"static_dir"`
	} `yaml:"server"`
	FPM struct {
		Network string `yaml:"network"`
		Address string `yaml:"address"`
	} `yaml:"fpm"`
}

func loadConfig() Config {
	// Setup default configuration
	cfg := Config{}
	cfg.Server.Port = "8080"
	cfg.Server.StaticDir = "./static"

	// Try loading from config.yaml
	data, err := os.ReadFile("config.yaml")
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Printf("Warning: failed to parse config.yaml: %v", err)
		}
	}

	// Override with environment variables if present
	if p := os.Getenv("PORT"); p != "" {
		cfg.Server.Port = p
	}
	if fn := os.Getenv("FPM_NETWORK"); fn != "" {
		cfg.FPM.Network = fn
	}
	if fa := os.Getenv("FPM_ADDRESS"); fa != "" {
		cfg.FPM.Address = fa
	}

	return cfg
}

func main() {
	cfg := loadConfig()

	// Ensure port starts with colon
	port := cfg.Server.Port
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	staticDir := cfg.Server.StaticDir
	// Ensure the static directory path is absolute. PHP-FPM needs absolute paths.
	absStaticDir, err := filepath.Abs(staticDir)
	if err != nil {
		log.Fatal(err)
	}

	fpmNetwork := cfg.FPM.Network
	fpmAddress := cfg.FPM.Address

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

	log.Printf("Starting server on http://localhost%s\n", port)
	log.Printf("Routing ALL requests to %s/index.php\n", absStaticDir)
	log.Printf("Proxying PHP requests to FastCGI on %s:%s\n", fpmNetwork, fpmAddress)

	// Start the web server
	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}
