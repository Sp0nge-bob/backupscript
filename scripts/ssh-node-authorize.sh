#!/usr/bin/env bash
set -eo pipefail

SCRIPT_URL="https://raw.githubusercontent.com/Sp0nge-bob/backupscript/main/scripts/ssh-node-authorize.sh"

if [ "$(id -u)" -ne 0 ]; then
	echo "Запустите от root: sudo -i"
	exit 1
fi

read_prompt() {
	local prompt="$1"
	local __var="$2"
	local value=""

	if [ ! -r /dev/tty ]; then
		echo "Нет TTY. Запустите так:"
		echo "  curl -fsSL -o /tmp/ssh-node-authorize.sh ${SCRIPT_URL}"
		echo "  bash /tmp/ssh-node-authorize.sh"
		echo ""
		echo "Или с ключом в аргументе:"
		echo "  bash /tmp/ssh-node-authorize.sh 'ssh-ed25519 AAAA...' IP_MASTER"
		exit 1
	fi

	exec 3</dev/tty
	IFS= read -r -u 3 -p "$prompt" value || true
	exec 3<&-

	printf -v "$__var" '%s' "$value"
}

validate_pubkey() {
	case "$1" in
	ssh-ed25519* | ssh-rsa*) return 0 ;;
	*)
		echo "Неверный формат. Нужна целая строка из .pub файла."
		return 1
		;;
	esac
}

add_key() {
	local pubkey="$1"
	local master_ip="$2"
	local entry=""

	mkdir -p /root/.ssh
	chmod 700 /root/.ssh
	touch /root/.ssh/authorized_keys
	chmod 600 /root/.ssh/authorized_keys

	if [ -n "$master_ip" ]; then
		entry="from=\"${master_ip}\" ${pubkey}"
	else
		entry="$pubkey"
	fi

	local key_sig
	key_sig="$(echo "$pubkey" | awk '{print $1" "$2}')"
	if grep -qF "$key_sig" /root/.ssh/authorized_keys 2>/dev/null; then
		echo "Ключ master уже есть в authorized_keys."
	else
		echo "$entry" >> /root/.ssh/authorized_keys
		echo "Ключ master добавлен."
	fi
}

MASTER_PUBKEY="${1:-}"
MASTER_IP="${2:-}"

if [ -n "$MASTER_PUBKEY" ]; then
	validate_pubkey "$MASTER_PUBKEY"
	add_key "$MASTER_PUBKEY" "$MASTER_IP"
	echo "Готово."
	exit 0
fi

echo "=== backupscript: SSH-доступ для master ==="
echo ""
echo "На master: cat /root/.ssh/backup_nodes.pub"
echo ""

read_prompt "Публичный ключ master: " MASTER_PUBKEY

if [ -z "$MASTER_PUBKEY" ]; then
	echo "Ключ не указан."
	exit 1
fi

validate_pubkey "$MASTER_PUBKEY"

echo ""
read_prompt "IP master (Enter — любой IP): " MASTER_IP

add_key "$MASTER_PUBKEY" "$MASTER_IP"
echo "Готово."