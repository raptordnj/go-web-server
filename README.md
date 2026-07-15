# Go Web Server

A simple, efficient, and auto-indexing web server written in Golang using the standard `net/http` library. It serves static files (HTML, CSS, JS, images, videos) out of the box and seamlessly proxies `.php` files to PHP-FPM using FastCGI.

## Features
- **Static File Serving**: Fast delivery of static assets.
- **PHP-FPM Support**: Automatically detects and forwards `.php` requests to a PHP-FPM backend.
- **Auto-Indexing**: Requests to directories automatically resolve to `index.php`, `index.html`, `index.htm`, `default.php`, `default.html`, or `default.htm`.
- **Smart Redirects**: Automatically adds trailing slashes to directory paths to prevent broken relative links.

## Getting Started

### Prerequisites
- [Go](https://golang.org/doc/install) installed on your machine.
- (Optional) **PHP-FPM** installed if you intend to serve PHP files.

### Running the Server
1. Clone this repository or download the source code.
2. Navigate to the project directory:
   ```bash
   cd web_server
   ```
3. Run the server:
   ```bash
   go run main.go
   ```
4. Open your web browser and go to `http://localhost:8080`.

## Adding Files
Any files you place inside the `static` directory will automatically be served.
- **Static Files**: Place an image like `logo.png` into the `static` folder, and access it via `http://localhost:8080/logo.png`.
- **PHP Files**: Place a file like `index.php` in the `static` folder, and it will be executed dynamically!

## PHP-FPM Configuration
By default, the server will attempt to auto-detect your PHP-FPM Unix socket (e.g. `/run/php/php-fpm.sock`). If it can't find one, it defaults to a TCP connection on `127.0.0.1:9000`.

You can override these settings using environment variables:
```bash
# Example using a custom TCP port:
FPM_NETWORK="tcp" FPM_ADDRESS="127.0.0.1:9001" go run main.go

# Example using a custom Unix socket:
FPM_NETWORK="unix" FPM_ADDRESS="/var/run/php/php8.4-fpm.sock" go run main.go
```
