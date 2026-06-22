# SSH-нода

В режиме **SSH** master сам подключается к удалённому VPS и забирает файлы по SFTP.

Подходит, если master может достучаться до ноды по SSH.

---

## SSH или Agent?

| | SSH-нода | Agent-нода |
|---|----------|------------|
| Софт на удалённом VPS | **Ничего не ставится** | `backup-agent` + systemd |
| Софт на master | `backup-bot` | `backup-bot` + порт 9876 |
| Как передаются файлы | Master забирает по SSH/SFTP | Нода сама отправляет на master |
| Гайд | этот файл | [install-agent-node.md](install-agent-node.md) |

> **backup-agent на SSH-ноде не нужен и не ставится.** Если нужен агент — используйте режим `agent`, не `ssh`.

---

## Схема

```
master ──SSH/SFTP──► удалённый VPS (nl2)
         забирает paths из config.yaml
```

Файлы в архиве попадают под префикс `имя-ноды/` — см. [Структура архива](archive.md).

---

## Шаг 1. Ключ на master

Скопируйте и вставьте **на master** (`/opt/backup-bot`):

```bash
set -e

KEY_FILE="/root/.ssh/backup_nodes"

if [ "$(id -u)" -ne 0 ]; then
  echo "Запустите от root: sudo -i"
  exit 1
fi

mkdir -p /root/.ssh
chmod 700 /root/.ssh

if [ ! -f "$KEY_FILE" ]; then
  ssh-keygen -t ed25519 -f "$KEY_FILE" -N "" -C "backupscript"
  echo "Создан новый ключ: $KEY_FILE"
else
  echo "Ключ уже существует: $KEY_FILE"
fi

chmod 600 "$KEY_FILE"
chmod 644 "${KEY_FILE}.pub"

echo ""
echo "========== Публичный ключ (скопируйте на ноду) =========="
cat "${KEY_FILE}.pub"
echo "=========================================================="
```

На удалённой ноде понадобится эта строка — скрипт на шаге 2 спросит её отдельно.

Узнать IP master (для ограничения доступа на ноде):

```bash
curl -4 -s ifconfig.me; echo
```

---

## Шаг 2. Установка на удалённой ноде

На SSH-ноде нет `git clone` и сборки — только **один раз** добавить ключ master в `authorized_keys`.

> **Не вставляйте многострочный скрипт в SSH построчно** — терминал часто ломает вставку. Используйте **одну команду** ниже.

### Способ A (рекомендуется): одна строка

На **удалённом VPS** под root — скопируйте **одну** команду и Enter:

```bash
curl -fsSL https://raw.githubusercontent.com/Sp0nge-bob/backupscript/main/scripts/ssh-node-authorize.sh | bash
```

Если нет `curl`:

```bash
wget -qO- https://raw.githubusercontent.com/Sp0nge-bob/backupscript/main/scripts/ssh-node-authorize.sh | bash
```

Дальше скрипт спросит:

1. `Публичный ключ master:` — вставьте строку с master (`cat /root/.ssh/backup_nodes.pub`)
2. `IP master:` — IP master или Enter (без ограничения)

### Способ B: скачать файл и запустить

```bash
curl -fsSL -o /tmp/ssh-node-authorize.sh https://raw.githubusercontent.com/Sp0nge-bob/backupscript/main/scripts/ssh-node-authorize.sh
bash /tmp/ssh-node-authorize.sh
rm -f /tmp/ssh-node-authorize.sh
```

### Проверка на ноде

```bash
grep backupscript /root/.ssh/authorized_keys
```

Должна быть строка с вашим `ssh-ed25519 ...`.

> **Рекомендация:** укажите IP master при запросе — тогда ключ сработает только с этого сервера.

---

## Шаг 3. Проверка с master

На **master** (подставьте IP, порт и пользователя ноды):

```bash
# Стандартный порт 22:
ssh -i /root/.ssh/backup_nodes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new \
  root@203.0.113.10 "ls /etc/nginx/conf.d/"

# Нестандартный порт (например 2222) — обязательно -p:
ssh -p 2222 -i /root/.ssh/backup_nodes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new \
  root@203.0.113.10 "ls /etc/nginx/conf.d/"
```

Если команда выполнилась без пароля — SSH настроен.  
Если **висит без ответа** — проверьте порт (`-p`) и firewall на ноде.

---

## Шаг 4. Добавьте ноду в config.yaml

**4a.** На master откройте конфиг в терминале:

```bash
nano /opt/backup-bot/config.yaml
```

**4b.** В открывшемся редакторе вставьте **только YAML** (не команды shell):

```yaml
nodes:
  - name: "nl2"
    mode: "ssh"
    host: "203.0.113.10"
    port: 2222          # ваш SSH-порт; если 22 — можно не менять
    user: "root"
    key_file: "/root/.ssh/backup_nodes"
    paths:
      - /etc/nginx/conf.d/
      - /etc/x-ui/x-ui.db
```

> В `config.yaml` не должно быть строк вроде `systemctl`, `nano`, `bash` — только YAML. Иначе бот не запустится.

Сохраните в nano: `Ctrl+O` → Enter → `Ctrl+X`.

**4c.** Уже **в терминале** (не в файле) перезапустите бот:

```bash
systemctl restart backup-bot
```

| Поле | Описание |
|------|----------|
| `name` | Имя папки в zip (`nl2/...`) |
| `host` | IP или домен ноды |
| `port` | SSH-порт ноды (**обязательно**, если не 22) |
| `user` | Пользователь SSH |
| `key_file` | Приватный ключ на master |
| `paths` | Что забирать с ноды |

Пути на ноде можно менять из Telegram: `/nodes paths add nl2 /путь`

Полный справочник: [Конфигурация](config.md)

---

## Шаг 5. Проверка в боте

```
/nodes list
/nodes status nl2
/nodes ping nl2
/backup
```

Если нода недоступна при бекапе — она **пропускается**, остальные источники попадают в архив. В подписи к файлу будут предупреждения.

---

## Частые проблемы

| Симптом | Решение |
|---------|---------|
| Команда **висит** без ошибки | Неверный порт — добавьте `-p` в ssh и `port:` в config.yaml; проверьте firewall |
| `Permission denied (publickey)` | Проверьте `authorized_keys` на ноде, права `600` на файл и `700` на `.ssh` |
| `Connection refused` | Откройте порт SSH в firewall ноды, проверьте `host` и `port` |
| `No such file` в предупреждениях | Путь не существует на ноде — добавьте или уберите через `/nodes paths` |
| Нода пропущена в архиве | Смотрите предупреждения в Telegram; проверьте `/nodes ping` |
| Бот не стартует после правки config | В файле случайно оказалась shell-команда — удалите строки `systemctl`, `nano` и т.п. |
| Скрипт на ноде «развалился» при вставке | Не копируйте многострочный bash в SSH — используйте `curl ... \| bash` (шаг 2, способ A) |

---

## См. также

- [Установка master](install-master.md)
- [Agent-нода](install-agent-node.md) — альтернатива, если SSH с master недоступен
- [Команды бота](commands.md)