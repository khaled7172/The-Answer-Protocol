.PHONY: install run-server run-client run-client-gui lint clean

install:
	go mod tidy

run-server:
	go run ./server/network/

run-client:
	go run ./cli_client/

run-client-gui:
	go run ./gui_client/

lint:
	go vet ./...

clean:
	rm -f tap-server tap-client tap-gui network
