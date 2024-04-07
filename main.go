package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	ffmpeg_go "github.com/u2takey/ffmpeg-go"
)

func biliAudioGetter(c *gin.Context) {
	/* initialize global vars */
	var JSON map[string]interface{}
	var cid string
	var maxBandwidth float64
	var aUrl string
	var oKwargs ffmpeg_go.KwArgs

	/* query url params */
	BV := c.Query("bv")
	page, err := strconv.Atoi(c.DefaultQuery("p", "1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid query parameter 'p'"})
		return
	}
	bitrate := c.Query("bitrate")
	oFormat := c.Query("format")

	/* check params bitrate */
	if bitrate != "" {
		if num, err := strconv.Atoi(bitrate); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "Parameter 'bitrate' malformed"})
			return
		} else if num > 320 {
			c.JSON(http.StatusForbidden, gin.H{"msg": "Reject transcoding with bitrate over 320k"})
			return
		}
	}
	if matched, err := regexp.MatchString("^BV\\w+$", BV); !matched || err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "Parameter 'bv' malformed"})
		return
	}

	/* get cid from video page */
	playlistUrl := fmt.Sprintf("https://api.bilibili.com/x/player/pagelist?bvid=%s", BV)
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
			/* Login required */
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

	/* format conversion with ffmpeg */
	if bitrate == "" && oFormat == "" {
		oKwargs = ffmpeg_go.KwArgs{"f": "adts", "acodec": "copy"}
	} else if bitrate != "" && oFormat == "" {
		oKwargs = ffmpeg_go.KwArgs{"f": "adts", "acodec": "copy", "b:a": bitrate}
	} else if bitrate == "" && oFormat == "mp3" {
		oKwargs = ffmpeg_go.KwArgs{"f": "mp3", "acodec": "libmp3lame"}
	} else if bitrate != "" && oFormat == "mp3" {
		oKwargs = ffmpeg_go.KwArgs{"f": "mp3", "acodec": "libmp3lame", "b:a": bitrate}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "Illegal or unsupported setting"})
		return
	}
	err = ffmpeg_go.
		Input("pipe:", ffmpeg_go.KwArgs{"f": "mp4"}).
		Output("pipe:", oKwargs).
		WithInput(iBuffer).
		WithOutput(oBuffer).
		Run()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "Failed to encode audio"})
		return
	}

	/* send converted to client for download */
	if oFormat == "" {
		oFormat = "aac"
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
