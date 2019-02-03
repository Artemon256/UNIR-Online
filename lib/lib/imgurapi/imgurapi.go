package imgurapi

import (
	"errors"
	"net/http"
	"mime/multipart"
	"bytes"
	"fmt"
	"io/ioutil"
	"encoding/json"
	"strconv"
)

type Client struct {
	Locked 		bool		`json:"imgur_locked"`
	ResetTime int			`json:"imgur_resetTime"`
	ClientID	string	`json:"imgur_clientID"`
	ClientSecret	string	`json:"imgur_clientSecret"`
	MashapeKey 		string	`json:"imgur_mashapeKey"`
}

func (ic *Client) UploadImage(image_url string) (string, error) {
	const UPLOAD_URL = "https://imgur-apiv3.p.mashape.com/3/image"

	var buf bytes.Buffer
	mpart := multipart.NewWriter(&buf)

	field, _ := mpart.CreateFormField("image")
	field.Write([]byte(image_url))
	field, _ = mpart.CreateFormField("type")
	field.Write([]byte("URL"))

	mpart.Close()

	req, _ := http.NewRequest("POST", UPLOAD_URL, &buf)

	req.Header.Add("Authorization", fmt.Sprintf("Client-ID %s", ic.ClientID))
	req.Header.Add("Content-Type", mpart.FormDataContentType())
	req.Header.Add("X-Mashape-Key", ic.MashapeKey)

	http_client := http.Client{}
	rsp, err := http_client.Do(req)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()

	ic.ResetTime, _ = strconv.Atoi(rsp.Header.Get("X-Post-Rate-Limit-Reset"))

	body_bytes, _ := ioutil.ReadAll(rsp.Body)

	var json_root, json_data map[string]*json.RawMessage
	var success bool = false
	var link string = ""

	json.Unmarshal(body_bytes, &json_root)

	if (json_root["success"] == nil) || (json_root["data"] == nil) {
		return "", errors.New(string(body_bytes))
	}

	json.Unmarshal(*json_root["success"], &success)
	json.Unmarshal(*json_root["data"], &json_data)

	if json_data["error"] != nil {
		var json_error map[string]*json.RawMessage
		json.Unmarshal(*json_data["error"], &json_error)
		var errcode int
		if json_error["code"] != nil {
			json.Unmarshal(*json_error["code"], &errcode)
			if errcode == 429 {
				ic.Locked = true
				return "", errors.New("Uploading too fast")
			}
		} else {
			ic.Locked = true
			return "", errors.New("Unknown error : " + string(body_bytes))
		}
	}

	if json_data["link"] == nil {
		return "", errors.New(string(body_bytes))
	}

	json.Unmarshal(*json_data["link"], &link)

	if success {
		return link, nil
	} else {
		return "", errors.New(string(body_bytes))
	}
}
