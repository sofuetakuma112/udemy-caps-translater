package translate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type ResGoogleTranslate struct {
	Code int    `json:"code"`
	Text string `json:"text"`
}

func Translate(text string, apiUrlFormat string) (ResGoogleTranslate, error) {
	var res ResGoogleTranslate

	url := fmt.Sprintf(apiUrlFormat, url.QueryEscape(text))

	for i := 0; i < 100; i++ {
		resp, err := http.Get(url)
		if err != nil {
			return ResGoogleTranslate{}, errors.New("HTTPリクエストの送信に失敗")
		}

		defer resp.Body.Close()

		byteArray, _ := ioutil.ReadAll(resp.Body)

		json.Unmarshal(byteArray, &res)

		if res.Code == 200 {
			return res, nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return ResGoogleTranslate{}, errors.New("翻訳に失敗")
}
