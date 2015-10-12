// vcfanno is a command-line application and an api for annotating intervals (bed or vcf).
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"time"

	"github.com/BurntSushi/toml"
	"github.com/brentp/bix"
	"github.com/brentp/irelate"
	"github.com/brentp/irelate/interfaces"
	. "github.com/brentp/vcfanno/api"
	. "github.com/brentp/vcfanno/shared"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
)

const VERSION = "0.0.8"

func main() {
	fmt.Fprintf(os.Stderr, `
=============================================
vcfanno version %s [built with %s]

see: https://github.com/brentp/vcfanno
=============================================
`, VERSION, runtime.Version())

	ends := flag.Bool("ends", false, "annotate the start and end as well as the interval itself.")
	notstrict := flag.Bool("permissive-overlap", false, "annotate with an overlapping variant even it doesn't"+
		" share the same ref and alt alleles. Default is to require exact match between variants.")
	js := flag.String("js", "", "optional path to a file containing custom javascript functions to be used as ops")
	base := flag.String("base-path", "", "optional base-path to prepend to annotation files in the config")
	procs := flag.Int("p", 2, "number of processes to use. default is 2")
	flag.Parse()
	inFiles := flag.Args()
	if len(inFiles) != 2 {
		fmt.Printf(`Usage:
%s config.toml intput.vcf > annotated.vcf

To run a server:

%s server

`, os.Args[0], os.Args[0])
		flag.PrintDefaults()
		return
	}
	queryFile := inFiles[1]
	if !(xopen.Exists(queryFile)) {
		fmt.Fprintf(os.Stderr, "\nERROR: can't find query file: %s\n", queryFile)
		os.Exit(2)
	}
	if !(xopen.Exists(queryFile + ".tbi")) {
		fmt.Fprintf(os.Stderr, "\nERROR: can't find index for query file: %s\n", queryFile)
		os.Exit(2)
	}
	runtime.GOMAXPROCS(*procs)

	var config Config
	if _, err := toml.DecodeFile(inFiles[0], &config); err != nil {
		panic(err)
	}
	config.Base = *base
	for _, a := range config.Annotation {
		err := CheckAnno(&a)
		if err != nil {
			log.Fatal("CheckAnno err:", err)
		}
	}
	sources, e := config.Sources()
	if e != nil {
		log.Fatal(e)
	}

	log.Printf("found %d sources from %d files\n", len(sources), len(config.Annotation))
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	jsString := ReadJs(*js)
	strict := !*notstrict
	var a = NewAnnotator(sources, jsString, *ends, strict)

	var out io.Writer = os.Stdout
	defer os.Stdout.Close()

	var err error
	var bx interfaces.RelatableIterator
	b, err := bix.New(queryFile, 1)

	a.UpdateHeader(b)

	if err != nil {
		log.Fatal(err)
	}
	bx, err = b.Query(nil)
	if err != nil {
		log.Fatal(err)
	}

	files, err := a.SetupStreams()
	if err != nil {
		log.Fatal(err)
	}
	aends := INTERVAL
	if *ends {
		aends = BOTH
	}

	fn := func(v interfaces.Relatable) {
		a.AnnotateEnds(v, aends)
	}

	queryables := make([]interfaces.Queryable, len(files))
	for i, f := range files {
		q, err := bix.New(f, 1)
		if err != nil {
			log.Fatal(err)
		}
		queryables[i] = q
	}
	stream := irelate.PIRelate(6000, 70000, bx, *ends, fn, queryables...)

	// make a reader from the string header.
	hdr := strings.NewReader(b.Header)
	v, err := vcfgo.NewReader(hdr, true)
	out, err = vcfgo.NewWriter(out, v.Header)

	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	n := 0

	if os.Getenv("IRELATE_PROFILE") == "TRUE" {
		log.Println("profiling to: irelate.pprof")
		f, err := os.Create("irelate.pprof")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	for interval := range stream {
		fmt.Fprintf(out, "%s\n", interval)
		n++
	}
	printTime(start, n)
}

func printTime(start time.Time, n int) {
	dur := time.Since(start)
	duri, duru := dur.Seconds(), "second"
	if duri > float64(600) {
		duri, duru = dur.Minutes(), "minute"
	}
	log.Printf("annotated %d variants in %.2f %ss (%.1f / %s)", n, duri, duru, float64(n)/duri, duru)
}
