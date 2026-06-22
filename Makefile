.PHONY: build build-agent install clean

build:
	go build -ldflags="-s -w" -o backup-bot ./cmd/backup-bot

build-agent:
	go build -ldflags="-s -w" -o backup-agent ./cmd/backup-agent

build-all: build build-agent

install: build
	install -m 755 backup-bot /opt/backup-bot/backup-bot

clean:
	rm -f backup-bot backup-agent