# Go Caching Proxy Server

A simple command-line caching proxy server built in Go, designed to sit in front of an origin server and cache its responses to speed up subsequent requests. This project was developed as a hands-on implementation of the Caching Server project from [roadmap.sh/projects/caching-server](https://roadmap.sh/projects/caching-server).

## Features

* **HTTP Proxying**: Forwards incoming HTTP GET requests to a specified origin server.
* **Response Caching**: Caches successful (2xx status code) responses from the origin server in-memory.
* **Cache Hit/Miss Indication**: Adds an `X-Cache` header to responses to indicate whether the content was served from cache (`HIT`) or fetched from the origin (`MISS`).
* **Command-Line Interface**: Configurable via command-line arguments for port and origin URL.
* **Cache Clearing**: Provides a command-line option to clear the in-memory cache.
* **Robust Cache Key**: Uses a normalized cache key (HTTP method + path + sorted query parameters) to ensure consistent caching.

## Requirements

* Go 1.16 or higher

## Installation

1.  **Clone the repository (or create the files manually):**
    ```bash
    git clone https://github.com/AugustoRamos94/caching-proxy.git
    cd caching-proxy
    ```

2.  **Build the executable:**
    ```bash
    go build -o caching-proxy
    ```

## Usage

The caching proxy server is a CLI tool that can be started with the `--port` and `--origin` arguments.

### Starting the Server

To start the server, specify the port it should listen on and the URL of the origin server you want to proxy.

```bash
./caching-proxy --port 8080 --origin [http://jsonplaceholder.typicode.com](http://jsonplaceholder.typicode.com)
