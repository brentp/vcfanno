package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
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

	mpr, err := r.MultipartReader()
	if check(err, w) {
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
	defer conn.Close()

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

	annot := api.NewAnnotator(h.config.Sources(), h.jsString, true, true, true)

	if check(err, w) {
		return
	}

	vcfWriter, err := vcfgo.NewWriter(wh, rdr.Header)
	if check(err, w) {
		return
	}

	streams, err := annot.SetupStreams(queryStream)
	if check(err, w) {
		return
	}

	for interval := range annot.Annotate(streams...) {
		fmt.Fprintf(vcfWriter, "%s\n", interval)
	}
	if err := wh.Flush(); err != nil {
		log.Println(err)
	}
}

func main() {
	var h AnnoHandler
	cfg := os.Args[1]
	basepath := os.Args[2]
	if !xopen.Exists(cfg) {
		fmt.Errorf("config not found %s", cfg)
	}

	var config shared.Config
	if _, err := toml.DecodeFile(cfg, &config); err != nil {
		log.Fatal(err)
	}
	config.Base = basepath
	for _, a := range config.Annotation {
		err := shared.CheckAnno(&a)
		if err != nil {
			log.Fatal("checkAnno err:", err)
		}
	}

	h.config = config
	var js string
	if len(os.Args) > 3 {
		js = os.Args[3]
		h.jsString = shared.ReadJs(js)
	}
	http.Handle("/", h)
	http.ListenAndServe(":8080", nil)
}
