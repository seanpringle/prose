all:
	GOPATH=/home/sean/go go build -o prose *.go
	./prose

prof:
	GOPATH=/home/sean/go go build -o prose *.go
	./prose -profile
	go tool pprof -callgrind -output=profile.grind prose profile
