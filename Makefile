.PHONY: install clean release test

BIN    := tt
PREFIX := $(HOME)/bin

# ─── Auto-detect OS ──────────────────────────────────────────────────────────
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	GOOS := darwin
else
	GOOS := linux
endif

# ─── Auto-detect Arch ────────────────────────────────────────────────────────
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),arm64)
	GOARCH := arm64
else ifeq ($(UNAME_M),aarch64)
	GOARCH := arm64
else
	GOARCH := amd64
endif

# ─── Targets ─────────────────────────────────────────────────────────────────

$(BIN):
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="-s -w" -o $(BIN) .

install: $(BIN)
	@mkdir -p $(PREFIX)
	cp $(BIN) $(PREFIX)/$(BIN)
	@echo "✅ $(BIN) → $(PREFIX)/$(BIN)  (OS: $(GOOS), Arch: $(GOARCH))"

clean:
	rm -f $(BIN) $(BIN)-*

release: clean
	GOOS=darwin  GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -ldflags="-s -w" -o $(BIN)-darwin-arm64 .
	GOOS=linux   GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -ldflags="-s -w" -o $(BIN)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)-windows-amd64.exe .
	@echo "✅ All 5 binaries built"

test:
	go vet ./...
	@echo "✅ Vet passed"
