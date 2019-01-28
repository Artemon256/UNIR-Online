package main

import (
	"fmt"
	"log"
	"net/http"
	"io/ioutil"
	"io"
	"os"
	"strings"
	"strconv"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"time"
	"math/rand"
	"./lib/ljapi"
	"./lib/wpapi"
	"syscall"
	"errors"
)

type settings struct {
	CertFile	string	`json:"site_cert"`
	KeyFile		string	`json:"site_key"`
	UseTLS 		bool		`json:"site_tls"`
	GroupID		int			`json:"gid"`
	UserID 		int			`json:"uid"`
}

var conf settings = settings {
	CertFile: "",
	KeyFile: "",
	UseTLS: false,
	GroupID: os.Getgid(),
	UserID: os.Getuid(),
}

func loadConfig(filename string) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Print("Failed to read config file. Using default settings. ")
		log.Print(err)
		return
	}
	err = json.Unmarshal(content, &conf)
	if err != nil {
		log.Print("Failed to parse config file. Using default settings. ")
		log.Print(err)
		return
	}
	log.Print("Config file successfuly loaded.")
}

func loadPage(response http.ResponseWriter, filename string) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	f, err := os.Open(filename)
	defer f.Close()
	if err != nil {
		log.Print(err)
		f, err = os.Open("pages/404.html")
		if err != nil {
			response.WriteHeader(http.StatusNotFound)
			return
		}
	}
	io.Copy(response, f)
}

func loadStyleSheet(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "text/css; charset=utf-8")
        f, err := os.Open("pages/style.css")
		defer f.Close()
        if err != nil {
				log.Print(err)
                loadPage(response, "pages/500.html")
                return
        }
        io.Copy(response, f)
}

func newLJ(user, password string) (error, ljapi.Client) {
	buf := md5.Sum([]byte(password))
	passhash := hex.EncodeToString(buf[:])
	lj := ljapi.Client{User: user, PassHash: passhash}
	ok, err := lj.TryLogIn()
	if err != nil {
		return err, lj
	}
	if !ok {
		return errors.New("newLJ(): wrong password"), lj
	}
	return nil, lj
}

func slashAddr(addr string) string {
	if len(addr) == 0 {
		return ""
	}
	if addr[len(addr)-1] == '/' {
		return addr
	}
	addr = addr + "/"
	return addr
}

func newWP(user, pass, address string) (err error, wp wpapi.Client) {
	wp = wpapi.Client{
		APIURL: slashAddr(address) + "xmlrpc.php",
		Username: user,
		Password: pass,
	}
	defer func() {
		if recover() != nil {
			err = errors.New("newWP(): unknown error. Wrong login credentials?")
			wp = wpapi.Client{}
		}
	}()
	blog, err := wp.GetUsersBlog()
	if err != nil {
		return err, wp
	}
	if blog == "" {
		return errors.New("newWP(): unknown error. Wrong login credentials?"), wp
	}
	return nil, wp
}

func getNonce() string {
	const ALPHABET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var res string = ""
	for i := 0; i < 8; i++ {
		res = res + string(ALPHABET[rand.Intn(len(ALPHABET))])
	}
	return res
}

type reuploadQuery struct {
	Mode	string					`json:"mode"`
	LJ		ljapi.Client		`json:"lj_client"`
	WP		wpapi.Client		`json:"wp_client"`
	Email string					`json:"email"`
	Posts []string				`json:"posts"`
	Rules []string				`json:"rules"`
}

func registerReuploadQueryLJ(response http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		log.Print(err)
		loadPage(response, "pages/500.html")
		return
	}

	user := request.Form.Get("user")
	password := request.Form.Get("password")
	email := request.Form.Get("email")

	links := strings.Split(request.Form.Get("links"), "\r\n")
	rules := strings.Split(request.Form.Get("rules"), "\r\n")

	if ((user == "") || (password == "") || (email == "") || (len(links) == 0) || (len(rules) == 0)) {
		loadPage(response, "pages/400.html")
		return
	}

	err, lj := newLJ(user, password)

	if err != nil {
		log.Print(err)
		if err.Error() == "newLJ(): wrong password" {
			loadPage(response, "pages/403.html")
		} else {
			loadPage(response, "pages/500.html")
		}
		return
	}

	query := reuploadQuery{
		Mode: "lj",
		LJ: lj,
		Email: email,
		Posts: links,
		Rules: rules,
	}

	js_bytes, err := json.Marshal(query)
	if err != nil {
		log.Print(err)
		loadPage(response, "pages/500.html")
		return
	}

	var taskfile string = strconv.Itoa(int(time.Now().Unix())) + "-" + getNonce()
	taskfile = "tasks/"+taskfile

	f, err := os.Create(taskfile)
	fmt.Fprint(f, string(js_bytes))
	f.Close()

	os.Chown(taskfile, conf.UserID, conf.GroupID)
	os.Chmod(taskfile, 0660)

	loadPage(response, "pages/reupload.html")

	log.Printf("Registered an LJ reupload query. Task file: %s", taskfile)
}

func registerReuploadQueryWP(response http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		log.Print(err)
		loadPage(response, "pages/500.html")
		return
	}

	user := request.Form.Get("user")
	password := request.Form.Get("password")
	address := request.Form.Get("site")
	email := request.Form.Get("email")

	links := strings.Split(request.Form.Get("links"), "\r\n")
	rules := strings.Split(request.Form.Get("rules"), "\r\n")

	if ((user == "") || (password == "") || (email == "") || (address == "") || (len(links) == 0) || (len(rules) == 0)) {
		loadPage(response, "pages/400.html")
		return
	}

	err, wp := newWP(user, password, address)

	if (err != nil) {
		log.Print(err)
		loadPage(response, "pages/403.html")
		return
	}

	query := reuploadQuery{
		Mode: "wp",
		WP: wp,
		Email: email,
		Posts: links,
		Rules: rules,
	}

	js_bytes, err := json.Marshal(query)
	if err != nil {
		log.Print(err)
		loadPage(response, "pages/500.html")
		return
	}

	var taskfile string = strconv.Itoa(int(time.Now().Unix())) + "-" + getNonce()
	taskfile = "tasks/"+taskfile

	f, err := os.Create(taskfile)
	fmt.Fprint(f, string(js_bytes))
	f.Close()

	os.Chown(taskfile, conf.UserID, conf.GroupID)
	os.Chmod(taskfile, 0660)

	loadPage(response, "pages/reupload.html")

	log.Printf("Registered a WP reupload query. Task file: %s", taskfile)
}

func loadFavicon(response http.ResponseWriter) {
	response.Header().Set("Content-Type", "image/x-icon")
	f, err := os.Open("pages/favicon.ico")
	defer f.Close()
	if err != nil {
		response.WriteHeader(http.StatusNotFound)
		return
	}
	io.Copy(response, f)
}

func handler(response http.ResponseWriter, request *http.Request) {
	var url string = request.URL.Path
	log.Printf("Request to %s from %s", url, request.RemoteAddr)
	switch url {
		case "/": loadPage(response, "pages/welcome.html")
		case "/style.css": loadStyleSheet(response)
		case "/favicon.ico": loadFavicon(response)
		case "/reupload_lj": registerReuploadQueryLJ(response, request)
		case "/reupload_wp": registerReuploadQueryWP(response, request)
		default: loadPage(response, "pages" + url)
	}
}

func main() {
	log.Print("UNIR Online Front-End")

	loadConfig("conf.json")

	oldmask := syscall.Umask(0)
	defer syscall.Umask(oldmask)

	rand.Seed(int64(time.Now().Unix()))

	os.RemoveAll("tasks/")
	os.Mkdir("tasks/", 0770)
	os.Chown("tasks/", conf.UserID, conf.GroupID)

	http.HandleFunc("/", handler)
	if conf.UseTLS {
		log.Fatal(http.ListenAndServeTLS(":443", conf.CertFile, conf.KeyFile, nil))
	} else {
		log.Fatal(http.ListenAndServe(":80", nil))
	}
}
