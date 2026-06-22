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
- Go 1.22+ (только для сборки)
- Telegram-бот ([@BotFather](https://t.me/BotFather))
- Доступ к файлам бекапа (часто нужен `root` для `/etc/nginx`, `/etc/x-ui`)

## Установка

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