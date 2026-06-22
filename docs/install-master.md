# Установка master

Master — главный сервер с Telegram-ботом и HTTP API для agent-нод (порт 9876).

## Перед установкой

1. Создайте бота у [@BotFather](https://t.me/BotFather) и скопируйте токен.
2. Узнайте свой Telegram user ID у [@userinfobot](https://t.me/userinfobot).
3. Подключитесь к VPS по SSH под **root**.

## Скрипт установки

Замените `ВАШ_ТОКЕН` и `ВАШ_USER_ID`, скопируйте **весь блок** и вставьте в терминал master:

```bash
set -e

# ========== НАСТРОЙТЕ ==========
TELEGRAM_BOT_TOKEN="ВАШ_ТОКЕН"
ALLOWED_USER_IDS="ВАШ_USER_ID"
# ================================

INSTALL_DIR="/opt/backup-bot"
GO_VERSION="1.22.5"

if [ "$(id -u)" -ne 0 ]; then
  echo "Запустите от root: sudo -i"
  exit 1
fi

if [ "$TELEGRAM_BOT_TOKEN" = "ВАШ_ТОКЕН" ] || [ "$ALLOWED_USER_IDS" = "ВАШ_USER_ID" ]; then
  echo "Укажите TELEGRAM_BOT_TOKEN и ALLOWED_USER_IDS"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Устанавливаю Go ${GO_VERSION}..."
  case "$(uname -m)" in
    x86_64)  GO_ARCH="amd64" ;;
    aarch64) GO_ARCH="arm64" ;;
    *) echo "Неподдерживаемая архитектура: $(uname -m)"; exit 1 ;;
  esac
  wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -O /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
  export PATH="/usr/local/go/bin:$PATH"
  grep -q '/usr/local/go/bin' /root/.bashrc 2>/dev/null || echo 'export PATH=$PATH:/usr/local/go/bin' >> /root/.bashrc
fi

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

echo "Готово."
systemctl --no-pager status backup-bot
```

## После установки

1. Отредактируйте конфиг: [Конфигурация](config.md)
2. Подключите ноды:
   - [SSH-нода](install-ssh-node.md)
   - [Agent-нода](install-agent-node.md)
3. В Telegram отправьте боту `/backup`

## Проверка

```bash
systemctl status backup-bot
journalctl -u backup-bot -f
```

## См. также

- [Конфигурация](config.md)
- [Команды бота](commands.md)
- [Обновление и логи](operations.md)