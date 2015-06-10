package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/brentp/vcfanno/api"
	"github.com/brentp/xopen"
)

func benchmarkAnno(b *testing.B, natural bool) {
	var configs Config
	if _, err := toml.DecodeFile("example/conf.toml", &configs); err != nil {
		panic(err)
	}

	out := bufio.NewWriter(ioutil.Discard)
	Js, _ := xopen.Ropen("example/custom.js")
	jbytes, _ := ioutil.ReadAll(Js)
	js_string := string(jbytes)

	a := api.NewAnnotator(configs.Sources(), js_string, false, true, natural)
	for n := 0; n < b.N; n++ {
		streams, _ := a.SetupStreams("example/query.vcf")
		for interval := range a.Annotate(streams...) {
			fmt.Fprintf(out, "%s\n", interval)
		}
		out.Flush()
	}
}

func BenchmarkNormal(b *testing.B)  { benchmarkAnno(b, false) }
func BenchmarkNatural(b *testing.B) { benchmarkAnno(b, true) }
