/*

BOOSTER-WEB: Web interface to BOOSTER (https://github.com/evolbioinfo/booster)
Alternative method to compute bootstrap branch supports in large trees.

Copyright (C) 2017 BOOSTER-WEB dev team

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.

*/

package server

import (
	"errors"
	"fmt"
	"html/template"
	goio "io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fredericlemoine/booster-web/config"
	"github.com/fredericlemoine/booster-web/database"
	"github.com/fredericlemoine/booster-web/model"
	"github.com/fredericlemoine/booster-web/processor"
	"github.com/fredericlemoine/booster-web/static"
	"github.com/fredericlemoine/booster-web/templates"
	"github.com/nu7hatch/gouuid"
	"github.com/russross/blackfriday"
)

const (
	DATABASE_TYPE_DEFAULT = "memory"
	HTTP_PORT_DEFAULT     = 8080 // Port 8080
)

var templatePath string

var templatesMap map[string]*template.Template

var db database.BoosterwebDB

var proc processor.Processor

var uuids chan string // channel of uuids generated by a go routine

var logfile *os.File = nil

// The config should contain following keys:
// runners.queuesize: Max number of jobs in the queue (default 10)
// runners.nbrunners: Max number of parallel running jobs (default 1)
// runners.timeout for each running job in Seconds (default 0=unlimited)
// runners.jobthreads : Number of cpus per bootstrap runner
// database.type: mysql or memory (default memory)
// database.user: user to connect to mysql if type is mysql
// database.host: host to connect to mysql if type is mysql
// database.port: port to connect to mysql if type is mysql
// database.pass: pass to connect to mysql if type is mysql
// database.dbname: name of db to connect to mysql if type is mysql
// logging.logfile : path to log file: stdout, stderr or any file name (default stderr)
func InitServer(cfg config.Provider) {
	initLog(cfg)
	log.Print("Starting booster-web")
	initUUIDGenerator()
	initDB(cfg)
	initProcessor(cfg)
	initCleanKill()
	initLogin(cfg)

	templatePath = "webapp" + string(os.PathSeparator) + "templates" + string(os.PathSeparator)

	formtpl, err1 := templates.Asset(templatePath + "inputform.html")
	if err1 != nil {
		log.Fatal(err1)
	}
	errtpl, err2 := templates.Asset(templatePath + "error.html")
	if err2 != nil {
		log.Fatal(err2)
	}
	viewtpl, err3 := templates.Asset(templatePath + "view.html")
	if err3 != nil {
		log.Fatal(err3)
	}
	indextpl, err4 := templates.Asset(templatePath + "index.html")
	if err4 != nil {
		log.Fatal(err4)
	}
	layouttpl, err5 := templates.Asset(templatePath + "layout.html")
	if err5 != nil {
		log.Fatal(err5)
	}
	helptpl, err6 := templates.Asset(templatePath + "help.html")
	if err6 != nil {
		log.Fatal(err6)
	}
	logintpl, err7 := templates.Asset(templatePath + "login.html")
	if err7 != nil {
		log.Fatal(err7)
	}

	templatesMap = make(map[string]*template.Template)

	if t, err := template.New("inputform").Parse(string(layouttpl) + string(formtpl)); err != nil {
		log.Fatal(err)
	} else {
		templatesMap["inputform"] = t
	}

	if t, err := template.New("error").Parse(string(layouttpl) + string(errtpl)); err != nil {
		log.Fatal(err)
	} else {
		templatesMap["error"] = t
	}

	if t, err := template.New("view").Parse(string(layouttpl) + string(viewtpl)); err != nil {
		log.Fatal(err)
	} else {
		templatesMap["view"] = t
	}

	if t, err := template.New("index").Parse(string(layouttpl) + string(indextpl)); err != nil {
		log.Fatal(err)
	} else {
		templatesMap["index"] = t
	}

	if t, err := template.New("help").Funcs(template.FuncMap{"markDown": markDowner}).Parse(string(layouttpl) + string(helptpl)); err != nil {
		log.Fatal(err)
	} else {
		templatesMap["help"] = t
	}

	if t, err := template.New("login").Parse(string(layouttpl) + string(logintpl)); err != nil {
		log.Fatal(err)
	} else {
		templatesMap["login"] = t
	}

	/* HTML handlers */
	http.HandleFunc("/new/", validateHtml(newHandler))                /* Handler for input form */
	http.HandleFunc("/run", validateHtml(runHandler))                 /* Handler for running a new analysis */
	http.HandleFunc("/help", validateHtml(helpHandler))               /* Handler for the help page */
	http.HandleFunc("/view/", validateHtml(makeHandler(viewHandler))) /* Handler for viewing analysis results */
	http.HandleFunc("/itol/", validateHtml(makeHandler(itolHandler))) /* Handler for uploading tree to itol */
	http.HandleFunc("/", validateHtml(indexHandler))                  /* Home Page*/
	http.HandleFunc("/login", loginHandler)                           /* Handler for login */
	http.HandleFunc("/settoken", setToken)                            /* Set token in cookie via form post */
	http.HandleFunc("/gettoken", getToken)                            /* get token via api using json post data */
	http.HandleFunc("/logout", validateHtml(logout))                  /* Handler for logout */

	/* Api handlers */
	http.HandleFunc("/api/analysis/", validateApi(makeApiHandler(apiAnalysisHandler))) /* Handler for returning an analysis */
	http.HandleFunc("/api/image/", validateApi(makeApiImageHandler(apiImageHandler)))  /* Handler for returning a tree image */

	/* Static files handlers : js, css, etc. */
	http.Handle("/static/", http.FileServer(static.AssetFS()))
	//http.Handle("/", http.RedirectHandler("/new/", http.StatusFound))

	port := cfg.GetInt("http.port")
	if port == 0 {
		port = HTTP_PORT_DEFAULT
	}
	log.Print(fmt.Sprintf("HTTP port: %d", port))
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func initProcessor(cfg config.Provider) {
	nbrunners := cfg.GetInt("runners.nbrunners")
	queuesize := cfg.GetInt("runners.queuesize")
	timeout := cfg.GetInt("runners.timeout")
	jobthreads := cfg.GetInt("runners.jobthreads")
	galaxykey := cfg.GetString("runners.galaxykey")
	galaxyurl := cfg.GetString("runners.galaxyurl")
	proctype := cfg.GetString("runners.type")
	switch proctype {
	case "galaxy":
		if galaxyurl == "" {
			log.Fatal("galaxyurl must be provided in configuration file when type=galaxy")
		}
		if galaxykey == "" {
			log.Fatal("galaxykey must be provided in configuration file when type=galaxy")
		}
		galproc := &processor.GalaxyProcessor{}
		galproc.InitProcessor(galaxyurl, galaxykey, db, queuesize)
		proc = galproc
	case "local", "":
		// Local or not set
		locproc := &processor.LocalProcessor{}
		locproc.InitProcessor(nbrunners, queuesize, timeout, jobthreads, db)
		proc = locproc
	default:
		log.Fatal(errors.New("No processor named " + proctype))
	}

}

func initUUIDGenerator() {
	uuids = make(chan string, 100)
	// The uuid generator will put uuids in the channel
	// When a new analysis is launched, one uuid will be taken
	// from the channel
	go func() {
		for {
			u, err := uuid.NewV4()
			if err != nil {
				log.Print(err)
			} else {
				uuids <- u.String()
			}
		}
	}()
}

func initCleanKill() {

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		for sig := range c {
			log.Print(sig)
			proc.CancelAnalyses()
			if err := db.Disconnect(); err != nil {
				log.Print(err)
			}
			os.Exit(1)
		}
	}()
}

func initDB(cfg config.Provider) {
	dbtype := cfg.GetString("database.type")
	switch dbtype {
	case "memory":
		db = database.NewMemoryBoosterWebDB()
	case "mysql":
		user := cfg.GetString("database.user")
		host := cfg.GetString("database.host")
		pass := cfg.GetString("database.pass")
		dbname := cfg.GetString("database.dbname")
		port := cfg.GetInt("database.port")
		db = database.NewMySQLBoosterwebDB(user, pass, host, dbname, port)
		if err := db.Connect(); err != nil {
			log.Fatal(err)
		}
	default:
		db = database.NewMemoryBoosterWebDB()
		log.Print("Database type not valid, using default: " + DATABASE_TYPE_DEFAULT)
	}

	if err := db.InitDatabase(); err != nil {
		log.Fatal(err)
	}
}

func initLog(cfg config.Provider) {
	logf := cfg.GetString("logging.logfile")
	switch logf {
	case "stderr":
		log.Print("Log file: stderr")
		logfile = os.Stderr
	case "stdout":
		log.Print("Log file: stdout")
		logfile = os.Stdout
	case "":
		log.Print("Log file: stderr")
		logfile = os.Stderr
	default:
		log.Print("Log file: " + logf)
		var err error
		logfile, err = os.OpenFile(logf, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.SetOutput(logfile)
}

func initLogin(cfg config.Provider) {
	user := cfg.GetString("authentication.user")
	pass := cfg.GetString("authentication.password")
	if user != "" && pass != "" {
		Authent = true
		Username = user
		Password = pass
	}
}

/*Algorithm: booster or classical */
func newAnalysis(reffile multipart.File, refheader *multipart.FileHeader, bootfile multipart.File, bootheader *multipart.FileHeader, algorithm string) (*model.Analysis, error) {

	algo, e := model.AlgorithmConst(algorithm)
	if e != nil {
		return nil, e
	}

	uuid := <-uuids

	a := &model.Analysis{
		uuid,
		"",
		"",
		"",
		model.STATUS_PENDING,
		algo,
		model.StatusStr(model.STATUS_PENDING),
		"",
		0,
		"",
		time.Now().Format(time.RFC1123),
		"",
		"",
	}

	log.Print(fmt.Sprintf("New analysis submited | id=%s", a.Id))

	/* tmp analysis folder */
	dir, err := ioutil.TempDir("", uuid)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	reftree := filepath.Join(dir, refheader.Filename)
	boottrees := filepath.Join(dir, bootheader.Filename)
	ref, err1 := os.OpenFile(reftree, os.O_WRONLY|os.O_CREATE, 0666)
	if err1 != nil {
		log.Print(err1)
		return nil, err1
	}

	boot, err2 := os.OpenFile(boottrees, os.O_WRONLY|os.O_CREATE, 0666)
	if err2 != nil {
		log.Print(err2)
		return nil, err2
	}

	goio.Copy(ref, reffile)
	goio.Copy(boot, bootfile)
	ref.Close()
	boot.Close()

	a.Reffile = reftree
	a.Bootfile = boottrees

	proc.LaunchAnalysis(a)

	return a, nil
}

func getAnalysis(id string) (a *model.Analysis, err error) {
	a, err = db.GetAnalysis(id)
	return
}

func markDowner(args ...interface{}) template.HTML {
	s := blackfriday.MarkdownCommon([]byte(fmt.Sprintf("%s", args...)))
	return template.HTML(s)
}
