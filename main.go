package main

import (
	"flag"
	"log"
	"os"
	"strings"
)

type Scraper interface {
	Scrape(url string)
}

func main() {
	var url string
	flag.StringVar(&url, "url", "", "URL of the website")
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
	default:
		log.Fatal("Domain is not supported")
		os.Exit(1)
	}
	scraper.Scrape(url)
}
