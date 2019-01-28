package main

import (
	"time"
	"os"
	"log"
	"io/ioutil"
	"./lib/ljapi"
	"./lib/imgurapi"
	"./lib/sender"
	"./lib/logger"
	"./lib/wpapi"
	"./fixer"
	"fmt"
	"net/url"
	"encoding/json"
	"path"
)

var imgur imgurapi.Client = imgurapi.Client {
	Locked: false,
	ResetTime: 0,
	ClientID: "",
	ClientSecret: "",
	MashapeKey: "",
}

var mail sender.SMTPSettings = sender.SMTPSettings {
	SmtpUsername: "",
	SmtpPassword: "",
	SmtpServer: "",
}

type task struct {
	LJ ljapi.Client	`json:"lj_client"`
	WP wpapi.Client	`json:"wp_client"`
	Mode	string		`json:"mode"`
	Email string			`json:"email"`
	Posts []string		`json:"posts"`
	Rules []string		`json:"rules"`
	Filename string
}

type image struct {
	URL, Domain string
	Size int
}

func loadConfig(filename string) bool {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Print("Failed to read config file.")
		log.Print(err)
		return false
	}
	err = json.Unmarshal(content, &imgur)
	if err != nil {
		log.Print("Failed to parse config file.")
		log.Print(err)
		return false
	}
	err = json.Unmarshal(content, &mail)
	if err != nil {
		log.Print("Failed to parse config file.")
		log.Print(err)
		return false
	}
	if (imgur.ClientID == "") || (imgur.ClientSecret == "") || (imgur.MashapeKey == "") {
		log.Print("Invalid config file.")
		return false
	}
	if (mail.SmtpUsername == "") || (mail.SmtpPassword == "") || (mail.SmtpServer == "") {
		log.Print("Invalid config file.")
		return false
	}
	log.Print("Config file successfuly loaded.")
	return true
}

func loadTask(filename string) task {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Printf("Failed to load task %s", filename)
		log.Print(err)
		os.Remove(filename)
		return task{}
	}
	var result task
	err = json.Unmarshal(content, &result)
	if err != nil {
		log.Printf("Failed to parse task %s", filename)
		log.Print(err)
		os.Remove(filename)
		return task{}
	}
	if (result.Mode != "lj") && (result.Mode != "wp") {
		log.Printf("Invalid task %s", filename)
		log.Print(err)
		os.Remove(filename)
		return task{}
	}
	result.Filename = filename
	log.Printf("Loaded task %s", filename)
	return result
}

//Ужасный блять костыль, но я ебал разбираться с интерфейсами
//Закройте глаза

func backupPostLJ(link string, post ljapi.Post) error {
	_, filename := path.Split(link)

	f, err := os.Create("report/" + filename + ".txt")
	defer f.Close()
	if err != nil {
		return err
	}
	fmt.Fprint(f, post.Header, "\n\n\n", post.Content)
	f.Close()

	post.Content = url.PathEscape(post.Content)
	post.Header = url.PathEscape(post.Header)

	buf, err := json.Marshal(post)
	if err != nil {
		return err
	}

	f, err = os.Create("report/" + filename + ".json")
	if err != nil {
		return err
	}
	fmt.Fprint(f, string(buf))
	return nil
}

func backupPostWP(link string, post wpapi.Post) error {
	_, filename := path.Split(link)

	f, err := os.Create("report/" + filename + ".txt")
	defer f.Close()
	if err != nil {
		return err
	}
	fmt.Fprint(f, post.Header, "\n\n\n", post.Content)
	f.Close()

	post.Content = url.PathEscape(post.Content)
	post.Header = url.PathEscape(post.Header)

	buf, err := json.Marshal(post)
	if err != nil {
		return err
	}

	f, err = os.Create("report/" + filename + ".json")
	if err != nil {
		return err
	}
	fmt.Fprint(f, string(buf))
	return nil
}

//Откройте глаза

func initReportDir() {
	os.RemoveAll("report/")
	os.Mkdir("report/", 0777)
}

var mainReport logger.Reporter

func executeTaskLJ(subject task) {
	for _, link := range subject.Posts {
		mainReport.Log(fmt.Sprintf("Started reuploading for post %s\n", link))
		post, err := subject.LJ.GetPost(link)
		if err != nil {
			mainReport.Log(fmt.Sprintf("Failed to get post %s\n", link))
			log.Print(err)
			continue
		}
		err = backupPostLJ(link, post)
		if err != nil {
			mainReport.Log(fmt.Sprintf("Failed to backup post %s\n", link))
			log.Print(err)
			continue
		}
		post.Content = fixer.ProcessPost(post.Content, subject.Rules, imgur, mainReport)
		if err != nil {
			mainReport.Log(fmt.Sprintf("Failed to process post %s\n", link))
			log.Print(err)
			continue
		}
		err = subject.LJ.EditPost(post)
		if err == nil {
			mainReport.Log(fmt.Sprintf("%s : done\n", link))
		} else {
			mainReport.Log(fmt.Sprintf("%s : error : %s\n", link, err))
		}
	}
}

func executeTaskWP(subject task) {
	for _, id := range subject.Posts {
		mainReport.Log(fmt.Sprintf("Started reuploading for post %s\n", id))
		post, err := subject.WP.GetPost(id)
		if err != nil {
			mainReport.Log(fmt.Sprintf("Failed to get post %s\n", id))
			log.Print(err)
			continue
		}
		err = backupPostWP(id, post)
		if err != nil {
			mainReport.Log(fmt.Sprintf("Failed to backup post %s\n", id))
			log.Print(err)
			continue
		}
		post.Content = fixer.ProcessPost(post.Content, subject.Rules, imgur, mainReport)
		if err != nil {
			mainReport.Log(fmt.Sprintf("Failed to process post %s\n", id))
			log.Print(err)
			continue
		}
		ok, err := subject.WP.EditPost(post)
		if (err == nil) || (!ok) {
			mainReport.Log(fmt.Sprintf("%s : done\n", id))
		} else {
			mainReport.Log(fmt.Sprintf("%s : error : %s\n", id, err))
		}
	}
}

func executeTask(subject task) {
	initReportDir()
	mainReport.Init("report/report.txt")
	defer mainReport.Finish()
	mainReport.Log(fmt.Sprintf("Started executing task for %s\n", subject.LJ.User))

	if subject.Mode == "lj" {
		executeTaskLJ(subject)
	}

	if subject.Mode == "wp" {
		executeTaskWP(subject)
	}

	mainReport.Finish()
	if !imgur.Locked {
		err := mail.SendReport(subject.Email, subject.LJ.User)
		if err != nil {
			log.Print(err)
		} else {
			log.Printf("Successfuly sent email to %s", subject.Email)
		}
		mainReport.Finish()
		os.Remove(subject.Filename)
	}
}

func main() {
	log.Print("UNIR Online Front-End")
	if !loadConfig("conf.json") {
		return
	}
	initReportDir()
	var check_id int = -1
	for true {
		if imgur.Locked {
			log.Printf("Imgur is locked, waiting %d seconds", imgur.ResetTime)
			time.Sleep(time.Duration(int64(imgur.ResetTime + 1) * 1000000000))
		}
		imgur.Locked = false
		time.Sleep(5 * time.Second)
		check_id++
		tasks, err := ioutil.ReadDir("tasks/")
		if err != nil {
			log.Printf("Check #%d: Failed to check tasks", check_id)
			log.Print(err)
			continue
		}
		if len(tasks) == 0 {
			log.Printf("Check #%d: No tasks were found", check_id)
			continue
		}
		executeTask(loadTask("tasks/" + tasks[0].Name()))
		log.Printf("Imgur reset time: %d", imgur.ResetTime)
	}
}
