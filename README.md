# backupscript

Лёгкий Telegram-бот для Linux-сервера: по команде или по расписанию упаковывает заданные файлы и папки в zip-архив и отправляет в Telegram.

Подходит для бекапа конфигов nginx, базы 3x-ui и любых других путей — всё настраивается через `config.yaml` без правки кода.

## Возможности

- Ручной бекап по команде `/backup` или кнопке
- Автоматический бекап по cron-расписанию
- Whitelist Telegram user ID — доступ только разрешённым пользователям
- Безопасное копирование SQLite (`.db`) перед архивацией
- Проверка лимита Telegram (50 МБ на файл)
- Низкое потребление RAM в простое (~8–15 МБ)

## Команды бота

| Команда   | Описание                              |
|-----------|---------------------------------------|
| `/start`    | Приветствие и кнопка «Сделать бекап»  |
| `/backup`   | Создать архив и отправить в чат       |
| `/schedule` | Интервал автобекапа: `30m`, `6h`, `7d` |
| `/list`     | Пути из конфига и их наличие на диске |
| `/status`   | Последний бекап и статус расписания   |
| `/help`     | Справка                               |

## Требования

- Linux (VPS), root
- Telegram-бот ([@BotFather](https://t.me/BotFather))
- Ваш Telegram user ID ([@userinfobot](https://t.me/userinfobot))
- Go ставится скриптом автоматически (нужен только для сборки)

## Быстрая установка

1. Замените `ВАШ_ТОКЕН` и `ВАШ_USER_ID` в блоке ниже.
2. Скопируйте **весь блок целиком** и вставьте в терминал на сервере.

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
  echo "Укажите TELEGRAM_BOT_TOKEN и ALLOWED_USER_IDS в начале скрипта"
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
go version

if [ -d "$INSTALL_DIR/.git" ]; then
  echo "Обновляю репозиторий..."
  git -C "$INSTALL_DIR" pull
else
  echo "Клонирую репозиторий..."
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

cp backup-bot.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable backup-bot
systemctl restart backup-bot

echo ""
echo "Готово. Статус:"
systemctl --no-pager status backup-bot
echo ""
echo "Логи: journalctl -u backup-bot -f"
echo "Пути бекапа: nano ${INSTALL_DIR}/config.yaml"
```

После установки при необходимости отредактируйте пути бекапа:

```bash
nano /opt/backup-bot/config.yaml
systemctl restart backup-bot
```

Пример `config.yaml` (уже создаётся из `config.yaml.example`):

```yaml
backup:
  name: "server-backup"
  paths:
    - /etc/nginx/nginx.conf
    - /etc/nginx/sites-enabled/
    - /etc/x-ui/x-ui.db
  exclude:
    - "*.log"
    - "*.sock"

schedule:
  enabled: true
  interval: "24h"
  notify_chat_id: null
```

Интервал можно менять прямо в боте (сохраняется в `config.yaml`):

```
/schedule          — текущий интервал
/schedule 6h       — каждые 6 часов
/schedule 7d       — каждые 7 дней
/schedule off      — выключить
/schedule on       — включить с текущим интервалом
```

Формат: число + единица — `m` (минуты), `h` (часы), `d` (дни), `w` (недели). Минимум `1m`.

## Обновление

Скопируйте и вставьте:

```bash
set -e
cd /opt/backup-bot
export PATH="/usr/local/go/bin:$PATH"
git pull
go build -ldflags="-s -w" -o backup-bot ./cmd/backup-bot
systemctl restart backup-bot
systemctl --no-pager status backup-bot
```

## Логи

```bash
journalctl -u backup-bot -f
```

## Ограничения

- Telegram Bot API принимает файлы до **50 МБ**. Если архив больше — бот вернёт ошибку; сузьте `paths` или разбейте бекап.
- Бот не создаёт дампы БД сам — только упаковывает указанные пути.
- Отсутствующие пути пропускаются с предупреждением, остальные включаются в архив.

## Структура проекта

```
cmd/backup-bot/     — точка входа
internal/backup/    — создание zip-архива
internal/bot/       — команды Telegram
internal/config/    — загрузка .env и config.yaml
internal/scheduler/ — cron-расписание
```

## Лицензия

MIT