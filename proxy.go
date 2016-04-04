package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/elazarl/goproxy"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Redirector struct {
	Proxy     *goproxy.ProxyHttpServer
	Hosts     []string
	Clocking  bool
	ProxyAddr string
	WebAddr   string
	Hours     []string
	OrgDir    string
	Blacklist string
	Blockmode bool
}

func main() {
	r := Redirector{}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&r.ProxyAddr, "proxy", ":8080", "Proxy listen address")
	flag.StringVar(&r.WebAddr, "web", ":8081", "Proxy listen address")
	flag.StringVar(&r.Blacklist, "blacklist", "blacklist", "File that contains a list of blocking urls(regexp)")
	flag.StringVar(&r.OrgDir, "orgdir", "", "Orgmode directory to parse clocking instructions")
	flag.BoolVar(&r.Blockmode, "blockmode", false, "Default blocking")

	var hours string
	flag.StringVar(&hours, "hours", "", "Working hours, example: 8-11,13-17")
	flag.Parse()

	r.Hours = strings.Split(hours, ",")

	r.Init()
}

func (r *Redirector) Init() error {
	if r.OrgDir != "" {
		r.InitOrgReader()
	}

	go r.InitWebServer()

	r.InitProxyServer()

	return nil
}

func (r *Redirector) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if _, err := os.Stat(req.URL.Path[1:]); os.IsNotExist(err) {
		http.ServeFile(w, req, "index.html")
	} else {
		http.ServeFile(w, req, req.URL.Path[1:])
	}
}

func (r *Redirector) InitWebServer() error {
	log.Fatalln(http.ListenAndServe(r.WebAddr, r))
	return nil
}

func (r *Redirector) InitProxyServer() error {
	r.Proxy = goproxy.NewProxyHttpServer()
	r.Proxy.Verbose = true

	r.InitHosts()

	for i := 0; i < len(r.Hosts); i++ {
		r.Proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*" + r.Hosts[i] + "$"))).HandleConnect(goproxy.AlwaysMitm)
	}

	r.Proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if r.IsDenied(ctx.Req.URL.Host) {
			r.Redirect(ctx)
		}
		return goproxy.OkConnect, host
	})

	r.Proxy.OnRequest().DoFunc(
		func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			if r.IsDenied(ctx.Req.URL.Host) {
				r.Redirect(ctx)
			}

			return req, nil
		})

	log.Fatalln(http.ListenAndServe(r.ProxyAddr, r.Proxy))

	return nil
}

func (r *Redirector) InitHosts() error {
	f, err := os.Open(r.Blacklist)
	if err != nil {
		log.Println(err)
		return err
	}
	defer f.Close()
	reader := bufio.NewReaderSize(f, 16*1024)
	line, isPrefix, err := reader.ReadLine()
	for err == nil && !isPrefix {
		s := string(line)

		r.Hosts = append(r.Hosts, s)

		line, isPrefix, err = reader.ReadLine()
	}
	if isPrefix {
		log.Println("buffer size to small")
		return nil
	}
	if err != io.EOF {
		log.Println(err)
		return err
	}

	return nil
}

func (r *Redirector) Redirect(ctx *goproxy.ProxyCtx) {
	parts := strings.Split(r.WebAddr, ":")
	if parts[0] == "" {
		parts[0] = "127.0.0.1"
	}
	ctx.Req.URL.Host = strings.Join(parts, ":")
	ctx.Req.RequestURI = ctx.Req.URL.Host
	ctx.Req.URL.Scheme = "http"
}

func (r *Redirector) IsDenied(host string) bool {

	found := false
	for i := 0; i < len(r.Hosts); i++ {
		found = found || strings.Contains(host, r.Hosts[i])
	}

	deny := r.Blockmode

	if len(r.Hours) > 0 {
		h, _, _ := time.Now().Clock()
		for i := 0; i < len(r.Hours); i++ {
			if r.Hours[i] != "" {
				hours := strings.Split(r.Hours[i], "-")
				start, _ := strconv.Atoi(hours[0])
				stop, _ := strconv.Atoi(hours[1])

				if h >= start && h <= stop {
					deny = true
				}
			}
		}
	}

	deny = deny || r.Clocking

	return found && deny
}

func (r *Redirector) InitOrgReader() error {
	go func() {
		for {
			r.IsClocking()
			log.Printf("Clocking %t", r.Clocking)
			time.Sleep(10 * time.Second)
		}
	}()

	return nil
}

func (r *Redirector) IsClocking() {
	r.Clocking = false
	filepath.Walk(r.OrgDir, r.Visit)
}

func (r *Redirector) Visit(path string, info os.FileInfo, e error) error {
	if info.IsDir() {
		return nil
	}

	isOrg, _ := regexp.MatchString("\\.org", path)
	if !isOrg {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		log.Println(err)
		return err
	}
	defer f.Close()
	reader := bufio.NewReaderSize(f, 16*1024)
	line, isPrefix, err := reader.ReadLine()
	for err == nil && !isPrefix {
		s := string(line)

		isClock, _ := regexp.MatchString(".*CLOCK:.*", s)
		isEnded, _ := regexp.MatchString(".*CLOCK:.*--.*=>.*", s)

		if isClock && !isEnded {
			r.Clocking = true
		}

		line, isPrefix, err = reader.ReadLine()
	}
	if isPrefix {
		log.Println("buffer size to small")
		return nil
	}
	if err != io.EOF {
		log.Println(err)
		return err
	}
	return nil
}
