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

	"time"

	"github.com/BurntSushi/toml"
	"github.com/brentp/bix"
	"github.com/brentp/irelate"
	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/irelate/parsers"
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
	lexsort := flag.Bool("lexicographical", false, "expect chromosomes in order of 1,10,11 ... 19, 2, 20... "+
		" default is 1, 10, 11, ..., 19, 2, 20... . All files must be in the same order.")
	region := flag.String("region", "", "optional region (chrom:start-end) to restrict annnotation. Useful for parallelization")
	base := flag.String("base-path", "", "optional base-path to prepend to annotation files in the config")
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
	if !(xopen.Exists(queryFile) || queryFile == "-") {
		fmt.Fprintf(os.Stderr, "\nERROR: can't find query file: %s\n", queryFile)
		os.Exit(2)
	}

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
	var a = NewAnnotator(sources, jsString, *ends, strict, !*lexsort, *region)

	var out io.Writer = os.Stdout
	defer os.Stdout.Close()

	var rdr *vcfgo.Reader
	var err error
	var q io.Reader

	if *region == "" {
		q, err = xopen.Ropen(queryFile)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		bx, err := bix.New(queryFile)
		if err != nil {
			log.Fatal(err)
		}
		chrom, start, end, err := irelate.RegionToParts(*region)
		if err != nil {
			log.Fatal(err)
		}
		q, err = bx.Query(chrom, start, end, true)
		if err != nil {
			log.Fatal(err)
		}
	}

	qs, rdr, err := parsers.VCFIterator(q)
	if err != nil {
		log.Fatal(err)
	}

	a.UpdateHeader(rdr)

	files, err := a.SetupStreams()
	if err != nil {
		log.Fatal(err)
	}

	fn := func(v interfaces.Relatable) {
		a.AnnotateOne(v, a.Strict)
	}

	stream := irelate.PIRelate(5000, 60000, qs, fn, files...)

	out, err = vcfgo.NewWriter(out, rdr.Header)
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
	if rdr != nil {
		if e := rdr.Error(); e != nil {
			log.Println(e)
		}
	}

}

func printTime(start time.Time, n int) {
	dur := time.Since(start)
	duri, duru := dur.Seconds(), "second"
	if duri > float64(600) {
		duri, duru = dur.Minutes(), "minute"
	}
	log.Printf("annotated %d variants in %.2f %ss (%.1f / %s)", n, duri, duru, float64(n)/duri, duru)
}
