package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
)

func biliAudioGetter(c *gin.Context) {
	BV := c.Query("BV")
	oFormat := c.DefaultQuery("format", "mp3")
	bitrate := c.DefaultQuery("bitrate", "128k")
	// check BV, if malformed raise exception
	url := fmt.Sprintf("https://www.bilibili.com/video/%s", BV)

	var matches []string

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			if ev.Type == "Fetch" {
				url := ev.Request.URL
				if strings.Contains(url, "upos-hz-mirrorakam.akamaized.net") {
					matches = append(matches, url)
				}
			}
		}
	})
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"msg": "Unable to load load web page"})
		return
	} else if len(matches) < 2 {
		c.JSON(http.StatusNotFound, gin.H{"msg": "Audio URL not found"})
		return
	}

	fmt.Println(matches[1])

	res, err := http.Get(matches[1])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to fetch audio"})
		return
	}
	defer res.Body.Close()

	audio, err := io.ReadAll(res.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to read audio data"})
		return
	}
	iBuffer := bytes.NewBuffer(audio)
	oBuffer := bytes.NewBuffer(nil)

	err = ffmpeg_go.
		Input("pipe:", ffmpeg_go.KwArgs{"f": "mp4"}).
		Output("pipe:", ffmpeg_go.KwArgs{"f": oFormat, "acodec": "libmp3lame", "b:a": bitrate}).
		WithInput(iBuffer).
		WithOutput(oBuffer).
		Run()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to convert audio"})
		return
	}

	c.Header("Content-Type", "audio/mpeg")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=audio.%s", oFormat))

	_, err = c.Writer.Write(oBuffer.Bytes())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to write response"})
		return
	}
}

func main() {
	r := gin.Default()
	r.SetTrustedProxies(nil)

	r.GET("/", biliAudioGetter)

	r.Run(":8081")
}
