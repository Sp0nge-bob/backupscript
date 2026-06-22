#!/usr/bin/env bash
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
	echo "Запустите от root: sudo -i"
	exit 1
fi

echo "=== backupscript: SSH-доступ для master ==="
echo ""
echo "На master: cat /root/.ssh/backup_nodes.pub"
echo "Вставьте одну строку (ssh-ed25519 ... или ssh-rsa ...)."
echo ""
read -r -p "Публичный ключ master: " MASTER_PUBKEY

if [ -z "$MASTER_PUBKEY" ]; then
	echo "Ключ не указан."
	exit 1
fi

case "$MASTER_PUBKEY" in
ssh-ed25519* | ssh-rsa*) ;;
*)
	echo "Неверный формат. Нужна целая строка из .pub файла."
	exit 1
	;;
esac

echo ""
echo "IP master (Enter — разрешить с любого IP):"
read -r -p "IP master: " MASTER_IP

mkdir -p /root/.ssh
chmod 700 /root/.ssh
touch /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys

if [ -n "$MASTER_IP" ]; then
	ENTRY="from=\"${MASTER_IP}\" ${MASTER_PUBKEY}"
else
	ENTRY="$MASTER_PUBKEY"
fi

KEY_SIG="$(echo "$MASTER_PUBKEY" | awk '{print $1" "$2}')"
if grep -qF "$KEY_SIG" /root/.ssh/authorized_keys 2>/dev/null; then
	echo "Ключ master уже есть в authorized_keys."
else
	echo "$ENTRY" >> /root/.ssh/authorized_keys
	echo "Ключ master добавлен."
fi

echo ""
echo "Готово."