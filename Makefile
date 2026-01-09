# rfc-mcp Makefile

# デフォルトターゲット
.PHONY: build
build:
	go build -o rfc-mcp cmd/rfc-mcp/*.go

# インストールターゲット
.PHONY: install
install: build
	# バイナリを/usr/local/binにインストール
	install -m 755 rfc-mcp /usr/local/bin/rfc-mcp
	
	# 設定ファイルを/usr/local/etcにインストール
	install -m 644 cmd/rfc-mcp/config.yaml /usr/local/etc/rfc-mcp.yaml
	
	# systemdサービスファイルを/etc/systemd/systemにインストール
	install -m 644 misc/rfc-mcp.service /etc/systemd/system/rfc-mcp.service
	
	# systemdデーモンをリロード
	systemctl daemon-reload

	# systemdサービスを有効化
	systemctl enable rfc-mcp.service

	# systemdサービスを再起動
	systemctl restart rfc-mcp.service 
	
clean:
	rm -f rfc-mcp

uninstall:
	# systemdサービスを停止
	systemctl stop rfc-mcp.service || true
	
	# systemdサービスを無効化
	systemctl disable rfc-mcp.service || true
	
	# systemdサービスファイルを削除
	rm -f /etc/systemd/system/rfc-mcp.service
	
	# systemdデーモンをリロード
	systemctl daemon-reload
	
	# バイナリを削除
	rm -f /usr/local/bin/rfc-mcp
	
	# 設定ファイルを削除
	rm -f /usr/local/etc/rfc-mcp.yaml
