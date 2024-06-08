package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type EhScraper struct{}

func (scraper *EhScraper) Scrape(url string, headless bool) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	channel := make(chan string)
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", headless))...)
	defer cancel()

	parentCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	go scraper.consumeImageLinks(parentCtx, channel, &waitGroup)
	go scraper.produceImageLinks(url, parentCtx, channel, &waitGroup)
	waitGroup.Wait()
}

func (scraper *EhScraper) consumeImageLinks(parentCtx context.Context, c chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		url, ok := <-c
		if !ok {
			log.Print("Failed to download " + url)
			return
		}
		if err := scraper.downloadImage(parentCtx, url); err != nil {
			log.Fatal(err)
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func (scraper *EhScraper) produceImageLinks(url string, parentCtx context.Context, c chan string, wg *sync.WaitGroup) {
	defer wg.Done()

	ctx, cancel := chromedp.NewContext(parentCtx)
	defer cancel()

	// We open new tabs here boi
	if err := chromedp.Run(ctx); err != nil {
		panic(err)
	}

	var fullHtml string
	var nodes []*cdp.Node

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Nodes(`#gdt > div > div > a`, &nodes, chromedp.ByQueryAll),
		// chromedp.Nodes(`#gdt > div > a`, &nodes, chromedp.ByQueryAll),
		chromedp.InnerHTML(`html`, &fullHtml),
	); err != nil {
		log.Fatal(err)
	}

	for _, node := range nodes {
		imageUrl, err := scraper.parseForImageUrl(ctx, node.AttributeValue("href"))
		if imageUrl != nil {
			c <- *imageUrl
		}
		if err != nil {
			log.Print(err)
		}
	}

	close(c)
}

func (scraper *EhScraper) parseForImageUrl(ctx context.Context, url string) (*string, error) {
	childCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	// Open new tab
	if err := chromedp.Run(childCtx); err != nil {
		panic(err)
	}

	var nodes []*cdp.Node
	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Nodes(`#img`, &nodes, chromedp.ByQuery),
	); err != nil {
		return nil, err
	}

	if len(nodes) > 0 {
		src := nodes[0].AttributeValue("src")
		return &src, nil
	}

	return nil, errors.New("empty length")
}

func (scraper *EhScraper) downloadImage(ctx context.Context, url string) error {
	childCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	// Open new tab
	if err := chromedp.Run(childCtx); err != nil {
		panic(err)
	}

	done := make(chan bool)
	var requestID network.RequestID
	chromedp.ListenTarget(childCtx, func(v interface{}) {
		switch ev := v.(type) {
		case *network.EventRequestWillBeSent:
			log.Printf("EventRequestWillBeSent: %v: %v", ev.RequestID, ev.Request.URL)
			if ev.Request.URL == url {
				requestID = ev.RequestID
			}
		case *network.EventLoadingFinished:
			log.Printf("EventLoadingFinished: %v", ev.RequestID)
			if ev.RequestID == requestID {
				close(done)
			}
		}
	})

	// Navigate to the download url
	if err := chromedp.Run(childCtx, chromedp.Navigate(url)); err != nil {
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
	fileName := scraper.extractSubstring(url)
	if err := os.WriteFile(*fileName, buf, 0644); err != nil {
		return err
	}

	return nil
}

func (scraper *EhScraper) extractSubstring(url string) *string {
	index := strings.LastIndex(url, "/")
	if index == -1 {
		return nil
	}
	substring := url[index+1:]
	return &substring
}
