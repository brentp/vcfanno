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
	"strconv"
	"sync"
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

var VERSION = "0.0.11"

func envGet(name string, vdefault int) int {
	sval := os.Getenv(name)
	var err error
	if sval != "" {
		vdefault, err = strconv.Atoi(sval)
		if err != nil {
			log.Printf("couldn't parse %s using %d\n", name, vdefault)
		} else {
			log.Printf("using %s of %d\n", name, vdefault)
		}
	}
	return vdefault
}

func init() {
	log.SetFlags(log.Lshortfile)
}

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
	lua := flag.String("lua", "", "optional path to a file containing custom javascript functions to be used as ops")
	base := flag.String("base-path", "", "optional base-path to prepend to annotation files in the config")
	procs := flag.Int("p", 2, "number of processes to use.")
	flag.Parse()
	inFiles := flag.Args()
	if len(inFiles) != 2 {
		fmt.Printf(`Usage:
%s config.toml input.vcf > annotated.vcf

`, os.Args[0])
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
		for _, op := range a.Ops {
			if len(op) > 4 && op[:4] == "lua:" && *lua == "" {
				log.Fatal("ERROR: requested lua op without specifying -lua flag")
			}
		}
	}
	for i := range config.PostAnnotation {
		r := config.PostAnnotation[i]
		err := CheckPostAnno(&r)
		if err != nil {
			log.Fatal(fmt.Sprintf("error in postannotaion section %s err: %s", r.Name, err))
		}
		if len(r.Op) > 4 && r.Op[:4] == "lua:" && *lua == "" {
			log.Fatal("ERROR: requested lua op without specifying -lua flag")
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

	luaString := ReadLua(*lua)
	strict := !*notstrict
	var a = NewAnnotator(sources, luaString, *ends, strict, config.PostAnnotation)

	var out io.Writer = os.Stdout
	defer os.Stdout.Close()

	var err error
	qrdr, err := xopen.Ropen(queryFile)
	if err != nil {
		log.Fatal(fmt.Errorf("error opening query file %s: %s", queryFile, err))
	}
	qstream, query, err := parsers.VCFIterator(qrdr)
	if err != nil {
		log.Fatal(fmt.Errorf("error parsing VCF query file %s: %s", queryFile, err))
	}

	queryables, err := a.Setup(query)
	if err != nil {
		log.Fatal(err)
	}
	aends := INTERVAL
	if *ends {
		aends = BOTH
	}

	lastMsg := struct {
		sync.RWMutex
		s [3]string
		i int
	}{}

	fn := func(v interfaces.Relatable) {
		e := a.AnnotateEnds(v, aends)
		if e != nil {
			lastMsg.RLock()
			em := e.Error()
			if em != lastMsg.s[0] && em != lastMsg.s[1] && em != lastMsg.s[2] {
				log.Println(e, ">> this error may occur many times. reporting once here...")
				lastMsg.RUnlock()
				lastMsg.Lock()
				lastMsg.s[lastMsg.i] = em
				if lastMsg.i == len(lastMsg.s)-1 {
					lastMsg.i = -1
				}
				lastMsg.i++

				lastMsg.Unlock()
			} else {
				lastMsg.RUnlock()
			}
		}
	}

	maxGap := envGet("IRELATE_MAX_GAP", 20000)
	maxChunk := envGet("IRELATE_MAX_CHUNK", 8000)

	stream := irelate.PIRelate(maxChunk, maxGap, qstream, *ends, fn, queryables...)

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
	log.Printf("annotated %d variants in %.2f %ss (%.1f / %s)", n, duri, duru, float64(n)/duri, duru)
}
