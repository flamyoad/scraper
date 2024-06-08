package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"strings"
)

type Scraper interface {
	Scrape(url string, headless bool)
}

func main() {
	defer os.Exit(0)

	var url string
	var headless bool
	flag.StringVar(&url, "url", "", "URL of the website")
	flag.BoolVar(&headless, "headless", true, "Run as headless Chrome browser")
	flag.Parse()

	if url == "" {
		log.Fatal("Missing URL. Please provide the url with -url")
		os.Exit(1)
	}
	log.Printf("URL: " + url)

	var scraper Scraper
	switch {
	case strings.Contains(url, "e-hentai.org"):
		scraper = &EhScraper{}
	case strings.Contains(url, "kemono.su"):
		scraper = &KemonoScraper{}
	default:
		log.Fatal("Domain is not supported")
		os.Exit(1)
	}
	scraper.Scrape(url, headless)

	runtime.Goexit()
}
