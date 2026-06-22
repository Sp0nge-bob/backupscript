# Структура архива

Каждый бекап — один zip-файл:

```
{backup.name}_{дата-время}.zip
```

Пример: `server-backup_2026-06-22_15-30-45.zip`

---

## Префиксы внутри zip

| Префикс | Источник |
|---------|----------|
| `local/` | Файлы с **master** (`backup.paths`) |
| `имя-ноды/` | Удалённая нода (`nodes[].name`) |

Имя папки ноды = поле `name` в `config.yaml`.

---

## Пример с тремя источниками

**Конфиг:**

- master: `/etc/nginx/nginx.conf`, `/etc/x-ui/x-ui.db`
- SSH-нода `nl2`: `/etc/nginx/conf.d/`
- agent-нода `nl3`: `/etc/x-ui/x-ui.db`

**Содержимое zip:**

```
local/etc/nginx/nginx.conf
local/etc/x-ui/x-ui.db
nl2/etc/nginx/conf.d/default.conf
nl2/etc/nginx/conf.d/site.conf
nl3/etc/x-ui/x-ui.db
```

Пути внутри архива — **полные пути с сервера** (без ведущего `/`), с префиксом источника.

---

## Порядок сборки

1. Все пути из `backup.paths` → `local/`
2. Ноды **по порядку** из `nodes:` в config.yaml

Если нода недоступна — её папки в архиве **не будет**. В Telegram в подписи к файлу появятся предупреждения.

---

## Восстановление файла

Уберите префикс и положите файл на нужный сервер:

| Путь в zip | Куда восстановить |
|------------|-------------------|
| `local/etc/nginx/nginx.conf` | master → `/etc/nginx/nginx.conf` |
| `nl2/etc/nginx/conf.d/site.conf` | нода nl2 → `/etc/nginx/conf.d/site.conf` |
| `nl3/etc/x-ui/x-ui.db` | нода nl3 → `/etc/x-ui/x-ui.db` |

---

## SQLite

Файлы `.db`, `.sqlite`, `.sqlite3` копируются через safe-copy (копия во временную папку), чтобы не повредить базу при чтении.

---

## Лимит размера

Telegram принимает файлы до **50 МБ**. С несколькими нодами лимит достигается быстро — сужайте `paths` или бекапьте ноды по отдельности.

---

## См. также

- [Команды бота](commands.md)
- [Конфигурация](config.md)