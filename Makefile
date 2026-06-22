.PHONY: build install clean

build:
	go build -ldflags="-s -w" -o backup-bot ./cmd/backup-bot

install: build
	install -m 755 backup-bot /opt/backup-bot/backup-bot

clean:
	rm -f backup-bot