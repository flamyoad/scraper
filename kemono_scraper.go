package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type KemonoScraper struct{}

func (scraper *KemonoScraper) Scrape(url string) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	channel := make(chan DownloadItem)
	parentCtx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	go scraper.consumeImageLinks(parentCtx, channel, &waitGroup)
	go scraper.produceImageLinks(url, parentCtx, channel, &waitGroup)
	waitGroup.Wait()
}

func (scraper *KemonoScraper) consumeImageLinks(parentCtx context.Context, c chan DownloadItem, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		item, ok := <-c
		if !ok {
			log.Print("Failed to download " + item.url)
			return
		}
		scraper.downloadImage(parentCtx, item)
		time.Sleep(2500 * time.Millisecond) // Kemono is very bonkers on error 429
	}
}

func (scraper *KemonoScraper) produceImageLinks(url string, parentCtx context.Context, c chan DownloadItem, wg *sync.WaitGroup) {
	defer wg.Done()

	ctx, cancel := chromedp.NewContext(parentCtx)
	defer cancel()

	if err := chromedp.Run(ctx); err != nil {
		panic(err)
	}

	var fullHtml string
	var nodes []*cdp.Node

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Nodes(`div.post__thumbnail > figure > a`, &nodes, chromedp.ByQueryAll),
		chromedp.InnerHTML(`html`, &fullHtml),
	); err != nil {
		log.Fatal(err)
	}

	log.Print(fullHtml)
	for _, node := range nodes {
		item := DownloadItem{
			fileName: node.AttributeValue("download"),
			url:      node.AttributeValue("href"),
		}
		if item.isValid() {
			c <- item
		} else {
			log.Printf("Item incomplete: %v, %v", item.fileName, item.url)
		}
	}

	close(c)
}

func (scraper *KemonoScraper) downloadImage(ctx context.Context, downloadItem DownloadItem) error {
	childCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	if err := chromedp.Run(childCtx); err != nil {
		panic(err)
	}

	done := make(chan bool)
	var requestID network.RequestID
	chromedp.ListenTarget(childCtx, func(v interface{}) {
		switch ev := v.(type) {
		case *network.EventRequestWillBeSent:
			log.Printf("EventRequestWillBeSent: %v: %v", ev.RequestID, ev.Request.URL)
			if ev.Request.URL == downloadItem.url {
				requestID = ev.RequestID
			}
		case *network.EventLoadingFinished:
			log.Printf("EventLoadingFinished: %v", ev.RequestID)
			if ev.RequestID == requestID {
				close(done)
			}
		}
	})

	if err := chromedp.Run(childCtx, chromedp.Navigate(downloadItem.url)); err != nil {
		return err
	}

	// This will block until the chromedp listener closes the channel
	<-done

	var buf []byte
	if err := chromedp.Run(childCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		buf, err = network.GetResponseBody(requestID).Do(ctx)
		return err
	})); err != nil {
		return err
	}

	// Saves the image
	if err := os.WriteFile(downloadItem.fileName, buf, 0644); err != nil {
		return err
	}

	return nil
}
