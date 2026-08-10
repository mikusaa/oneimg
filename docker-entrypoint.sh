#!/bin/sh

set -eu

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

validate_id() {
	name="$1"
	value="$2"
	case "$value" in
		''|*[!0-9]*)
			echo "错误: ${name} 必须是非负整数，当前值为 '${value}'" >&2
			exit 1
			;;
	esac
}

validate_id "PUID" "$PUID"
validate_id "PGID" "$PGID"

if [ "$(id -u)" -ne 0 ]; then
	echo "OneImg 已由非 root 用户启动，跳过 PUID/PGID 和目录属主调整"
	exec "$@"
fi

for directory in /app/data /app/uploads; do
	mkdir -p "$directory"
	if ! chown -R "$PUID:$PGID" "$directory"; then
		echo "错误: 无法将 ${directory} 的属主设置为 ${PUID}:${PGID}" >&2
		exit 1
	fi
done

echo "OneImg 将以 UID ${PUID}、GID ${PGID} 运行"
exec su-exec "$PUID:$PGID" "$@"
