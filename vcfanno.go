// vcfanno is a command-line application and an api for annotating intervals (bed or vcf).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/brentp/irelate"
	. "github.com/brentp/vcfanno/api"
	. "github.com/brentp/vcfanno/shared"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
)

const VERSION = "0.0.7"

func main() {
	fmt.Fprintf(os.Stderr, `
vcfanno version %s

see: https://github.com/brentp/vcfanno
`, VERSION)

	ends := flag.Bool("ends", false, "annotate the start and end as well as the interval itself.")
	notstrict := flag.Bool("permissive-overlap", false, "annotate with an overlapping variant even it doesn't"+
		" share the same ref and alt alleles. Default is to require exact match between variants.")
	js := flag.String("js", "", "optional path to a file containing custom javascript functions to be used as ops")
	lexsort := flag.Bool("lexicographical", false, "expect chromosomes in order of 1,10,11 ... 19, 2, 20... "+
		" default is 1, 10, 11, ..., 19, 2, 20... . All files must be in the same order.")
	base := flag.String("base-path", "", "optional base-path to prepend to annotation files in the config")
	flag.Parse()
	inFiles := flag.Args()
	if len(inFiles) != 2 {
		fmt.Printf(`Usage:
%s config.toml intput.vcf > annotated.vcf

`, os.Args[0])
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
	sources := config.Sources()
	log.Printf("found %d sources from %d files\n", len(sources), len(config.Annotation))

	jsString := ReadJs(*js)
	strict := !*notstrict
	var a = NewAnnotator(sources, jsString, *ends, strict, !*lexsort)

	var out io.Writer = os.Stdout
	defer os.Stdout.Close()

	var rdr *vcfgo.Reader
	var err error
	var queryStream irelate.RelatableChannel
	if strings.HasSuffix(queryFile, ".bed") || strings.HasSuffix(queryFile, ".bed.gz") {
		queryStream, err = irelate.Streamer(queryFile)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		rdr = irelate.Vopen(queryFile)
		queryStream = irelate.StreamVCF(rdr)
	}

	streams, err := a.SetupStreams(queryStream)
	if err != nil {
		log.Fatal(err)
	}
	var cadd *CaddIdx

	if nil != rdr { // it was vcf, print the header
		var err error
		cadd = config.Cadd(rdr.Header, a.Ends)
		out, err = vcfgo.NewWriter(out, rdr.Header)
		if err != nil {
			log.Fatal(err)
		}

	} else {
		bw := bufio.NewWriter(out)
		defer bw.Flush()
		out = bw
	}
	start := time.Now()
	n := 0

	for interval := range a.Annotate(streams...) {
		caddAnno(cadd, interval)
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

// if the cadd index was requested, annotate the variant.
func caddAnno(cadd *CaddIdx, interval irelate.Relatable) {
	if cadd != nil {
		if v, ok := interval.(*irelate.Variant); ok {
			for _, src := range cadd.Sources {
				vals := make([][]interface{}, len(v.Alt))
				vStr := make([]string, len(v.Alt))
				// handle multiple alts.
				for iAlt, alt := range v.Alt {
					vals[iAlt] = make([]interface{}, 0)
					// e.g ref is ACTGC, alt is C, report list of changes from ref[i] to C.
					if len(alt) == 1 {
						for pos := int(interval.Start()) + 1; pos <= int(interval.End()); pos++ {
							score, err := cadd.Idx.At(interval.Chrom(), pos, alt)
							if err != nil {
								log.Println("cadd error:", err)
							}
							vals[iAlt] = append(vals[iAlt], score)
						}
					} else {
						// take the flanking positions.
						for j, pos := range []int{int(interval.Start() + 1), int(interval.End())} {
							k := 0
							if j > 0 {
								k = len(alt) - 1
							}
							score, err := cadd.Idx.At(interval.Chrom(), pos, alt[k:k+1])
							if err != nil {
								log.Println("cadd error:", err)
							}
							vals[iAlt] = append(vals[iAlt], score)
						}
					}
					// TODO: handle ends (left, right end of SV?)
					src.AnnotateOne(v, vals[iAlt], "")
					vStr[iAlt] = string(v.Info.SGet(src.Name))
					v.Info.Delete(src.Name)
				}
				v.Info.Set(src.Name, strings.Join(vStr, ","))
			}
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
