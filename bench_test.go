package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/brentp/bix"
	"github.com/brentp/irelate"
	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/vcfanno/api"
	"github.com/brentp/vcfanno/shared"
	"github.com/brentp/xopen"
)

func benchmarkAnno(b *testing.B) {
	var configs shared.Config
	if _, err := toml.DecodeFile("example/conf.toml", &configs); err != nil {
		panic(err)
	}

	out := bufio.NewWriter(ioutil.Discard)
	Js, _ := xopen.Ropen("example/custom.js")
	jbytes, _ := ioutil.ReadAll(Js)
	js_string := string(jbytes)

	srcs, err := configs.Sources()
	if err != nil {
		log.Fatal(err)
	}
	a := api.NewAnnotator(srcs, js_string, false, true)
	files, err := a.SetupStreams()
	if err != nil {
		log.Fatal(err)
	}
	for n := 0; n < b.N; n++ {
		q, err := bix.New("example/query.vcf.gz", 1)
		if err != nil {
			log.Fatal(err)
		}
		bx, err := q.Query(nil)
		if err != nil {
			log.Fatal(err)
		}
		a.UpdateHeader(q)

		//files := []string{"example/cadd.sub.txt.gz", "example/query.vcf.gz", "example/fitcons.bed.gz"}
		//files := []string{"example/query.vcf.gz", "example/fitcons.bed.gz"}

		queryables := make([]interfaces.Queryable, len(files))
		for i, f := range files {
			q, err := bix.New(f, 1)
			if err != nil {
				log.Fatal(err)
			}
			queryables[i] = q
		}

		fn := func(v interfaces.Relatable) {
			a.AnnotateEnds(v, api.INTERVAL)
		}
		stream := irelate.PIRelate(4000, 20000, bx, false, fn, queryables...)

		for interval := range stream {
			fmt.Fprintf(out, "%s\n", interval)
		}
		out.Flush()
	}
}

func BenchmarkNormal(b *testing.B) { benchmarkAnno(b) }
