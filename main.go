package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
)

func estimateBitrate(audio []byte, duration float64) int {
	size := len(audio)
	bytesPerSecond := float64(size) / duration
	bitrateKbps := int(bytesPerSecond * 8 / 1024)
	minBitrateKbps, maxBitrateKbps := 48, 640
	if bitrateKbps < minBitrateKbps {
		bitrateKbps = minBitrateKbps
	} else if bitrateKbps > maxBitrateKbps {
		bitrateKbps = maxBitrateKbps
	}
	return bitrateKbps
}

func biliAudioGetter(c *gin.Context) {
	/* initialize global vars */
	var JSON map[string]interface{}
	var duration float64
	var cid string
	var maxBandwidth float64
	var aUrl string

	/* query url params */
	BV := c.Query("bv")
	page, err := strconv.Atoi(c.DefaultQuery("p", "1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameter 'p'"})
		return
	}
	bitrate := c.Query("bitrate")
	oFormat := c.DefaultQuery("format", "mp3")
	playlistUrl := fmt.Sprintf("https://api.bilibili.com/x/player/pagelist?bvid=%s", BV) // check BV, if malformed raise exception

	/* get cid from video page */
	bvPage, err := http.Get(playlistUrl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to load video page"})
		return
	}
	defer bvPage.Body.Close()
	bvByte, err := io.ReadAll(bvPage.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to read playlist-meta page body"})
		return
	}
	if err := json.Unmarshal(bvByte, &JSON); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to parse playlist-meta JSON data"})
		return
	}
	if pages, ok := JSON["data"].([]interface{}); ok {
		if page > len(pages) {
			c.JSON(http.StatusNotFound, gin.H{"msg": "The 'p' you are looking for does not exist (exceeded upper bound)"})
			return
		}
		if pageMeta, ok := pages[page-1].(map[string]interface{}); ok {
			if pageDuration, ok := pageMeta["duration"].(float64); ok {
				duration = pageDuration
			}
			if pageCid, ok := pageMeta["cid"].(float64); ok {
				cid = strconv.FormatFloat(pageCid, 'f', 0, 64)
			}
		}
	}
	if cid == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "'cid' not found"})
		return
	}

	/* get page video meta from api.bilibili */
	pageMetaUrl := fmt.Sprintf(
		"https://api.bilibili.com/x/player/playurl?fnval=80&cid=%s&bvid=%s", cid, BV)
	metaPage, err := http.Get(pageMetaUrl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to load api page"})
		return
	}
	defer metaPage.Body.Close()
	apiByte, err := io.ReadAll(metaPage.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to read page-meta page body"})
		return
	}
	if err := json.Unmarshal(apiByte, &JSON); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to parse page-meta JSON data"})
		return
	}
	if data, ok := JSON["data"].(map[string]interface{}); ok {
		if dash, ok := data["dash"].(map[string]interface{}); ok {
			// Login required
			// if flac, ok := dash["flac"].(map[string]interface{}); ok {
			// 	if audioMeta, ok := flac["audio"].(map[string]interface{}); ok {
			// 		if baseUrl, ok := audioMeta["baseUrl"].(string); ok {
			// 			aUrl = baseUrl
			// 		}
			// 	}
			// } else
			if audioList, ok := dash["audio"].([]interface{}); ok {
				for _, v := range audioList {
					if audioMeta, ok := v.(map[string]interface{}); ok {
						if bandwidth, ok := audioMeta["bandwidth"].(float64); ok {
							if bandwidth > maxBandwidth {
								maxBandwidth = bandwidth
								if baseUrl, ok := audioMeta["baseUrl"].(string); ok {
									aUrl = baseUrl
								}
							}
						}
					}
				}
			}
		}
	}
	if aUrl == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Resource Url not found"})
		return
	}

	/* fetch audio from upos-hz-mirrorakam.akamaized.net */
	res, err := http.Get(aUrl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to fetch from resource"})
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

	/* estimate bitrate */
	if bitrate == "" {
		bitrate = fmt.Sprint(estimateBitrate(audio, float64(duration)))
	}

	/* format conversion with ffmpeg */
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

	/* send converted to client for download */
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
