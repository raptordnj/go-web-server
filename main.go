package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yookoala/gofast"
	"gopkg.in/yaml.v3"
)

type VHostConfig struct {
	StaticDir  string `yaml:"static_dir"`
	FpmNetwork string `yaml:"fpm_network"`
	FpmAddress string `yaml:"fpm_address"`
}

type Config struct {
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	DefaultVHost VHostConfig            `yaml:"default_vhost"`
	VHosts       map[string]VHostConfig `yaml:"vhosts"`
}

type VHost struct {
	AbsStaticDir string
	PhpHandler   http.Handler
	PhpVersion   string
}

func loadConfig() Config {
	cfg := Config{}
	cfg.Server.Port = "8080"
	cfg.DefaultVHost.StaticDir = "./static"

	data, err := os.ReadFile("config.yaml")
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Printf("Warning: failed to parse config.yaml: %v", err)
		}
	}

	if p := os.Getenv("PORT"); p != "" {
		cfg.Server.Port = p
	}

	return cfg
}

func setupVHost(vc VHostConfig) (*VHost, error) {
	if vc.StaticDir == "" {
		vc.StaticDir = "./static" // Fallback
	}
	absStaticDir, err := filepath.Abs(vc.StaticDir)
	if err != nil {
		return nil, err
	}
	// Create directory if it doesn't exist to prevent crashes
	os.MkdirAll(absStaticDir, 0755)

	fpmNetwork := vc.FpmNetwork
	fpmAddress := vc.FpmAddress

	if fpmNetwork == "" || fpmAddress == "" {
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
		if !foundSocket {
			fpmNetwork = "tcp"
			fpmAddress = "127.0.0.1:9000"
		}
	}

	connFactory := gofast.SimpleConnFactory(fpmNetwork, fpmAddress)
	clientFactory := gofast.SimpleClientFactory(connFactory)
	phpHandler := gofast.NewHandler(
		gofast.NewPHPFS(absStaticDir)(gofast.BasicSession),
		clientFactory,
	)

	phpVersion := "PHP"
	phpBin := "php"

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

	return &VHost{
		AbsStaticDir: absStaticDir,
		PhpHandler:   phpHandler,
		PhpVersion:   phpVersion,
	}, nil
}

func main() {
	cfg := loadConfig()

	port := cfg.Server.Port
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	defaultVH, err := setupVHost(cfg.DefaultVHost)
	if err != nil {
		log.Fatalf("Failed to setup default vhost: %v", err)
	}

	vhosts := make(map[string]*VHost)
	for host, vc := range cfg.VHosts {
		vh, err := setupVHost(vc)
		if err != nil {
			log.Printf("Warning: Failed to setup vhost %s: %v", host, err)
			continue
		}
		vhosts[host] = vh
		log.Printf("Loaded VHost: %s (Dir: %s)", host, vh.AbsStaticDir)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if strings.Contains(host, ":") {
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
		}

		vh, ok := vhosts[host]
		if !ok {
			vh = defaultVH
		}

		w.Header().Set("Server", "Go-Web-Server")
		w.Header().Set("X-Powered-By", vh.PhpVersion)

		r.URL.Path = "/index.php"
		vh.PhpHandler.ServeHTTP(w, r)
	})

	log.Printf("Starting server on http://localhost%s\n", port)
	log.Printf("Default VHost routing ALL requests to %s/index.php\n", defaultVH.AbsStaticDir)

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}
