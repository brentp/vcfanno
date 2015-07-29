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
	vhttp "github.com/brentp/vcfanno/http"
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

	if len(os.Args) > 1 && os.Args[1] == "server" {
		vhttp.Server()
		os.Exit(0)
	}

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
		cadd, err = config.Cadd(rdr.Header, a.Ends)
		if err != nil {
			log.Fatal(err)
		}

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
		cadd3(cadd, interval)
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

func cadd3(cadd *CaddIdx, interval irelate.Relatable) {
	if cadd == nil {
		return
	}
	var v *irelate.Variant
	var ok bool

	if v, ok = interval.(*irelate.Variant); !ok {
		return
	}
	caddAnno(cadd, v, "")
	cip0, cip1, okp := v.CIPos()
	cie0, cie1, oke := v.CIEnd()
	ends := []string{LEFT, RIGHT}
	oks := []bool{okp, oke}
	for i, lr := range [][]uint32{{cip0, cip1}, {cie0, cie1}} {
		if !oks[i] {
			continue
		}
		l, r := lr[0], lr[1]
		prefix := ends[i]

		if l == v.Start() && r == v.End() {
			return
		}
		pos, ref, alt := v.Pos, v.Ref, v.Alt
		v.Ref, v.Alt = "A", []string{"<DUP>"}
		svlen, _ := v.Info.Get("SVLEN")
		v.Pos = uint64(l + 1)
		v.Info.Set("SVLEN", r-l-1)
		caddAnno(cadd, v, prefix)
		v.Pos, v.Ref, v.Alt = pos, ref, alt
		if svlen != nil && svlen != "" {
			v.Info.Set("SVLEN", svlen)
		} else {
			v.Info.Delete("SVLEN")
		}
	}
}

// if the cadd index was requested, annotate the variant.
func caddAnno(cadd *CaddIdx, v *irelate.Variant, prefix string) {
	if cadd == nil {
		return
	}

	for _, src := range cadd.Sources {
		vals := make([][]interface{}, len(v.Alt))
		vStr := make([]string, len(v.Alt))
		// handle multiple alts.
		for iAlt, alt := range v.Alt {
			vals[iAlt] = make([]interface{}, 0)
			// report list of changes from ref[i] to C.
			for pos := int(v.Start()) + 1; pos <= int(v.End()); pos++ {
				score, err := cadd.Idx.At(v.Chrom(), pos, alt)
				if err != nil && (alt[0] != '<' && !(alt[0] == ']' || alt[0] == '[' || alt[len(alt)-1] == ']' || alt[len(alt)-1] == '[')) {
					log.Println("cadd error:", err)
				}
				vals[iAlt] = append(vals[iAlt], score)
			}
			src.AnnotateOne(v, vals[iAlt], prefix)
			vStr[iAlt] = string(v.Info.SGet(prefix + src.Name))
			v.Info.Delete(prefix + src.Name)
		}
		v.Info.Set(prefix+src.Name, strings.Join(vStr, ","))

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
