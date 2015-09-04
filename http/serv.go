package server

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/brentp/irelate"
	"github.com/brentp/vcfanno/api"
	"github.com/brentp/vcfanno/shared"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
)

type AnnoHandler struct {
	config   shared.Config
	jsString string
}

func check(e error, w http.ResponseWriter) bool {
	if e != nil {
		log.Println(e)
		http.Error(w, e.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

func (h AnnoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" && r.Method != "PUT" {
		w.Write([]byte("must POST or PUT a VCF"))
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Println("hijacking not supported")
		http.Error(w, "hijacking not supported", 500)
		return
	}
	conn, wh, err := hj.Hijack()
	if check(err, w) {
		return
	}

	mpr, err := r.MultipartReader()
	if check(err, w) {
		return
	}

	defer conn.Close()
	//conn.Write([]byte("Content-Type: text/plain\n"))
	//conn.Write([]byte("Content-Disposition: attachment\n"))

	var vcf io.Reader
	var wtr io.WriteCloser
	vcf, wtr = io.Pipe()

	// This and the hijacking are to support streaming the upload
	// *while* streaming the download.
	// Without this, it's not possible to read from r.Body once we've
	// written to the writer.
	go func() {
		for {
			part, err := mpr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				wh.WriteString(fmt.Sprintf("%s", err))
				log.Println(err)
				return
			}
			io.Copy(wtr, part)
			log.Println("copied")
		}
		wtr.Close()
	}()

	vcf = bufio.NewReader(vcf)
	if is, err := xopen.IsGzip(vcf.(*bufio.Reader)); err == nil && is {
		vcf, err = gzip.NewReader(vcf)
		if check(err, w) {
			return
		}
	}

	var rdr *vcfgo.Reader
	if rdr, err = vcfgo.NewReader(vcf, true); check(err, w) {
		return
	}
	queryStream := irelate.StreamVCF(rdr)

	srcs, err := h.config.Sources()
	if check(err, w) {
		return
	}
	annot := api.NewAnnotator(srcs, h.jsString, true, true, true, "")

	vcfWriter, err := vcfgo.NewWriter(wh, rdr.Header)
	if check(err, w) {
		return
	}

	streams, getters, err := annot.SetupStreams(queryStream)
	if check(err, w) {
		return
	}

	for interval := range annot.Annotate(streams, getters) {
		fmt.Fprintf(vcfWriter, "%s\n", interval)
	}
	if err := wh.Flush(); err != nil {
		log.Println(err)
	}
}

func Server() {
	var h AnnoHandler
	// os.Args[1] == "server"
	if os.Args[1] == "server" {
		if len(os.Args) > 1 {
			os.Args = append(os.Args[:1], os.Args[2:]...)
		} else {
			os.Args = append(os.Args[:1])
		}
	}

	port := flag.String("port", ":8080", "port for server")
	js := flag.String("js", "", "optional path to a file containing custom javascript functions to be used as ops")
	base := flag.String("base-path", "", "optional base-path to prepend to annotation files in the config")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 || !xopen.Exists(args[0]) {
		fmt.Printf("must send in path to config file\n")
		flag.PrintDefaults()
		return
	}
	cfg := args[0]

	if !xopen.Exists(cfg) {
		log.Fatalf("config not found %s", cfg)
	}

	var config shared.Config
	if _, err := toml.DecodeFile(cfg, &config); err != nil {
		log.Fatal(err)
	}
	config.Base = *base
	for _, a := range config.Annotation {
		err := shared.CheckAnno(&a)
		if err != nil {
			log.Fatal("checkAnno err:", err)
		}
	}

	h.config = config
	if *js != "" {
		h.jsString = shared.ReadJs(*js)
	}
	http.Handle("/", h)
	fmt.Fprintf(os.Stderr, "\nstarting vcfanno server on port %s\n", *port)
	log.Println(http.ListenAndServe(*port, nil))
}
