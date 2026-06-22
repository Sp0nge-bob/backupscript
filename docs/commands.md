# Команды бота

Бот отвечает только пользователям из `ALLOWED_USER_IDS` (whitelist).

## Основные

| Команда | Описание |
|---------|----------|
| `/backup` | Создать архив и отправить в чат |
| `/paths` | Пути **master**: add / remove / list |
| `/nodes` | Удалённые ноды: list / status / ping / paths |
| `/schedule` | Интервал автобекапа: `30m`, `6h`, `7d` |
| `/list` | Все настройки (расписание, пути, ноды) |
| `/status` | Результат последнего бекапа |
| `/help` | Краткая справка |

---

## Пути master

```
/paths list
/paths add /etc/nginx/conf.d/
/paths remove /etc/foo
```

Изменения сохраняются в `config.yaml` на master.

---

## Расписание

```
/schedule              — текущий интервал
/schedule 6h           — каждые 6 часов
/schedule 7d           — каждые 7 дней
/schedule off          — выключить
/schedule on           — включить
```

**Формат интервала:** `30m`, `6h`, `7d`, `1w`. Минимум `1m`.

---

## Ноды

```
/nodes list
/nodes status nl2
/nodes ping nl3
/nodes paths list nl2
/nodes paths add nl2 /etc/nginx/conf.d/
/nodes paths remove nl2 /etc/foo
```

| Подкоманда | Что делает |
|------------|------------|
| `list` | Все ноды и режим (ssh / agent) |
| `status <имя>` | Детали ноды, пути, доступность |
| `ping <имя>` | Проверка SSH или agent прямо сейчас |
| `paths list/add/remove` | Управление путями на ноде |

> Добавление новой ноды (host, token, mode) — только в `config.yaml`. Пути на ноде — через `/nodes paths`.

---

## Поведение при ошибках

- Отсутствующий локальный путь → предупреждение, остальное в архиве.
- Недоступная нода (SSH / agent) → предупреждение, нода пропускается, бекап продолжается.
- Архив > 50 МБ → ошибка, файл не отправляется.

Предупреждения отображаются в подписи к zip в Telegram.

---

## См. также

- [Структура архива](archive.md)
- [Конфигурация](config.md)