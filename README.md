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
| `/start`  | Приветствие и кнопка «Сделать бекап»  |
| `/backup` | Создать архив и отправить в чат       |
| `/list`   | Пути из конфига и их наличие на диске |
| `/status` | Последний бекап и статус расписания   |
| `/help`   | Справка                               |

## Требования

- Linux (VPS)
- Go 1.22+ (только для сборки; после сборки можно удалить)
- Telegram-бот ([@BotFather](https://t.me/BotFather))
- Доступ к файлам бекапа (часто нужен `root` для `/etc/nginx`, `/etc/x-ui`)

## Установка Go

Go нужен только чтобы собрать бинарник. На сервере без `go` в PATH сначала установите его.

### Способ 1 — официальный архив (рекомендуется)

Подходит для Ubuntu, Debian и других дистрибутивов. Версия из репозитория дистрибутива часто слишком старая.

Скопируйте блок **целиком**. Если выполнять команды по одной — сначала задайте версию: `export GO_VERSION=1.22.5`, иначе `wget` скачает несуществующий `go.linux-amd64.tar.gz` (404).

```bash
export GO_VERSION=1.22.5

# amd64 (большинство VPS)
wget "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
tar -C /usr/local -xzf "go${GO_VERSION}.linux-amd64.tar.gz"
rm "go${GO_VERSION}.linux-amd64.tar.gz"

# для ARM64 (например Oracle Ampere) вместо amd64:
# wget "https://go.dev/dl/go${GO_VERSION}.linux-arm64.tar.gz"
# tar -C /usr/local -xzf "go${GO_VERSION}.linux-arm64.tar.gz"
# rm "go${GO_VERSION}.linux-arm64.tar.gz"

echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

go version
```

Должно вывести `go version go1.22.5 linux/amd64` (или вашу архитектуру).

Либо без переменных — прямая ссылка:

```bash
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
rm go1.22.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version
```

### Способ 2 — через apt (Ubuntu / Debian)

Быстрее, но проверьте версию — нужна **1.22 или новее**:

```bash
apt update && apt install -y golang-go
go version
```

Если версия ниже 1.22, используйте способ 1.

## Установка бота

```bash
git clone https://github.com/Sp0nge-bob/backupscript.git /opt/backup-bot
cd /opt/backup-bot

go build -ldflags="-s -w" -o backup-bot ./cmd/backup-bot

cp config.yaml.example config.yaml
cp .env.example .env
```

### Настройка `.env`

```env
TELEGRAM_BOT_TOKEN=123456789:ABCdefGHIjklMNOpqrsTUVwxyz
ALLOWED_USER_IDS=123456789
CONFIG_PATH=/opt/backup-bot/config.yaml
BACKUP_TMP_DIR=/tmp/backup-bot
```

- Токен — от [@BotFather](https://t.me/BotFather)
- User ID — от [@userinfobot](https://t.me/userinfobot), можно несколько через запятую

```bash
chmod 600 .env
```

### Настройка `config.yaml`

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
  cron: "0 3 * * *"        # каждый день в 03:00
  notify_chat_id: null     # null = первый ID из ALLOWED_USER_IDS
```

Добавляйте и убирайте пути в `paths` по необходимости.

## Запуск через systemd

```bash
cp backup-bot.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now backup-bot
systemctl status backup-bot
```

Логи:

```bash
journalctl -u backup-bot -f
```

## Сборка

```bash
make build
```

Бинарник появится как `./backup-bot`.

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