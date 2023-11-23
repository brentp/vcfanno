all: get build

release:
	CGO_ENABLED=0 GOARCH=amd64 go build -o vcfanno_linux64 --ldflags '-extldflags "-static"' vcfanno.go
	CGO_ENABLED=0 GOARCH=arm64 go build -o vcfanno_linux_aarch64 --ldflags '-extldflags "-static"' vcfanno.go
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o vcfanno_osx --ldflags '-extldflags "-static"' vcfanno.go

get:
	go get -t ./...

build:
	go build -o vcfanno vcfanno.go
