package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/hauke96/sigolo"
)

const configPath = "./tiny.json"

var config *Config
var cache *Cache

var client *http.Client

func main() {
	prepare()

	sigolo.Info("Ready to serve")

	port := os.Getenv("PORT")
	if port == "" {
		fmt.Println("$PORT must be set")
		return
	}

	server := &http.Server{
		Addr:         ":" + port,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
		Handler:      http.HandlerFunc(handleGet),
	}

	err := server.ListenAndServe()
	if err != nil {
		sigolo.Fatal(err.Error())
	}
}

func configureLogging() {
	sigolo.FormatFunctions[sigolo.LOG_INFO] = sigolo.LogPlain
	//sigolo.LogLevel = sigolo.LOG_DEBUG
}

func prepare() {
	var err error

	sigolo.Info("Load config")
	config, err = LoadConfig(configPath)
	if err != nil {
		sigolo.Fatal("Could not read config: '%s'", err.Error())
	}

	sigolo.Info("Init cache")
	cache, err = CreateCache(config.CacheFolder)

	if err != nil {
		sigolo.Fatal("Could not init cache: '%s'", err.Error())
	}

	client = &http.Client{
		Timeout: time.Second * 30,
	}
}

func handleGet(w http.ResponseWriter, r *http.Request) {
	fullUrl := r.URL.Path + "?" + r.URL.RawQuery

	sigolo.Info("Requested '%s'", fullUrl)

	// Only pass request to target host when cache does not has an entry for the
	// given URL.
	if cache.has(fullUrl) {
		content, err := cache.get(fullUrl)

		if err != nil {
			handleError(err, w)
		} else {
			w.Write(content)
		}
	} else {
		response, err := client.Get(config.Target + fullUrl)
		if err != nil {
			handleError(err, w)
			return
		}

		body, err := ioutil.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			handleError(err, w)
			return
		}

		if response.StatusCode < 200 || response.StatusCode >= 300 {
			goto write
		}
		err = cache.put(fullUrl, body)
		if err != nil {
			sigolo.Error("Could not write into cache: %s", err)
			handleError(err, w)
			return
		}
	write:
		w.Write(body)
	}
}

func handleError(err error, w http.ResponseWriter) {
	sigolo.Error(err.Error())
	w.WriteHeader(500)
	fmt.Fprintf(w, err.Error())
}
