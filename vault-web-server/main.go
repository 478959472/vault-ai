package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pashpashpash/vault/serverutil"
	"github.com/pashpashpash/vault/vectordb"
	"github.com/pashpashpash/vault/vectordb/pinecone"
	"github.com/pashpashpash/vault/vectordb/qdrant"

	"github.com/pashpashpash/vault/vault-web-server/postapi"

	openai "github.com/sashabaranov/go-openai"

	"github.com/codegangsta/negroni"
)

const (
	NegroniLogFmt = `{{.StartTime}} | {{.Status}} | {{.Duration}}
          {{.Method}} {{.Path}}`
	NegroniDateFmt = time.Stamp
)

var (
	debugSite = flag.Bool(
		"debug", false, "debug site")
	port = flag.String(
		"port", "8100", "server port")
	siteConfig = map[string]string{
		"DEBUG_SITE": "false",
	}
)

func main() {
	args := os.Args
	if len(args) < 4 {
		fmt.Println("参数有误，参数数量应为三个，分别为 dbType[qdrant,pinecone] openaiApiKey PINECONE_API_KEY PINECONE_API_ENDPOINT QDRANT_API_ENDPOINT")
		//return
	} else {
		fmt.Printf("参数1：%s\n参数2：%s\n参数3：%s\n参数3：%s\n参数3：%s\n ", args[1], args[2], args[3], args[4], args[5])
	}
	// Parse command line flags + override defaults
	flag.Parse()
	siteConfig["DEBUG_SITE"] = strconv.FormatBool(*debugSite)
	rand.Seed(time.Now().UnixNano())
	dbType := args[1]
	openaiApiKey := os.Getenv("OPENAI_API_KEY")
	if openaiApiKey == "" {
		openaiApiKey = args[2]
	}
	if len(openaiApiKey) == 0 {
		log.Fatalln("MISSING OPENAI API KEY ENV VARIABLE")
	}
	// NewClientWithConfig
	// openaiClient := openai.NewClient(openaiApiKey)
	openaiConfig := openai.DefaultConfig(openaiApiKey)
	openaiConfig.BaseURL = "http://94.74.89.252:7758/5g-openai/v1"
	// openaiConfig.BaseURL = "https://api.openai-proxy.com/v1"
	openaiClient := openai.NewClientWithConfig(openaiConfig)

	log.Println("[main ] openaiConfig.BaseURLt\n", openaiConfig.BaseURL)
	var vectorDB vectordb.VectorDB
	var err error

	qdrantApiEndpoint := args[5] //os.Getenv("QDRANT_API_ENDPOINT")

	if len(qdrantApiEndpoint) != 0 && dbType == "qdrant" {
		vectorDB, err = qdrant.New(qdrantApiEndpoint)
		if err != nil {
			log.Fatalln("ERROR INITIALIZING QDRANT:", err)
		}
	}
	pineconeApiEndpoint := args[4]

	if len(pineconeApiEndpoint) != 0 && dbType == "pinecone" {
		pineconeApiKey := args[3]
		if len(pineconeApiKey) == 0 {
			log.Fatalln("MISSING PINECONE API KEY ENV VARIABLE")
		}

		vectorDB, err = pinecone.New(pineconeApiEndpoint, pineconeApiKey)
		if err != nil {
			log.Fatalln("ERROR INITIALIZING PINECONE:", err)
		}
	}

	if vectorDB == nil {
		log.Fatalln("NO VECTOR DB CONFIGURED (QDRANT_API_ENDPOINT or PINECONE_API_ENDPOINT)")
	}

	handlerContext := postapi.NewHandlerContext(openaiClient, vectorDB)

	// Configure main web server
	server := negroni.New()
	server.Use(negroni.NewRecovery())
	l := negroni.NewLogger()
	l.SetFormat(NegroniLogFmt)
	l.SetDateFormat(NegroniDateFmt)
	server.Use(l)
	mx := mux.NewRouter()

	// Path Routing Rules: [POST]
	mx.HandleFunc("/api/questions", handlerContext.QuestionHandler).Methods("POST")
	mx.HandleFunc("/upload", handlerContext.UploadHandler).Methods("POST")

	// Path Routing Rules: Static Handlers
	mx.HandleFunc("/github", StaticRedirectHandler("https://github.com/pashpashpash/vault"))
	mx.PathPrefix("/").Handler(ReactFileServer(http.Dir(serverutil.WebAbs(""))))

	// Start up web server
	server.UseHandler(mx)

	// if serving on https, need to provide self-signed certs
	if *port == "443" {
		go httpRedirect() // redirect all http to https
		certFile := "/etc/letsencrypt/live/vault.pash.city/fullchain.pem"
		keyFile := "/etc/letsencrypt/live/vault.pash.city/privkey.pem"
		log.Println("[negroni] listening on :443")
		log.Fatal(http.ListenAndServeTLS(":"+*port, certFile, keyFile, server))
	} else {
		server.Run("0.0.0.0:" + *port)
	}
}

// / Takes a response writer Meta config and URL and servers the react app with the correct metadata
func ServeIndex(w http.ResponseWriter, r *http.Request, meta serverutil.SiteConfig) {
	//Here we handle the possible dev environments or pass the basic Hostpath with "/" at the end for the / metadata for each site
	var currentHost string
	var currentSite string

	// create local version of the Global SiteConfig variable to prevent editing concurrent variables.
	var localSiteConfig = map[string]string{}
	for key, element := range siteConfig {
		localSiteConfig[key] = element
	}

	//set the host Manually when on local host
	if r.Host == "localhost:8101" {
		currentHost = "vault.pash.city"
		currentSite = "vault"

	} else {
		currentHost = r.Host
		currentSite = "vault"
	}

	currentpath := currentHost + r.URL.Path
	//check if the currentpath has Metadata associated with it
	//if no metadata is founnd use the default / route
	ctx := r.Context()
	defer ctx.Done()

	// TODO fix metadata api
	currentMetaData := meta.SitePath[currentpath]

	localSiteConfig["PageTitle"] = currentMetaData.PageTitle
	localSiteConfig["PageIcon"] = currentMetaData.PageIcon
	localSiteConfig["MetaType"] = currentMetaData.MetaType
	localSiteConfig["MetaTitle"] = currentMetaData.MetaTitle
	localSiteConfig["MetaDescription"] = currentMetaData.MetaDescription
	localSiteConfig["MetaUrl"] = "https://" + currentHost + r.URL.String()
	localSiteConfig["MetaKeywords"] = currentMetaData.MetaKeywords
	localSiteConfig["Site"] = currentSite
	localSiteConfig["TwitterUsername"] = currentMetaData.TwitterUsername
	/// Here we need to check the type and either add an Image meta tag or a video metatag depending on the result
	if currentMetaData.MetaImage != "!" {
		localSiteConfig["contentType"] = "og:image"
		localSiteConfig["content"] = currentMetaData.MetaImage
	} else {
		localSiteConfig["contentType"] = "og:video"
		if currentMetaData.MetaVideo != "!" {
			localSiteConfig["content"] = currentMetaData.MetaVideo

		} else { //
			localSiteConfig["content"] = ""
			log.Fatalln("Image and video tag missing from JSON template")
		}
	}

	replaceEmpty := func(i string, r string) string {
		if i == "" {
			return r
		}
		return i
	}

	localSiteConfig["MetaTitle"] = replaceEmpty(localSiteConfig["MetaTitle"], "OP Question-Answer Stack")
	localSiteConfig["MetaType"] = replaceEmpty(localSiteConfig["MetaType"], "website")
	localSiteConfig["MetaDescription"] = replaceEmpty(localSiteConfig["MetaDescription"],
		"Upload any number of files (pdf, text, epub) and use them as context when asking OpenAI questions.")
	localSiteConfig["TwitterUsername"] = replaceEmpty(localSiteConfig["TwitterUsername"], "@pashmerepat")
	localSiteConfig["MetaKeywords"] = replaceEmpty(localSiteConfig["MetaKeywords"], "OpenAI, Pinecone, ChatGPT")
	localSiteConfig["PageTitle"] = replaceEmpty(localSiteConfig["PageTitle"], "小梦AI | OP Question-Answer Stack")
	localSiteConfig["PageIcon"] = replaceEmpty(localSiteConfig["Icon"], "/img/logos/vault-favicon.png")
	localSiteConfig["content"] = replaceEmpty(localSiteConfig["content"], "https://i.imgur.com/6YSvyEV.png")
	localSiteConfig["contentType"] = replaceEmpty(localSiteConfig["contentType"], "og:image")
	localSiteConfig["ImageHeight"] = replaceEmpty(localSiteConfig["ImageHeight"], "1024")
	localSiteConfig["ImageWidth"] = replaceEmpty(localSiteConfig["ImageWidth"], "1024")

	t, err := template.ParseFiles(serverutil.WebAbs("index.html"))
	config := struct {
		Config map[string]string
	}{
		localSiteConfig,
	}
	if err != nil {
		log.Fatalln("Critical error parsing index template!", err)
	}

	if err2 := t.Execute(w, config); err2 != nil {
		log.Fatalln("Template execute error!", err)
	}
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Forwards all traffic to React, except basic file serving
func ReactFileServer(fs http.FileSystem) http.Handler {
	fsh := http.FileServer(fs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//get the Metadata Config
		jsonConfig := serverutil.GetConfig()

		if path.Clean(r.URL.Path) == "/" || path.Clean(r.URL.Path) == "/index.html" {
			ServeIndex(w, r, jsonConfig.SiteMetaData)
			return
		}

		if _, err := os.Stat(serverutil.WebAbs(r.URL.Path)); os.IsNotExist(err) {
			ServeIndex(w, r, jsonConfig.SiteMetaData)
			return
		}

		// if gzip not possible serve as is
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			fsh.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		fsh.ServeHTTP(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
	})
}

func StaticRedirectHandler(to string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, to, http.StatusPermanentRedirect)
	}
}

func httpRedirect() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpsURL := fmt.Sprintf("https://%s%s", r.Host, r.URL)
		http.Redirect(w, r, httpsURL, http.StatusMovedPermanently)
	})

	log.Println("HTTP redirect server listening on :80")
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatalf("Error starting HTTP redirect server: %v", err)
	}
}
