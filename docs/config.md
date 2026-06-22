# Конфигурация

Файл на master: `/opt/backup-bot/config.yaml`

После изменений:

```bash
systemctl restart backup-bot
```

---

## Полный пример

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

---

## Секция `backup`

| Поле | Описание |
|------|----------|
| `name` | Префикс имени zip-файла (`server-backup_2026-06-22_15-30-45.zip`) |
| `paths` | Файлы и папки на **master** (в zip под `local/`) |
| `exclude` | Маски имён файлов для пропуска (`*.log` и т.д.) |

Пути master можно менять из бота: `/paths add`, `/paths remove`.

---

## Секция `schedule`

| Поле | Описание |
|------|----------|
| `enabled` | `true` / `false` — автобекап |
| `interval` | Интервал: `30m`, `6h`, `24h`, `7d` |
| `notify_chat_id` | Chat ID для уведомлений (или `null` — в чат инициатора) |

Управление из бота: `/schedule`.

---

## Секция `agent`

Нужна, если есть хотя бы одна нода с `mode: agent`.

| Поле | Описание |
|------|----------|
| `listen` | Адрес HTTP API на master (`0.0.0.0:9876`) |
| `max_staging_age` | Предупреждение, если данные агента старше этого возраста |
| `sync_timeout` | Сколько ждать свежую загрузку от агента при `/backup` |

---

## Секция `nodes`

Список удалённых серверов. Каждая нода — отдельная папка в zip.

### Общие поля

| Поле | Описание |
|------|----------|
| `name` | Уникальное имя (префикс в zip: `nl2/`, `nl3/`) |
| `mode` | `ssh` или `agent` |
| `paths` | Что бекапить на этой ноде |

### SSH-нода (`mode: ssh`)

| Поле | Описание |
|------|----------|
| `host` | IP или домен |
| `port` | SSH-порт (по умолчанию 22) |
| `user` | Пользователь SSH |
| `key_file` | Путь к приватному ключу **на master** |

Установка: [SSH-нода](install-ssh-node.md)

### Agent-нода (`mode: agent`)

| Поле | Описание |
|------|----------|
| `token` | Секрет (должен совпадать с `/etc/backup-agent/config.yaml` на ноде) |

Установка: [Agent-нода](install-agent-node.md)

---

## Переменные окружения (master)

Файл `/opt/backup-bot/.env`:

| Переменная | Описание |
|------------|----------|
| `TELEGRAM_BOT_TOKEN` | Токен бота |
| `ALLOWED_USER_IDS` | Telegram user ID через запятую |
| `CONFIG_PATH` | Путь к `config.yaml` |
| `BACKUP_TMP_DIR` | Временная папка для zip |

---

## Конфиг агента

Файл на ноде: `/etc/backup-agent/config.yaml`

См. [Agent-нода](install-agent-node.md#шаг-3-настройка-агента).