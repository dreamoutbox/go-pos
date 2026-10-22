#!/usr/bin/env sh
set -eux

app_dir=/app/go-pos/
asset_url=https://github.com/dreamoutbox/go-pos/releases/latest/download/go-pos-linux-amd64.tar.gz
service_file=/etc/systemd/system/go-pos.service

if [ "$(id -u)" -ne 0 ]; then
	echo "error: run this setup script as root (for example, with sudo)" >&2
	exit 1
fi

if ! command -v systemctl >/dev/null 2>&1; then
	echo "error: systemctl is not available; a systemd host is required" >&2
	exit 1
fi

service_user=${SUDO_USER:-$(id -un)}
if ! id "$service_user" >/dev/null 2>&1; then
	echo "error: service user '$service_user' does not exist" >&2
	exit 1
fi
service_group=$(id -gn "$service_user")

download_dir=$(mktemp -d)
trap 'rm -rf "$download_dir"' EXIT HUP INT TERM

mkdir -p "$app_dir"
curl -fL "$asset_url" -o "$download_dir/go-pos-linux-amd64.tar.gz"

# clean existing files in the app directory before extracting the new version
rm -rf "$app_dir/*"

tar -xzf "$download_dir/go-pos-linux-amd64.tar.gz" -C "$app_dir"

chmod +x "$app_dir/go-pos-linux-amd64"
rm -rf "$download_dir"

mkdir -p "$app_dir/data/product"
chown -R "$service_user:$service_group" "$app_dir/data"

printf '%s\n' \
	'[Unit]' \
	'Description=Go POS' \
	'Wants=network-online.target' \
	'After=network-online.target' \
	'' \
	'[Service]' \
	'Type=simple' \
	"User=$service_user" \
	"Group=$service_group" \
	"WorkingDirectory=$app_dir" \
	'Environment=MOCK_DATA=1' \
	"Environment=DB_PATH=$app_dir/data/sqlite.db" \
	"Environment=UPLOAD_DIR=$app_dir/data/product" \
	"ExecStart=$app_dir/go-pos-linux-amd64" \
	'Restart=always' \
	'RestartSec=5' \
	'' \
	'[Install]' \
	'WantedBy=multi-user.target' \
	> "$service_file"

systemctl daemon-reload
systemctl enable go-pos.service

if systemctl is-active --quiet go-pos.service; then
	systemctl restart go-pos.service
else
	systemctl start go-pos.service
fi
