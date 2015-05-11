package main

import (
	"bufio"
	"io/ioutil"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/brentp/xopen"
)

func BenchmarkAnno(b *testing.B) {
	var configs Config
	if _, err := toml.DecodeFile("example/conf.toml", &configs); err != nil {
		panic(err)
	}

	out := bufio.NewWriter(ioutil.Discard)
	Js, _ := xopen.Ropen("example/custom.js")
	jbytes, _ := ioutil.ReadAll(Js)
	js_string := string(jbytes)

	a := NewAnnotator(configs.Sources(), js_string, false, true)
	for n := 0; n < b.N; n++ {
		a.Annotate("example/query.vcf", ioutil.Discard)
		out.Flush()
	}
}
