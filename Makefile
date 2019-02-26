all: get build

get:
	go get github.com/brentp/irelate
	go get github.com/brentp/irelate/interfaces
	go get github.com/brentp/irelate/parsers
	go get github.com/brentp/vcfanno/api
	go get github.com/brentp/vcfanno/shared
	go get github.com/brentp/vcfgo
	go get github.com/brentp/xopen

build:
	go build -o vcfanno vcfanno.go
