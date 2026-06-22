#!/usr/bin/env bash
set -eo pipefail

SCRIPT_URL="https://raw.githubusercontent.com/Sp0nge-bob/backupscript/main/scripts/install-master.sh"
INSTALL_DIR="/opt/backup-bot"
GO_VERSION="1.22.5"

if [ "$(id -u)" -ne 0 ]; then
	echo "Запустите от root: sudo -i"
	exit 1
fi

read_prompt() {
	local prompt="$1"
	local __var="$2"
	local secret="${3:-}"
	local value=""

	if [ ! -r /dev/tty ]; then
		echo "Нет TTY. Запустите так:"
		echo "  curl -fsSL -o /tmp/install-master.sh ${SCRIPT_URL}"
		echo "  bash /tmp/install-master.sh"
		echo ""
		echo "Или с аргументами:"
		echo "  bash /tmp/install-master.sh 'BOT_TOKEN' 'USER_ID'"
		exit 1
	fi

	exec 3</dev/tty
	if [ -n "$secret" ]; then
		IFS= read -r -s -u 3 -p "$prompt" value || true
		echo "" >&2
	else
		IFS= read -r -u 3 -p "$prompt" value || true
	fi
	exec 3<&-

	printf -v "$__var" '%s' "$value"
}

install_go() {
	if command -v go >/dev/null 2>&1; then
		return 0
	fi

	echo "Устанавливаю Go ${GO_VERSION}..."
	local go_arch=""
	case "$(uname -m)" in
	x86_64) go_arch="amd64" ;;
	aarch64) go_arch="arm64" ;;
	*)
		echo "Неподдерживаемая архитектура: $(uname -m)"
		exit 1
		;;
	esac

	wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${go_arch}.tar.gz" -O /tmp/go.tar.gz
	rm -rf /usr/local/go
	tar -C /usr/local -xzf /tmp/go.tar.gz
	rm /tmp/go.tar.gz
	export PATH="/usr/local/go/bin:$PATH"
	grep -q '/usr/local/go/bin' /root/.bashrc 2>/dev/null || echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc
}

TELEGRAM_BOT_TOKEN="${1:-}"
ALLOWED_USER_IDS="${2:-}"

if [ -z "$TELEGRAM_BOT_TOKEN" ] || [ -z "$ALLOWED_USER_IDS" ]; then
	echo "=== backupscript: установка master ==="
	echo ""
	echo "Токен: @BotFather  |  User ID: @userinfobot"
	echo ""

	read_prompt "Telegram bot token: " TELEGRAM_BOT_TOKEN hidden
	if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
		echo "Токен не указан."
		exit 1
	fi

	read_prompt "Telegram user ID (можно несколько через запятую): " ALLOWED_USER_IDS
	if [ -z "$ALLOWED_USER_IDS" ]; then
		echo "User ID не указан."
		exit 1
	fi
fi

install_go
export PATH="/usr/local/go/bin:$PATH"

if [ -d "$INSTALL_DIR/.git" ]; then
	git -C "$INSTALL_DIR" pull
else
	git clone https://github.com/Sp0nge-bob/backupscript.git "$INSTALL_DIR"
fi

cd "$INSTALL_DIR"
go build -ldflags="-s -w" -o backup-bot ./cmd/backup-bot

if [ ! -f config.yaml ]; then
	cp config.yaml.example config.yaml
fi

cat > .env <<EOF
TELEGRAM_BOT_TOKEN=${TELEGRAM_BOT_TOKEN}
ALLOWED_USER_IDS=${ALLOWED_USER_IDS}
CONFIG_PATH=${INSTALL_DIR}/config.yaml
BACKUP_TMP_DIR=/tmp/backup-bot
EOF
chmod 600 .env
chmod 600 config.yaml

cp backup-bot.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable backup-bot
systemctl restart backup-bot

echo ""
echo "Готово."
systemctl --no-pager status backup-bot