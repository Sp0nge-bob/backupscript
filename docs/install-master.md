# Установка master

Master — главный сервер с Telegram-ботом и HTTP API для agent-нод (порт 9876).

## Перед установкой

1. Создайте бота у [@BotFather](https://t.me/BotFather) и скопируйте токен.
2. Узнайте свой Telegram user ID у [@userinfobot](https://t.me/userinfobot).
3. Подключитесь к VPS по SSH под **root**.

## Установка

Скопируйте **одну** команду на master (ничего заранее править не нужно):

```bash
curl -fsSL -o /tmp/install-master.sh https://raw.githubusercontent.com/Sp0nge-bob/backupscript/main/scripts/install-master.sh && bash /tmp/install-master.sh
```

Скрипт спросит:

1. `Telegram bot token:` — вставьте токен от BotFather (символы не отображаются — это нормально)
2. `Telegram user ID:` — ваш ID; несколько ID через запятую: `123,456`

> Не используйте `curl | bash` — ломает ввод. Только `curl -o файл && bash файл`.

### Без вопросов (аргументы)

```bash
curl -fsSL -o /tmp/install-master.sh https://raw.githubusercontent.com/Sp0nge-bob/backupscript/main/scripts/install-master.sh
bash /tmp/install-master.sh '123456789:AA...' 'YOUR_USER_ID'
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