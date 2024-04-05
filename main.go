package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
)

func biliAudioGetter(c *gin.Context) {
	BV := c.Param("BV")
	// check BV, if malformed raise exception
	url := fmt.Sprintf("https://www.bilibili.com/video/%s", BV)

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			if ev.Type == "Fetch" {
				url := ev.Request.URL
				if strings.Contains(url, "upos-hz-mirrorakam.akamaized.net") {
					fmt.Println(url)
				}
			}
		}
	})

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"msg": "Unable to load URL"})
		return
	}
	// ...
}

func main() {
	r := gin.Default()

	r.GET("/:BV", biliAudioGetter)

	r.Run(":8081")
}
