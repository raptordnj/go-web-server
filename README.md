# Go Static Web Server

A simple and efficient web server written in Golang using the standard `net/http` library. It can serve static files such as HTML, CSS, JavaScript, images, and videos.

## Getting Started

### Prerequisites
- [Go](https://golang.org/doc/install) installed on your machine.

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

## Adding Static Files
Any files you place inside the `static` directory will automatically be served by the web server. For instance, if you add an image `logo.png` to the `static` directory, you can access it via `http://localhost:8080/logo.png`.
