package fixer

import (
	"net/url"
	"net/http"
	"errors"
	"strconv"
	"strings"
	"fmt"
	"../lib/imgurapi"
	"../lib/logger"
)

type image struct {
	URL, Domain string
	Size int
}

type Fix struct {
	Name	string	`json:"name"`
	From	string	`json:"from"`
	To		string	`json:"to"`
}

func (i *image) GetImageInfo() error {
	u, err := url.Parse(i.URL)
	if (err != nil) {
		return err
	}
	i.Domain = u.Host
	if (err != nil) {
		return err
	}
	head, err := http.Head(i.URL)
	if (err != nil) {
		return err
	}
	if (head.StatusCode != http.StatusOK) {
		return errors.New("Unknown error : " + head.Status)
	}
	i.Size, err = strconv.Atoi(head.Header.Get("Content-Length"));
	if (err != nil) {
		return err
	}
	return nil
}

func (i *image) CheckImage(rules []string) bool {
  if i.Domain == "i.imgur.com" {
    return false
  }
	var rules_map map[string][]string = make(map[string][]string)
	for _, rule := range rules {
		entry := strings.Split(rule, " ")
		rules_map[entry[0]] = entry[1:]
	}
	if (rules_map["MORETHAN"] != nil) {
		if size, err := strconv.Atoi(rules_map["MORETHAN"][0]); (err != nil || i.Size <= size) {
			return false
		}
	}
	if (rules_map["LESSTHAN"] != nil) {
		if size, err := strconv.Atoi(rules_map["LESSTHAN"][0]); (err != nil || i.Size <= size) {
			return false
		}
	}
	if (rules_map["EXCLUDE"] != nil) {
		for _, domain := range rules_map["EXCLUDE"] {
			if (strings.Contains(i.Domain, domain)) {
				return false
			}
		}
	}
	if (rules_map["INCLUDE"] != nil) {
		var found bool = false
		for _, domain := range rules_map["INCLUDE"] {
			if (domain == "*" || strings.Contains(i.Domain, domain)) {
				found = true
				break
			}
		}
		if (!found) {
			return false
		}
	}
	return true
}

func ProcessPost(postText string, rules []string, imgur imgurapi.Client,
	reporter logger.Reporter, fixes []Fix) string {

  var tokens = [...]string {"<", "img", "src", "=", "\"", "\""}
  var tokenNum, urlBegin, urlEnd int = 0, 0, 0
  var resultText string = postText

	for _, f := range fixes {
		if strings.Contains(resultText, f.From) {
			resultText = strings.Replace(resultText, f.From, f.To, -1)
			reporter.Log("Performed a(n) " + f.Name)
		}
	}

  for i := 0; i < len(postText); i++ {
    if postText[i] == '>' {
      tokenNum = 0
      urlBegin = 0
      urlEnd = 0
      continue
    }
    if len(postText)-i < len(tokens[tokenNum]) {
      break
    }
    var check string = postText[i:i+len(tokens[tokenNum])]
    if check == tokens[tokenNum] {
      if tokenNum == 4 {
        urlBegin = i + 1
      }
      if tokenNum == 5 {
        urlEnd = i
        var oldUrl string = postText[urlBegin:urlEnd]
        var img image = image{URL: oldUrl}
        img.GetImageInfo()

        tokenNum = 0
        urlBegin = 0
        urlEnd = 0

        if img.CheckImage(rules) {
          newUrl, err := imgur.UploadImage(oldUrl)
          if err != nil {
            reporter.Log(fmt.Sprintf("Error uploading photo %s", oldUrl))
            continue
          }
          resultText = strings.Replace(resultText, oldUrl, newUrl, -1)
          reporter.Log(fmt.Sprintf("%s -> %s", oldUrl, newUrl))
        } else {
					reporter.Log(fmt.Sprintf("Skipped %s due to rules", oldUrl))
				}
      }
      tokenNum++
    }
  }
	return resultText
}
