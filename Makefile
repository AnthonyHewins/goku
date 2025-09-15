.DEFAULT: goku

goku:
	go build  -o bin/goku ./cmd/goku

test:
	go test ./...
