package ljapi

import (
	"net/http"
	"crypto/md5"
	"encoding/hex"
	"net/url"
	"io/ioutil"
	"html"
	"errors"
	"bytes"
	"fmt"
	"bufio"
	"path"
	"strings"
	"strconv"
	"io"
)

type Client struct {
	User string		`json:"user"`
	PassHash string	`json:"passhash"`
}

type Post struct {
	Header, Content, Year, Month, Day, Hour, Minute, Second, ID string
}

func (lj *Client) getChallenge() (string, error) {
	const URL = "http://www.livejournal.com/interface/flat"
	const TYPE = "application/x-www-form-urlencoded"
	const CONTENT = "mode=getchallenge"
	contentReader := bytes.NewReader([]byte(CONTENT))
	resp, err := http.Post(URL, TYPE, contentReader)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	var flag bool = false
	for scanner.Scan() {
		if flag {
			return scanner.Text(), err
		}
		flag = (scanner.Text() == "challenge")
	}
	return "", err
}

func (lj *Client) getChallengeData() (string, string, error) {
	challenge, err := lj.getChallenge()
	if err != nil {
		return "", "", err
	}
	var challenge_response string = challenge + lj.PassHash
	md5_buf := md5.Sum([]byte(challenge_response))
	challenge_response = hex.EncodeToString(md5_buf[:])
	return challenge, challenge_response, err
}

func (lj *Client) TryLogIn() (bool, error) {
	challenge, challenge_response, err := lj.getChallengeData()
	if err != nil {
		return false, err
	}
	const URL = "http://www.livejournal.com/interface/flat"
	const TYPE = "application/x-www-form-urlencoded"
	const CONTENT = "ver=1&mode=login&user=%s&auth_method=challenge&auth_challenge=%s&auth_response=%s"
	content := fmt.Sprintf(CONTENT, lj.User, challenge, challenge_response)
	contentReader := bytes.NewReader([]byte(content))
	resp, err := http.Post(URL, TYPE, contentReader)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if scanner.Text() == "errmsg" {
			return false, err
		}
	}
	return true, err
}

func readLine(reader io.Reader) ([]byte, bool) {
	buf := make([]byte,1)
	var res []byte
	for true {
		n, _ := reader.Read(buf)
		if n == 0 {
			return res, true
		}
		if string(buf[0])=="\n" {
			break
		}
		res = append(res, buf[0])
	}
	return res, false;
}

func (lj *Client) EditPost(post Post) error {
	challenge, challenge_response, err := lj.getChallengeData()
	if err != nil {
		return err
	}
	const URL = "http://www.livejournal.com/interface/flat"
	const TYPE = "application/x-www-form-urlencoded"
	const CONTENT = "mode=editevent&user=%s&auth_method=challenge&auth_challenge=%s&auth_response=%s&ver=1&itemid=%s&event=%s&subject=%s&year=%s&mon=%s&day=%s&hour=%s&min=%s"

	content := fmt.Sprintf(CONTENT, lj.User, challenge, challenge_response, post.ID, url.QueryEscape(post.Content), url.QueryEscape(post.Header), post.Year, post.Month, post.Day, post.Hour, post.Minute)
	contentReader := bytes.NewReader([]byte(content))

	resp, err := http.Post(URL, TYPE, contentReader)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		buf, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(buf))
		return errors.New(resp.Status)
	}

	return err
}

func (lj *Client) GetPost(post_url string) (Post, error) {
	challenge, challenge_response, err := lj.getChallengeData()
	if err != nil {
		return Post{}, err
	}
	const URL = "http://www.livejournal.com/interface/flat"
	const TYPE = "application/x-www-form-urlencoded"
	const CONTENT = "ver=1&mode=getevents&user=%s&auth_method=challenge&auth_challenge=%s&auth_response=%s&selecttype=one&itemid=%s"
	_, public_id := path.Split(post_url)
	public_id = strings.TrimSuffix(public_id, path.Ext(public_id))
	public_id_i, _ := strconv.Atoi(public_id)
	var post_id string = strconv.Itoa((public_id_i - (public_id_i - (public_id_i / 256)*256)) / 256)
	content := fmt.Sprintf(CONTENT, lj.User, challenge, challenge_response, post_id)
	contentReader := bytes.NewReader([]byte(content))
	resp, err := http.Post(URL, TYPE, contentReader)
	if err != nil {
		return Post{}, err
	}
	defer resp.Body.Close()
	var prev string = ""
	var result Post
	result.ID = post_id
	for true {
		cur, is_last := readLine(resp.Body)
		switch prev {
			case "events_1_event": result.Content = string(cur[:])
			case "events_1_subject": result.Header = string(cur[:])
			case "events_1_eventtime": {
				datetime := strings.Split(string(cur[:]), " ")
				date := strings.Split(datetime[0], "-")
				time := strings.Split(datetime[1], ":")
				result.Year = date[0]
				result.Month = date[1]
				result.Day = date[2]
				result.Hour = time[0]
				result.Minute = time[1]
				result.Second = time[2]
			}
		}
		if (is_last) {
			break
		}
		prev = string(cur[:])
	}
	text, err := url.PathUnescape(result.Content)
	result.Content = html.UnescapeString(text)
	return result, err
}
