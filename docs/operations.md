# Эксплуатация

Обновление, логи и ограничения системы.

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

---

## Обновление агента

На **удалённой ноде**:

```bash
set -e
cd /opt/backup-agent
export PATH="/usr/local/go/bin:$PATH"
git pull
go build -ldflags="-s -w" -o backup-agent ./cmd/backup-agent
systemctl restart backup-agent
systemctl --no-pager status backup-agent
```

---

## Логи

**Master:**

```bash
journalctl -u backup-bot -f
```

**Agent:**

```bash
journalctl -u backup-agent -f
```

---

## Ограничения

| Ограничение | Пояснение |
|-------------|-----------|
| **50 МБ** | Максимальный размер файла в Telegram |
| Нет дампов БД | Бот упаковывает только указанные пути, не делает `mysqldump` и т.п. |
| Недоступные ноды | Пропускаются с предупреждением — остальной архив создаётся |
| Новая нода | Host, token, mode — только через `config.yaml` |
| Пути на ноде | Через `/nodes paths` в боте |

---

## Безопасность

- `config.yaml` и `.env` на master — права `600`
- SSH-ключ бекапа — отдельный (`/root/.ssh/backup_nodes`), лучше с `from="IP_MASTER"` в `authorized_keys`
- Token agent-ноды — уникальный на каждую ноду
- Бот отвечает только user ID из whitelist

---

## См. также

- [Установка master](install-master.md)
- [SSH-нода](install-ssh-node.md)
- [Agent-нода](install-agent-node.md)
- [Команды бота](commands.md)