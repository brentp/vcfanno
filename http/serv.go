package main

import (
	"fmt"
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

func (h AnnoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" && r.Method != "PUT" {
		w.Write([]byte("must POST or PUT a VCF"))
		return
	}
	err := r.ParseMultipartForm(100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println(r.MultipartForm.File)
	handle := r.MultipartForm.File["vcf"][0]
	vcf, err := handle.Open()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer vcf.Close()

	rdr, err := vcfgo.NewReader(vcf, true)
	log.Println(err)
	queryStream := irelate.StreamVCF(rdr)

	annot := api.NewAnnotator(h.config.Sources(), h.jsString, true, true, true)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	vcfWriter, err := vcfgo.NewWriter(w, rdr.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	streams := annot.SetupStreams(queryStream)
	for interval := range annot.Annotate(streams...) {
		fmt.Fprintf(vcfWriter, "%s\n", interval)
	}
}

func main() {
	var h AnnoHandler
	cfg := os.Args[1]
	if !xopen.Exists(cfg) {
		fmt.Errorf("config not found %s", cfg)
	}

	var config shared.Config
	if _, err := toml.DecodeFile(cfg, &config); err != nil {
		log.Fatal(err)
	}
	for _, a := range config.Annotation {
		err := shared.CheckAnno(&a)
		if err != nil {
			log.Fatal("checkAnno err:", err)
		}
	}

	h.config = config
	var js string
	if len(os.Args) > 2 {
		js = os.Args[2]
		h.jsString = shared.ReadJs(js)
	}
	http.Handle("/", h)
	http.ListenAndServe(":8080", nil)
}
