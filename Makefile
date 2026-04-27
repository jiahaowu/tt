.PHONY: install clean release test

BIN     := tt
PREFIX  := $(HOME)/bin

install: $(BIN)
	cp $(BIN) $(PREFIX)/$(BIN)
	@echo "✅ Installed to $(PREFIX)/$(BIN)"

$(BIN):
	go build -ldflags="-s -w" -o $(BIN) .

clean:
	rm -f $(BIN)

release: $(BIN)
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o $(BIN)-darwin-arm64 .
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o $(BIN)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)-windows-amd64.exe .
	@echo "✅ All binaries built"

test:
	go vet ./...
	@echo "✅ Vet passed"
