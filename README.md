# backupscript

Telegram-бот для Linux: собирает **один zip-архив** из файлов master-сервера и удалённых **нод**, отправляет в Telegram по команде или по расписанию.

Подходит для бекапа nginx, 3x-ui и любых других путей. В простое ~8–15 МБ RAM.

## Быстрый старт

| Шаг | Что делать | Документация |
|-----|------------|--------------|
| 1 | Установить **master** (бот + API агентов) | [Установка master](docs/install-master.md) |
| 2 | Настроить `config.yaml` | [Конфигурация](docs/config.md) |
| 3 | Подключить ноды | [SSH-нода](docs/install-ssh-node.md) или [Agent-нода](docs/install-agent-node.md) |
| 4 | Проверить `/backup` в Telegram | [Команды бота](docs/commands.md) |

## Документация

### Установка

- [Master — главный сервер с ботом](docs/install-master.md)
- [SSH-нода — master сам забирает файлы](docs/install-ssh-node.md)
- [Agent-нода — нода сама отправляет на master](docs/install-agent-node.md)

### Справочник

- [Команды Telegram-бота](docs/commands.md)
- [Конфигурация config.yaml](docs/config.md)
- [Структура zip-архива](docs/archive.md)

### Эксплуатация

- [Обновление, логи, ограничения](docs/operations.md)

## Возможности

- Один архив из нескольких серверов (master + ноды)
- Два режима нод: **SSH pull** и **agent push**
- Управление путями и расписанием из Telegram
- Whitelist user ID, safe-copy SQLite (`.db`)
- Недоступные ноды пропускаются с предупреждением — остальной бекап сохраняется

## Режимы нод

| Режим | Что ставить на удалённой ноде | Когда использовать |
|-------|-------------------------------|--------------------|
| **SSH** | **Ничего** — только ключ в `authorized_keys` | Master подключается к ноде по SSH |
| **Agent** | `backup-agent` (git + systemd) | Нода за NAT, закрытый SSH, push-модель |

## Требования

- Linux (VPS), root
- Telegram-бот ([@BotFather](https://t.me/BotFather))
- User ID ([@userinfobot](https://t.me/userinfobot))
- Go 1.22+ (только для сборки)

## Структура проекта

```
cmd/backup-bot/       — бот + agent API (master)
cmd/backup-agent/     — агент для удалённых нод
internal/backup/      — сборка zip
internal/remote/      — SSH/SFTP
internal/agent/       — HTTP API и клиент агента
internal/bot/         — команды Telegram
internal/config/      — config.yaml
docs/                 — документация
```

## Лицензия

MIT