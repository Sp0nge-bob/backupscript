# Agent-нода

В режиме **agent** на удалённом VPS ставится `backup-agent`. Нода **сама** подключается к master и по команде `/backup` отправляет свежие файлы.

Подходит, если нода за NAT, SSH с master закрыт, или удобнее push-модель.

---

## Схема

```
удалённый VPS (backup-agent) ──HTTP──► master:9876
         long-poll ожидание          будит → upload → zip
```

Файлы в архиве — под префиксом `имя-ноды/` — см. [Структура архива](archive.md).

---

## Шаг 1. Master: порт и нода в config.yaml

На **master** откройте порт API агентов:

```bash
# Одна нода — ограничьте по IP
ufw allow from IP_НОДЫ to any port 9876 proto tcp

# Несколько нод — откройте порт (защита по token)
ufw allow 9876/tcp
ufw reload
```

Добавьте agent-ноду в `/opt/backup-bot/config.yaml`:

```yaml
agent:
  listen: "0.0.0.0:9876"
  max_staging_age: "2h"
  sync_timeout: "3m"

nodes:
  - name: "nl3"
    mode: "agent"
    token: "a8f3c91e2b7d4f6e8c0a1b2c3d4e5f6"
    paths:
      - /etc/nginx/conf.d/
      - /etc/x-ui/x-ui.db
```

Сгенерируйте уникальный `token` на каждую ноду. Перезапустите master:

```bash
systemctl restart backup-bot
```

---

## Шаг 2. Установка агента на ноде

Скопируйте и вставьте **на удалённом VPS**:

```bash
set -e

INSTALL_DIR="/opt/backup-agent"
GO_VERSION="1.22.5"

if [ "$(id -u)" -ne 0 ]; then
  echo "Запустите от root: sudo -i"
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
go build -ldflags="-s -w" -o backup-agent ./cmd/backup-agent

mkdir -p /etc/backup-agent
if [ ! -f /etc/backup-agent/config.yaml ]; then
  cp agent.yaml.example /etc/backup-agent/config.yaml
fi

cp backup-agent.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable backup-agent

echo "Готово. Отредактируйте /etc/backup-agent/config.yaml и запустите: systemctl start backup-agent"
```

---

## Шаг 3. Настройка агента

Откройте конфиг в терминале:

```bash
nano /etc/backup-agent/config.yaml
```

Вставьте в файл **только YAML**:

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

| Поле | Должно совпадать с |
|------|---------------------|
| `node` | `name` ноды в `config.yaml` на master |
| `token` | `token` той же ноды на master |
| `master_url` | IP/домен master и порт `9876` |

Сохраните файл (`Ctrl+O`, Enter, `Ctrl+X`). Затем в терминале:

```bash
chmod 600 /etc/backup-agent/config.yaml
systemctl start backup-agent
systemctl status backup-agent
journalctl -u backup-agent -f
```

---

## Шаг 4. Проверка

В Telegram на master:

```
/nodes status nl3
/nodes ping nl3
/backup
```

---

## Как это работает

- Агент держит **одно долгое HTTP-соединение** с master (до `listen_timeout`, по умолчанию 6h). Это не heartbeat — просто ожидание команды. Нагрузка минимальная.
- **`/backup`** — master будит агент → агент упаковывает **актуальные** файлы и загружает → собирается zip.
- **`/nodes ping nl3`** — проверка доступности по команде (не автоматически).
- **`/nodes list`** — «online», если агент был на связи за последние 6h.

Если агент недоступен при бекапе — нода **пропускается** с предупреждением в подписи архива.

---

## См. также

- [Установка master](install-master.md)
- [SSH-нода](install-ssh-node.md) — альтернатива без агента
- [Конфигурация](config.md)
- [Обновление агента](operations.md#обновление-агента)