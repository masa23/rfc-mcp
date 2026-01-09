# RFC MCP Server

このプロジェクトは、Model Context Protocol (MCP) に準拠したRFC検索サーバーです。Roo CodeなどのMCP対応エージェントから使用できます。  
雑にAIを活用して作っているため、動作の保証はありません。自己責任でご利用ください。

## 機能

1. **rfc_search**: RFC一覧からキーワード検索を行う
2. **rfc_fetch**: 指定されたRFCの全文を取得する
3. **rfc_extract**: 指定されたRFCから特定のセクションを抽出する

#### stdioモードの場合

1. Roo Codeの設定画面を開く
2. MCPサーバーの設定を選択
3. 新しいサーバーを追加
4. コマンドに `go run /path/to/rfc-mcp/cmd/rfc-mcp/main.go -mode stdio` を指定
   （実際のパスに置き換えてください）
5. サーバーを有効化

#### HTTPモードの場合

1. Roo Codeの設定画面を開く
2. MCPサーバーの設定を選択
3. 新しいサーバーを追加
4. サーバータイプを「HTTP」に選択
5. URLに `http://localhost:7991/mcp` を指定
   （ポート番号やホスト名、パスは設定に応じて変更してください）
6. サーバーを有効化

### 3. ツールの使用方法

#### rfc_search

RFC一覧からキーワード検索を行います。

パラメータ:
- `query`: 検索キーワード（必須）
- `limit`: 最大結果数（任意、デフォルト: 10）

使用例:
```json
{
  "query": "http security",
  "limit": 5
}
```

#### rfc_fetch

指定されたRFCの全文を取得します。

パラメータ:
- `number`: RFC番号（必須）
- `maxBytes`: 最大バイト数（任意、デフォルト: 10000）

使用例:
```json
{
  "number": 2616,
  "maxBytes": 5000
}
```

#### rfc_extract

指定されたRFCから特定のセクションを抽出します。

パラメータ:
- `number`: RFC番号（必須）
- `section`: セクション指定（必須）

使用例:
```json
{
  "number": 2616,
  "section": "Section 3.1"
}
```

## 開発

### ビルド

```bash
go build -o rfc-mcp cmd/rfc-mcp/main.go
```

### テスト

```bash
go test ./...
```

## 設定

`config.yaml` ファイルで設定を変更できます：

```yaml
# RFC MCP Server Configuration

# Cache settings
cache:
  dir: .cache/rfc-mcp
  ttl: 1h

# Server settings
server:
  mode: http
  host: localhost
  port: 7991
  path: /mcp
```

設定項目の説明：
 - `cache.dir`: キャッシュディレクトリ
 - `cache.ttl`: キャッシュの有効期間（TTL）

- `server.mode`: サーバーの動作モード（`stdio` または `http`）
- `server.port`: HTTPモードで使用するポート番号
- `server.host`: HTTPモードでバインドするホストアドレス
- `server.path`: HTTPモードでのMCPエンドポイントパス

設定ファイルはプログラムの起動時に自動的に読み込まれます。設定ファイルが存在しない場合や読み込みに失敗した場合は、デフォルト値が使用されます。

## Streamable HTTP (SSE) モードでの使用例

HTTPモードでは、Streamable HTTP (Server-Sent Events) を使用してMCPサーバーと通信します。これにより、より効率的な双方向通信が可能になります。

Roo CodeなどのMCPクライアントからHTTPモードのサーバーに接続するには、以下のようなURLを使用します：

```
http://localhost:7991/mcp
```

ポート番号やホスト名は、設定ファイルや起動時のフラグで変更されている場合があります。

## キャッシュ

HTTPリクエストの結果は `config.yaml` で指定されたディレクトリにキャッシュされます。これにより、同じリクエストを繰り返し行った場合にネットワークトラフィックを削減し、応答速度を向上させることができます。

キャッシュの有効期間（TTL）は、設定ファイルで指定できます。デフォルトのTTLは1時間です。
