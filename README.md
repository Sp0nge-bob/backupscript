# backupscript

Лёгкий Telegram-бот для Linux: собирает **один zip-архив** из локальных файлов master-сервера и удалённых **нод**, отправляет в Telegram по команде или по расписанию.

Подходит для бекапа nginx, 3x-ui и любых других путей.

## Возможности

- Один архив из нескольких серверов (master + ноды)
- Два режима нод: **SSH pull** (master забирает сам) и **agent push** (нода сама отправляет)
- Команды в Telegram: бекап, пути, ноды, расписание
- Whitelist user ID
- Safe-copy SQLite (`.db`)
- Лимит 50 МБ Telegram
- ~8–15 МБ RAM в простое

## Команды бота

| Команда | Описание |
|---------|----------|
| `/backup` | Создать архив и отправить |
| `/paths` | Пути **master**: add / remove / list |
| `/nodes` | Удалённые ноды: list / status / ping / paths |
| `/schedule` | Интервал автобекапа: `30m`, `6h`, `7d` |
| `/list` | Все настройки |
| `/status` | Последний бекап |
| `/help` | Справка |

### Пути master

```
/paths list
/paths add /etc/nginx/conf.d/
/paths remove /etc/foo
```

### Расписание

```
/schedule          — текущий интервал
/schedule 6h       — каждые 6 часов
/schedule 7d       — каждые 7 дней
/schedule off      — выключить
/schedule on       — включить
```

Формат: `30m`, `6h`, `7d`, `1w`. Минимум `1m`.

### Ноды

```
/nodes list
/nodes status nl2
/nodes ping nl3
/nodes paths list nl2
/nodes paths add nl2 /etc/nginx/conf.d/
/nodes paths remove nl2 /etc/foo
```

Добавление новой ноды (host, token, режим) — в `config.yaml`. Пути на ноде — через бот.

## Структура архива

```
local/etc/nginx/nginx.conf         # master
nl2/etc/nginx/conf.d/site.conf     # SSH-нода
nl3/etc/x-ui/x-ui.db               # agent-нода
```

## Требования

- Linux (VPS), root
- Telegram-бот ([@BotFather](https://t.me/BotFather))
- User ID ([@userinfobot](https://t.me/userinfobot))
- Go 1.22+ (только для сборки)

---

## Установка master (главный сервер с ботом)

1. Замените `ВАШ_ТОКЕН` и `ВАШ_USER_ID`.
2. Скопируйте **весь блок** и вставьте в терминал.

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

---

## Настройка config.yaml (master)

```bash
nano /opt/backup-bot/config.yaml
systemctl restart backup-bot
```

Пример с нодами:

```yaml
backup:
  name: "server-backup"
  paths:
    - /etc/nginx/nginx.conf
    - /etc/nginx/conf.d/
    - /etc/x-ui/x-ui.db
  exclude:
    - "*.log"
    - "*.sock"
    - "*.save"

schedule:
  enabled: true
  interval: "24h"
  notify_chat_id: null

agent:
  listen: "0.0.0.0:9876"
  max_staging_age: "2h"
  sync_timeout: "3m"

nodes:
  - name: "nl2"
    mode: "ssh"
    host: "203.0.113.10"
    port: 22
    user: "root"
    key_file: "/root/.ssh/backup_nodes"
    paths:
      - /etc/nginx/conf.d/
      - /etc/x-ui/x-ui.db

  - name: "nl3"
    mode: "agent"
    token: "случайный-секрет-на-ноду"
    paths:
      - /etc/nginx/conf.d/
      - /etc/x-ui/x-ui.db
```

| Секция | Описание |
|--------|----------|
| `backup.paths` | Файлы на **master** (префикс `local/` в zip) |
| `agent.listen` | HTTP API для agent-нод (порт 9876) |
| `agent.max_staging_age` | Предупреждение, если агент давно не загружал |
| `nodes` | Удалённые серверы |

---

## Нода: режим SSH (master сам забирает файлы)

Подходит, если master может подключиться к ноде по SSH.

### 1. Создайте ключ на master

```bash
ssh-keygen -t ed25519 -f /root/.ssh/backup_nodes -N ""
cat /root/.ssh/backup_nodes.pub
```

### 2. Добавьте ключ на удалённую ноду

На **удалённом VPS**:

```bash
echo 'ssh-ed25519 AAAA... root@master' >> /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
```

Рекомендуется ограничить доступ только с IP master:

```bash
echo 'from="IP_MASTER" ssh-ed25519 AAAA... backup' >> /root/.ssh/authorized_keys
```

### 3. Проверьте подключение с master

```bash
ssh -i /root/.ssh/backup_nodes root@203.0.113.10 "ls /etc/nginx/conf.d/"
```

### 4. Добавьте ноду в config.yaml

```yaml
nodes:
  - name: "nl2"
    mode: "ssh"
    host: "203.0.113.10"
    port: 22
    user: "root"
    key_file: "/root/.ssh/backup_nodes"
    paths:
      - /etc/nginx/conf.d/
```

### 5. Проверьте в боте

```
/nodes list
/nodes status nl2
/backup
```

---

## Нода: режим Agent (нода сама отправляет на master)

Подходит, если нода за NAT или не хотите открывать SSH.

### 1. На master: откройте порт для агентов

```bash
ufw allow from IP_НОДЫ to any port 9876 proto tcp
# или для нескольких нод:
ufw allow 9876/tcp
ufw reload
```

Убедитесь, что в `config.yaml` есть agent-нода с уникальным `token`:

```yaml
nodes:
  - name: "nl3"
    mode: "agent"
    token: "a8f3c91e2b7d4f6e8c0a1b2c3d4e5f6"
    paths:
      - /etc/nginx/conf.d/
      - /etc/x-ui/x-ui.db
```

`token` на master и на ноде должен совпадать.

### 2. На удалённой ноде: установите агент

Скопируйте и вставьте **на удалённом VPS**:

```bash
set -e
INSTALL_DIR="/opt/backup-agent"
GO_VERSION="1.22.5"

if ! command -v go >/dev/null 2>&1; then
  case "$(uname -m)" in
    x86_64)  GO_ARCH="amd64" ;;
    aarch64) GO_ARCH="arm64" ;;
    *) echo "Неподдерживаемая архитектура"; exit 1 ;;
  esac
  wget -q "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" -O /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
  export PATH="/usr/local/go/bin:$PATH"
fi

export PATH="/usr/local/go/bin:$PATH"

if [ -d "$INSTALL_DIR/.git" ]; then
  git -C "$INSTALL_DIR" pull
else
  git clone https://github.com/Sp0nge-bob/backupscript.git "$INSTALL_DIR"
fi

cd "$INSTALL_DIR"
go build -ldflags="-s -w" -o backup-agent ./cmd/backup-agent

mkdir -p /etc/backup-agent
if [ ! -f /etc/backup-agent/config.yaml ]; then
  cp agent.yaml.example /etc/backup-agent/config.yaml
fi

cp backup-agent.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable backup-agent

echo "Отредактируйте /etc/backup-agent/config.yaml и запустите: systemctl start backup-agent"
```

### 3. Настройте агент

```bash
nano /etc/backup-agent/config.yaml
```

```yaml
node: "nl3"
master_url: "http://IP_MASTER:9876"
token: "a8f3c91e2b7d4f6e8c0a1b2c3d4e5f6"
paths:
  - /etc/nginx/conf.d/
  - /etc/x-ui/x-ui.db
listen_timeout: "6h"
tmp_dir: "/tmp/backup-agent"
```

```bash
chmod 600 /etc/backup-agent/config.yaml
systemctl start backup-agent
systemctl status backup-agent
journalctl -u backup-agent -f
```

### 4. Проверьте на master

```
/nodes status nl3
/backup
```

Агент держит **одно долгое соединение** с master (до `listen_timeout`, по умолчанию 6h) — это не heartbeat, а ожидание команды. Нагрузка минимальная: нет периодических запросов.

- **`/backup`** — master будит агент → агент упаковывает **актуальные** файлы и загружает → собирается zip
- **`/nodes ping nl3`** — проверка доступности agent/SSH по команде (не автоматически)
- **`/nodes list`** — статус «online» если агент был на связи за последние 6h

---

## Обновление master

```bash
set -e
cd /opt/backup-bot
export PATH="/usr/local/go/bin:$PATH"
git pull
go build -ldflags="-s -w" -o backup-bot ./cmd/backup-bot
systemctl restart backup-bot
systemctl --no-pager status backup-bot
```

## Обновление агента на ноде

```bash
set -e
cd /opt/backup-agent
export PATH="/usr/local/go/bin:$PATH"
git pull
go build -ldflags="-s -w" -o backup-agent ./cmd/backup-agent
systemctl restart backup-agent
```

## Логи

Master:

```bash
journalctl -u backup-bot -f
```

Agent:

```bash
journalctl -u backup-agent -f
```

## Ограничения

- Telegram: максимум **50 МБ** на файл. С несколькими нодами легко превысить — сужайте `paths` или делайте бекап по отдельности.
- Бот не создаёт дампы БД — только упаковывает указанные пути.
- Отсутствующие пути / недоступные ноды — предупреждение, остальное включается в архив.
- Новая нода (host, token, mode) — только через `config.yaml`. Пути на ноде — через `/nodes paths`.

## Структура проекта

```
cmd/backup-bot/       — бот + agent API (master)
cmd/backup-agent/     — агент для удалённых нод
internal/backup/      — сборка zip
internal/remote/      — SSH/SFTP
internal/agent/       — HTTP API и клиент агента
internal/bot/         — команды Telegram
internal/config/      — config.yaml
```

## Лицензия

MIT