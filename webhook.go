package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LinkMsg `link message struct`
type LinkMsg struct {
	Title      string `json:"title"`
	MessageURL string `json:"messageUrl"`
	PicURL     string `json:"picUrl"`
}

// ActionCard `action card message struct`
type ActionCard struct {
	Text           string `json:"text"`
	Title          string `json:"title"`
	SingleTitle    string `json:"singleTitle"`
	SingleURL      string `json:"singleUrl"`
	BtnOrientation string `json:"btnOrientation"`
	HideAvatar     string `json:"hideAvatar"` //  robot message avatar
	Buttons        []struct {
		Title     string `json:"title"`
		ActionURL string `json:"actionUrl"`
	} `json:"btns"`
}

// PayLoad payload
type PayLoad struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
	Link struct {
		Title      string `json:"title"`
		Text       string `json:"text"`
		PicURL     string `json:"picURL"`
		MessageURL string `json:"messageUrl"`
	} `json:"link"`
	Markdown struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"markdown"`
	ActionCard ActionCard `json:"actionCard"`
	FeedCard   struct {
		Links []LinkMsg `json:"links"`
	} `json:"feedCard"`
	At struct {
		AtMobiles []string `json:"atMobiles"`
		IsAtAll   bool     `json:"isAtAll"`
	} `json:"at"`
}

// WebHook `web hook base config`
type WebHook struct {
	AccessToken string `json:"accessToken"`
	APIURL      string `json:"apiUrl"`
	Secret      string
}

// Response `DingTalk web hook response struct`
type Response struct {
	ErrorCode    int    `json:"errcode"`
	ErrorMessage string `json:"errmsg"`
}

// NewWebHook `new a WebHook`
func NewWebHook(accessToken string) *WebHook {
	baseAPI := "https://oapi.dingtalk.com/robot/send"
	return &WebHook{AccessToken: accessToken, APIURL: baseAPI}
}

// reset api URL
func (w *WebHook) resetAPIURL() {
	w.APIURL = "https://oapi.dingtalk.com/robot/send"
}

var regStr = `^1([38][0-9]|14[57]|5[^4])\d{8}$`
var regPattern = regexp.MustCompile(regStr)

//  real send request to api
func (w *WebHook) sendPayload(payload *PayLoad) error {
	params := make(map[string]string)
	var apiURL string
	if strings.Contains(w.AccessToken, w.APIURL) {
		apiURL = w.AccessToken
	} else {
		params["access_token"] = w.AccessToken
		apiURL = w.APIURL
	}

	if w.Secret != "" {
		params["timestamp"], params["sign"] = w.getSign()
	}

	// add params
	if len(params) > 0 {
		apiURL = addParamsToURL(params, apiURL)
	}

	//  get config
	bs, _ := json.Marshal(payload)
	//  request api
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(bs))
	if nil != err {
		return errors.New("api request error: " + err.Error())
	}

	//  read response body
	body, _ := ioutil.ReadAll(resp.Body)
	//  api unusual
	if 200 != resp.StatusCode {
		return fmt.Errorf("api response error: %d", resp.StatusCode)
	}

	var result Response
	//  json decode
	err = json.Unmarshal(body, &result)
	if nil != err {
		return errors.New("response struct error: response is not a json anymore, " + err.Error())
	}

	if 0 != result.ErrorCode {
		return fmt.Errorf("api custom error: {code: %d, msg: %s}", result.ErrorCode, result.ErrorMessage)
	}

	return nil
}

// SendTextMsg `send a text message`
func (w *WebHook) SendTextMsg(content string, isAtAll bool, mobiles ...string) error {
	//  send request
	return w.sendPayload(&PayLoad{
		MsgType: "text",
		Text: struct {
			Content string `json:"content"`
		}{
			Content: content,
		},
		At: struct {
			AtMobiles []string `json:"atMobiles"`
			IsAtAll   bool     `json:"isAtAll"`
		}{
			AtMobiles: mobiles,
			IsAtAll:   isAtAll,
		},
	})
}

// SendLinkMsg `send a link message`
func (w *WebHook) SendLinkMsg(title, content, picURL, msgURL string) error {
	return w.sendPayload(&PayLoad{
		MsgType: "link",
		Link: struct {
			Title      string `json:"title"`
			Text       string `json:"text"`
			PicURL     string `json:"picURL"`
			MessageURL string `json:"messageUrl"`
		}{
			Title:      title,
			Text:       content,
			PicURL:     picURL,
			MessageURL: msgURL,
		},
	})
}

// SendMarkdownMsg `send a markdown msg`
func (w *WebHook) SendMarkdownMsg(title, content string, isAtAll bool, mobiles ...string) error {
	firstLine := false
	for _, mobile := range mobiles {
		if regPattern.MatchString(mobile) {
			if false == firstLine {
				content += "#####"
			}
			content += " @" + mobile
			firstLine = true
		}
	}
	//  send request
	return w.sendPayload(&PayLoad{
		MsgType: "markdown",
		Markdown: struct {
			Title string `json:"title"`
			Text  string `json:"text"`
		}{
			Title: title,
			Text:  content,
		},
		At: struct {
			AtMobiles []string `json:"atMobiles"`
			IsAtAll   bool     `json:"isAtAll"`
		}{
			AtMobiles: mobiles,
			IsAtAll:   isAtAll,
		},
	})
}

// SendActionCardMsg `send single action card message`
func (w *WebHook) SendActionCardMsg(title, content string, linkTitles, linkUrls []string, hideAvatar, btnOrientation bool) error {
	//  validation is empty
	if 0 == len(linkTitles) || 0 == len(linkUrls) {
		return errors.New("links or titles is empty！")
	}
	//  validation is equal
	if len(linkUrls) != len(linkTitles) {
		return errors.New("links length and titles length is not equal！")
	}
	//  hide robot avatar
	var strHideAvatar = "0"
	if hideAvatar {
		strHideAvatar = "1"
	}
	//  button sort
	var strBtnOrientation = "0"
	if btnOrientation {
		strBtnOrientation = "1"
	}
	//  button struct
	var buttons []struct {
		Title     string `json:"title"`
		ActionURL string `json:"actionUrl"`
	}
	//  inject to button
	for i := 0; i < len(linkTitles); i++ {
		buttons = append(buttons, struct {
			Title     string `json:"title"`
			ActionURL string `json:"actionUrl"`
		}{
			Title:     linkTitles[i],
			ActionURL: linkUrls[i],
		})
	}
	//  send request
	return w.sendPayload(&PayLoad{
		MsgType: "actionCard",
		ActionCard: ActionCard{
			Title:          title,
			Text:           content,
			HideAvatar:     strHideAvatar,
			BtnOrientation: strBtnOrientation,
			Buttons:        buttons,
		},
	})
}

// SendLinkCardMsg `send link card message`
func (w *WebHook) SendLinkCardMsg(messages []LinkMsg) error {
	return w.sendPayload(&PayLoad{
		MsgType: "feedCard",
		FeedCard: struct {
			Links []LinkMsg `json:"links"`
		}{
			Links: messages,
		},
	})
}

// getSign get sign
func (w *WebHook) getSign() (timestamp, sha string) {
	timestamp = strconv.FormatInt(time.Now().UnixNano() / int64(time.Millisecond), 10)
	message := timestamp + "\n" + w.Secret

	h := hmac.New(sha256.New, []byte(w.Secret))
	h.Write([]byte(message))

	return timestamp, base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// addPramsToUrl
func addParamsToURL(params map[string]string, originURL string) string {
	u, _ := url.Parse(originURL)
	q, _ := url.ParseQuery(u.RawQuery)

	for key, val := range params {
		q.Set(key, val)
	}

	u.RawQuery = q.Encode()
	return u.String()
}