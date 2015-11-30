// vcfanno is a command-line application and an api for annotating intervals (bed or vcf).
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"

	"time"

	"github.com/BurntSushi/toml"
	"github.com/brentp/irelate"
	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/irelate/parsers"
	. "github.com/brentp/vcfanno/api"
	. "github.com/brentp/vcfanno/shared"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
)

const VERSION = "0.0.9a"

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
	if !(xopen.Exists(queryFile) || queryFile == "") {
		fmt.Fprintf(os.Stderr, "\nERROR: can't find query file: %s\n", queryFile)
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
	/*
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	*/

	jsString := ReadJs(*js)
	strict := !*notstrict
	var a = NewAnnotator(sources, jsString, *ends, strict)

	var out io.Writer = os.Stdout
	defer os.Stdout.Close()

	var err error
	qrdr, err := xopen.Ropen(queryFile)
	if err != nil {
		log.Fatal(fmt.Errorf("error opening query file %s", queryFile, err))
	}
	qstream, query, err := parsers.VCFIterator(qrdr)
	if err != nil {
		log.Fatal(fmt.Errorf("error parsing VCF query file %s", queryFile, err))
	}

	queryables, err := a.Setup(query)
	if err != nil {
		log.Fatal(err)
	}
	aends := INTERVAL
	if *ends {
		aends = BOTH
	}

	fn := func(v interfaces.Relatable) {
		e := a.AnnotateEnds(v, aends)
		if e != nil {
			log.Println(e)
		}
	}

	stream := irelate.PIRelate(6000, 20000, qstream, *ends, fn, queryables...)

	// make a new writer from the string header.
	out, err = vcfgo.NewWriter(out, query.Header)

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
		//log.Printf("%v\n", interval)
		fmt.Fprintln(out, interval)
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
