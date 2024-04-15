package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	ffmpeg_go "github.com/u2takey/ffmpeg-go"
)

func getPageMetaUrl(BV string, page int) (string, error) {
	/* initialize global vars */
	var JSON map[string]interface{}
	var cid string

	/* get page video meta from api.bilibili */
	playlistUrl := fmt.Sprintf("https://api.bilibili.com/x/player/pagelist?bvid=%s", BV)
	bvPage, err := http.Get(playlistUrl)
	if err != nil {
		return "", err
	}
	defer bvPage.Body.Close()
	bvByte, err := io.ReadAll(bvPage.Body)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(bvByte, &JSON); err != nil {
		return "", err
	}
	if pages, ok := JSON["data"].([]interface{}); ok {
		if page > len(pages) {
			return "", fmt.Errorf("'p' you are looking for does not exist (exceeded upper bound)")
		}
		if pageMeta, ok := pages[page-1].(map[string]interface{}); ok {
			if pageCid, ok := pageMeta["cid"].(float64); ok {
				cid = strconv.FormatFloat(pageCid, 'f', 0, 64)
			}
		}
	}
	if cid == "" {
		return "", fmt.Errorf("'cid' not found")
	}

	/* return api url */
	pageMetaUrl := fmt.Sprintf(
		"https://api.bilibili.com/x/player/playurl?fnval=80&cid=%s&bvid=%s", cid, BV)
	return pageMetaUrl, nil
}

func audioGetter(w http.ResponseWriter, r *http.Request) {
	/* initialize global vars */
	var JSON map[string]interface{}
	var maxBandwidth float64
	var aUrl string
	var oKwargs ffmpeg_go.KwArgs

	/* query url params */
	BV := r.URL.Query().Get("bv")
	page, err := strconv.Atoi(r.URL.Query().Get("p"))
	if err != nil {
		page = 1
	}
	bitrate := r.URL.Query().Get("bitrate")
	oFormat := r.URL.Query().Get("format")

	/* check params bitrate */
	if bitrate != "" {
		if num, err := strconv.Atoi(bitrate); err != nil {
			http.Error(w, `{"msg": "Parameter 'bitrate' malformed"}`, http.StatusBadRequest)
			return
		} else if num > 320 {
			http.Error(w, `{"msg": "Reject transcoding with bitrate over 320k"}`, http.StatusForbidden)
			return
		}
	}
	if matched, err := regexp.MatchString("^BV\\w+$", BV); !matched || len(BV) != 12 || err != nil {
		http.Error(w, `{"msg": "Parameter 'bv' malformed"}`, http.StatusBadRequest)
		return
	}

	/* get meta page url */
	pageMetaUrl, err := getPageMetaUrl(BV, page)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"msg": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	/* get page video meta from api.bilibili */
	metaPage, err := http.Get(pageMetaUrl)
	if err != nil {
		http.Error(w, `{"msg": "Failed to load api page"}`, http.StatusInternalServerError)
		return
	}
	defer metaPage.Body.Close()
	apiByte, err := io.ReadAll(metaPage.Body)
	if err != nil {
		http.Error(w, `{"msg": "Failed to read page-meta page body"}`, http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal(apiByte, &JSON); err != nil {
		http.Error(w, `{"msg": "Failed to parse page-meta JSON data"}`, http.StatusInternalServerError)
		return
	}
	if data, ok := JSON["data"].(map[string]interface{}); ok {
		if dash, ok := data["dash"].(map[string]interface{}); ok {
			// Login & VIP required
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
		http.Error(w, `{"msg": "Resource Url not found"}`, http.StatusInternalServerError)
		return
	}

	/* fetch audio from upos-hz-mirrorakam.akamaized.net */
	res, err := http.Get(aUrl)
	if err != nil {
		http.Error(w, `{"msg": "Failed to fetch from resource"}`, http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()
	audio, err := io.ReadAll(res.Body)
	if err != nil {
		http.Error(w, `{"msg": "Failed to read audio data"}`, http.StatusInternalServerError)
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
		http.Error(w, `{"msg": "Illegal or unsupported setting"}`, http.StatusBadRequest)
		return
	}
	err = ffmpeg_go.
		Input("pipe:", ffmpeg_go.KwArgs{"f": "mp4"}).
		Output("pipe:", oKwargs).
		WithInput(iBuffer).
		WithOutput(oBuffer).
		Run()
	if err != nil {
		http.Error(w, `{"msg": "Failed to encode audio"}`, http.StatusInternalServerError)
		return
	}

	/* send converted to client for download */
	if oFormat == "" {
		oFormat = "aac"
	}
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=audio.%s", oFormat))
	_, err = w.Write(oBuffer.Bytes())
	if err != nil {
		http.Error(w, `{"msg": "Failed to write response"}`, http.StatusInternalServerError)
		return
	}
}

func apiGetter(w http.ResponseWriter, r *http.Request) {
	/* query url params */
	BV := r.URL.Query().Get("bv")
	page, err := strconv.Atoi(r.URL.Query().Get("p"))
	if err != nil {
		page = 1
	}

	/* check if BV malformed */
	if matched, err := regexp.MatchString("", BV); !matched || len(BV) != 12 || err != nil {
		http.Error(w, `{"msg": "Parameter 'bv' malformed"}`, http.StatusBadRequest)
		return
	}

	/* get meta page url */
	pageMetaUrl, err := getPageMetaUrl(BV, page)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"msg": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	/* send api url to client */
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte(fmt.Sprintf(`{"url": "%s"}`, pageMetaUrl)))
	if err != nil {
		http.Error(w, `{"msg": "Failed to write response"}`, http.StatusInternalServerError)
		return
	}
}

func main() {
	http.HandleFunc("/", audioGetter)
	http.HandleFunc("/api/", apiGetter)
	fmt.Println("BiliAudioGetter is currently running at :8081")
	http.ListenAndServe(":8081", nil)
}
