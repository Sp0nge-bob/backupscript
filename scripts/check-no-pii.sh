#!/usr/bin/env bash
# Fail if docs/examples contain non-RFC5737 IP addresses or known hostnames.
set -eo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

scan_files() {
	find docs README.md config.yaml.example agent.yaml.example scripts \
		-type f \( -name '*.md' -o -name '*.example' -o -name '*.sh' \) \
		! -name 'check-no-pii.sh' 2>/dev/null
}

allowed_ip() {
	local ip="$1"
	case "$ip" in
	0.0.0.0 | 127.0.0.1) return 0 ;;
	192.0.2.* | 198.51.100.* | 203.0.113.*) return 0 ;;
	esac
	return 1
}

errors=0

while IFS= read -r file; do
	[ -n "$file" ] || continue
	while IFS= read -r ip; do
		[ -n "$ip" ] || continue
		if ! allowed_ip "$ip"; then
			echo "FORBIDDEN IP $ip in $file"
			errors=$((errors + 1))
		fi
	done < <(grep -oE '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' "$file" 2>/dev/null | sort -u)

	for pattern in nlsrv rusrv; do
		if grep -q "$pattern" "$file" 2>/dev/null; then
			echo "FORBIDDEN hostname $pattern in $file"
			errors=$((errors + 1))
		fi
	done
done < <(scan_files)

if [ "$errors" -gt 0 ]; then
	echo ""
	echo "$errors problem(s). Use RFC 5737 IPs (203.0.113.x) or placeholders (IP_НОДЫ)."
	exit 1
fi

echo "OK: no personal IPs/hostnames in docs/examples."