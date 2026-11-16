.PHONY: install build run-server run-client run-client-gui test lint clean

install:
	go mod download

build:
	go build -o tap-server ./server/network/
	go build -o tap-client ./cli_client/
	go build -o tap-gui ./gui_client/

run-server:
	go run ./server/network/

run-client:
	go run ./cli_client/

run-client-gui:
	go run ./gui_client/

test:
	go test ./...

lint:
	test -z "$$(gofmt -l .)"
	go vet ./...

clean:
	rm -f tap-server tap-client tap-gui
