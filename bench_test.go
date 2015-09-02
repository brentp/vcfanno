package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/brentp/irelate"
	"github.com/brentp/vcfanno/api"
	"github.com/brentp/vcfanno/shared"
	"github.com/brentp/xopen"
)

func benchmarkAnno(b *testing.B, natural bool) {
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
	a := api.NewAnnotator(srcs, js_string, false, true, natural)
	for n := 0; n < b.N; n++ {
		q := irelate.Vopen("example/query.vcf")
		stream := irelate.StreamVCF(q)
		streams, getters, _ := a.SetupStreams(stream)

		for interval := range a.Annotate(streams, getters) {
			fmt.Fprintf(out, "%s\n", interval)
		}
		out.Flush()
	}
}

func BenchmarkNormal(b *testing.B)  { benchmarkAnno(b, false) }
func BenchmarkNatural(b *testing.B) { benchmarkAnno(b, true) }
