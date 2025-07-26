package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type CachedResponse struct {
	Response   []byte
	StatusCode int
	Headers    http.Header
	Timestamp  time.Time
}

var cache = make(map[string]*CachedResponse)
var cacheMutex sync.Mutex
var globalOriginURL *url.URL

func main() {
	port := flag.Int("port", 8080, "Port to run the caching proxy server on")
	originStr := flag.String("origin", "", "URL of the origin server")
	clearCache := flag.Bool("clear-cache", false, "Clear the cache and exit")

	flag.Parse()

	if *clearCache {
		fmt.Println("Clearing cache...")
		cacheMutex.Lock()
		cache = make(map[string]*CachedResponse)
		cacheMutex.Unlock()
		fmt.Println("Cache cleared successfully.")
		return
	}

	if *originStr == "" {
		log.Fatal("--origin URL is required")
	}

	var err error
	globalOriginURL, err = url.Parse(*originStr)
	if err != nil {
		log.Fatalf("Invalid origin URL: %v", err)
	}

	log.Printf("Starting caching proxy on :%d, forwarding to %s", *port, globalOriginURL.String())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), createProxyHandler(globalOriginURL)))
}

func createProxyHandler(originURL *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(originURL)

	proxy.ModifyResponse = func(resp *http.Response) error {
		cacheKey := generateCacheKey(resp.Request)
		log.Printf("[ModifyResponse] Processing response for cacheKey: '%s'", cacheKey)

		// Read the entire response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[ModifyResponse] Failed to read response body for cacheKey '%s': %v", cacheKey, err)
			return fmt.Errorf("failed to read response body for caching: %w", err)
		}
		// IMPORTANT: Restore the body for subsequent reads (i.e., for the proxy to send it to the client)
		resp.Body = io.NopCloser(bytes.NewBuffer(body))

		// Only cache successful responses (2xx range)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			cacheMutex.Lock()
			cache[cacheKey] = &CachedResponse{
				Response:   body,
				StatusCode: resp.StatusCode,
				Headers:    resp.Header, // Capture ALL headers from the origin response
				Timestamp:  time.Now(),
			}
			cacheMutex.Unlock()
			log.Printf("[ModifyResponse] Successfully cached response for cacheKey: '%s' (Status: %d, Size: %d bytes)", cacheKey, resp.StatusCode, len(body))
		} else {
			log.Printf("[ModifyResponse] Not caching response for cacheKey: '%s' (Status: %d, not a 2xx success)", cacheKey, resp.StatusCode)
		}

		return nil
	}

	// Director modifies the request before it's sent to the origin.
	proxy.Director = func(req *http.Request) {
		req.URL.Host = originURL.Host
		req.URL.Scheme = originURL.Scheme
		req.Host = originURL.Host // Crucial for many origin servers (virtual hosts)
		req.Header.Del("X-Cache") // Ensure no X-Cache header is forwarded to origin
		log.Printf("[Director] Forwarding request to origin: %s %s", req.Method, req.URL.String())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For simplicity, we only cache GET requests.
		if r.Method != http.MethodGet {
			log.Printf("[Handler] Non-GET request (%s) for %s, bypassing cache.", r.Method, r.URL.String())
			w.Header().Set("X-Cache", "BYPASS") // Indicate bypass for clarity
			proxy.ServeHTTP(w, r)
			return
		}

		// Generate the cache key using the consistent function
		cacheKey := generateCacheKey(r)
		log.Printf("[Handler] Incoming request for cacheKey: '%s'", cacheKey)

		// Try to serve from cache first
		cacheMutex.Lock()
		cachedResp, found := cache[cacheKey]
		cacheMutex.Unlock()

		if found {
			log.Printf("[Handler] Cache HIT for cacheKey: '%s'", cacheKey)
			w.Header().Set("X-Cache", "HIT")
			// Copy all headers from the cached response
			for k, vv := range cachedResp.Headers {
				// Avoid adding hop-by-hop headers that are specific to the origin connection
				// (e.g., Connection, Transfer-Encoding)
				if k == "Connection" || k == "Transfer-Encoding" {
					continue
				}
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			// Explicitly set Content-Length from the cached response body
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(cachedResp.Response)))
			w.WriteHeader(cachedResp.StatusCode)
			w.Write(cachedResp.Response)
			return
		}

		// If not in cache, forward to origin
		log.Printf("[Handler] Cache MISS for cacheKey: '%s'. Forwarding to origin.", cacheKey)
		w.Header().Set("X-Cache", "MISS")

		proxy.ServeHTTP(w, r)
	})
}

func generateCacheKey(r *http.Request) string {
	params := r.URL.Query()
	if len(params) == 0 {
		return r.Method + ":" + r.URL.Path
	}

	// Sort query parameters for consistent key generation
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var queryParts []string
	for _, k := range keys {
		for _, v := range params[k] {
			queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	sortedQuery := strings.Join(queryParts, "&")
	return r.Method + ":" + r.URL.Path + "?" + sortedQuery
}
