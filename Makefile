.DEFAULT: goku

version := $(shell git rev-parse HEAD)
goku:
	go build -ldflags="-X 'main.version=$(version)'" -o ./bin/$@ ./cmd/$@

test:
	go test ./...
