<h1 align="center">ClipSync-Admin</h1>

<p align="center">
  <b>ClipSync 管理画面バックエンド</b><br/>
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

---

ClipSync-Admin は、セルフホスト型 [ClipSync](https://github.com/JH-Clipsync) クロスデバイスメッセージ同期システムの管理画面バックエンドで、**Go + Gin + GORM + MySQL + Redis + JWT** で構築されています。運用/サポート/管理者向けに、ダッシュボード統計、ユーザー管理、端末管理、RBAC（管理者/ロール/メニュー/権限）、管理者認証の機能を提供し、Redis Pub/Sub と HTTP の二つの経路で [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) と連携します。これにより「パスワードをリセットしたら即座に端末をキック」「端末を無効化したら即座に切断」が実現されます。

フロントエンドは [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web)（Vue 3 + Element Plus）です。

---

## ✨ 主な機能

| 分類 | 内容 |
|------|------|
| 📊 **ダッシュボード** | ユーザー総数、アクティブユーザー数、管理者数、ロール数の統計 |
| 👤 **ユーザー管理** | 一覧（検索/無効フィルタ/ページング）、詳細、編集、有効/無効化、パスワードリセット（ランダムパスワードを自動生成し平文を一度だけ返却）、削除、強制キック。各ユーザーに端末総数とオンライン数を表示 |
| 📱 **端末管理** | ユーザーの全端末を一覧（ロール/プラットフォーム/カスタム名/最終 IP/オンライン状態）、全ユーザー横断のページング検索、有効/無効化、リネーム、単一端末のキック |
| 🔌 **リアルタイムオンライン状態** | 端末一覧はまず Server の `GET /server-admin/users/{id}/devices` を呼び出し、オンライン状態は Server のメモリ Hub 準拠。Server 不通時はローカル MySQL + Redis にフォールバック |
| 📣 **Server 連携通知** | パスワードリセット/ユーザー凍結/ユーザー削除/端末無効化時に Redis Pub/Sub（チャネル `clipsync:admin:kick_user`）で Server に即時切断を通知。Redis 不通時は HTTP にフォールバック |
| 🛡️ **RBAC** | 管理者/ロール/メニュー/API 権限の完全な CRUD。ロール-メニュー、メニュー-権限、管理者-ロールの多対多リレーション、エンドポイントレベルの認可。内蔵スーパー管理者 `admin` は削除不可で保護 |
| 🔐 **管理者認証** | JWT（TTL 2時間、スライディングリフレッシュ）、管理者パスワードは bcrypt、失敗回数によるアカウントロック、失効可能なトークン（jti を Redis に保存） |
| ✍️ **API リクエスト署名** | すべてのリクエストに HMAC-SHA256 署名（method/path/query/timestamp/nonce/bodyMD5）が必要。ログイン前は静的鍵 `sign_static_secret`、ログイン後はセッション単位の動的鍵に切り替え、改ざん/リプレイを防止 |
| 🖼️ **画像アップロード** | 管理者アバターなど。拡張子ホワイトリスト + サイズ制限 + 日付ごとのディレクトリ分割 |
| 🧩 **DB 共有** | ClipSync-Server と同一の MySQL `clipsync` DB を共有。業務テーブル `users`/`devices` は Server が作成し、Admin は自身の `admin_rbac_*` テーブルのみ移行（すべて `admin_` プレフィックスで衝突を回避） |
| 🔑 **デュアルハッシュ** | 管理者パスワードは bcrypt、業務ユーザーパスワードは scrypt（Server と完全互換、N=32768/r=8/p=1） |
| 🐳 **Docker ネイティブ** | マルチステージビルドで distroless nonroot イメージを生成、ホストネットワークでホスト上の MySQL/Redis に直接接続、待受 `:28002`。デプロイスクリプトが Server 設定から Admin 設定を自動生成 |

---

## 🏗️ 技術スタック

- **言語**: Go 1.22以上
- **HTTP フレームワーク**: [Gin](https://github.com/gin-gonic/gin) v1.10
- **ORM**: [GORM](https://gorm.io) v1.25 + `gorm.io/driver/mysql`
- **データベース**: MySQL 8（ClipSync-Server と共有）
- **キャッシュ**: Redis（別 DB、デフォルト db=2）
- **JWT**: `golang-jwt/jwt/v5`、jti による失効対応
- **設定**: [viper](https://github.com/spf13/viper)（YAML + `CLIPSYNC_ADMIN_` 環境変数プレフィックス）
- **ログ**: [zap](https://github.com/uber-go/zap)
- **パスワードハッシュ**: 管理者は bcrypt / 業務ユーザーは scrypt
- **イメージ**: `gcr.io/distroless/base-debian12:nonroot`

---

## 🚀 クイックスタート

### 前提

- MySQL 8（ClipSync-Server によって `clipsync` DB と `users` / `sessions` / `devices` テーブルが初期化済み）
- Redis（Server と同一インスタンスを共有、Admin は db=2 を使用）
- ClipSync-Server が稼働し、`server.admin_token` が設定済み

### 方法1: Docker（推奨）

`deploy/deploy.sh` は Server の `config.yaml` から MySQL/Redis 接続情報を自動的に読み取り、Admin の設定ファイルを生成します。

```bash
git clone https://github.com/JH-Clipsync/ClipSync-Admin.git
cd ClipSync-Admin

# デプロイスクリプトはデフォルトで /app/ClipSync/admin/api に配置
sudo mkdir -p /app/ClipSync/admin/api/config /app/ClipSync/admin/api/uploads
sudo cp deploy/docker-compose.yml /app/ClipSync/admin/api/

# config/config.yaml はデプロイスクリプトが Server 設定から自動生成します。
# server.http_admin_token が Server の server.admin_token と一致していることを確認

cd /app/ClipSync/admin/api
docker compose up -d
docker compose logs -f admin
```

デフォルトの待受アドレスは `:28002`。

### 方法2: 公式イメージを取得

```bash
docker run -d --name clipsync-admin \
  --network host \
  -v $(pwd)/config:/data/config:ro \
  -v $(pwd)/uploads:/data/uploads \
  -e TZ=Asia/Shanghai \
  ghcr.io/jh-clipsync/clipsync-admin:latest
```

イメージレジストリ: [ghcr.io/jh-clipsync/clipsync-admin](https://github.com/orgs/JH-Clipsync/packages)

### 方法3: ソースから実行

```bash
# 前提: Go 1.22以上、到達可能な MySQL と Redis
cp config.example.yaml config.yaml
# config.yaml を編集:
#   - mysql.dsn を clipsync DB に向ける
#   - redis.addr / redis.db
#   - jwt.secret をランダムな文字列に
#   - server.http_admin_token を Server の server.admin_token と一致させる

go mod tidy
go run .
# 設定ファイルを指定する場合
go run . -c /path/to/config.yaml
```

### デフォルトアカウント

初回起動時にスーパー管理者が自動的にシードされます。

- アカウント: `admin`
- パスワード: `Admin**8`

**初回ログイン後、直ちにパスワードを変更してください。** 内蔵スーパー管理者は保護されており削除できません。

---

## ⚙️ 設定

[config.example.yaml](config.example.yaml) を参照してください。設定ファイルは `-c` で指定できるほか、環境変数 `CLIPSYNC_ADMIN_` プレフィックスでも上書きできます（例: `CLIPSYNC_ADMIN_APP_ADDR`）。

| セクション | 主要項目 | 説明 |
|---|---|---|
| `app` | `addr` / `mode` | 待受アドレス（デフォルト `:28002`） / Gin モード（`debug` / `release`） |
| `mysql` | `dsn` / `max_idle_conns` / `max_open_conns` | Server と同じ `clipsync` DB。Admin が移行するのは RBAC テーブルのみ |
| `redis` | `addr` / `password` / `db` | デフォルト db=2（Server の db=0 との衝突を回避） |
| `jwt` | `secret` / `ttl` / `refresh_on_access` | JWT 署名鍵、有効期間（秒、デフォルト7200=2時間）、スライディングリフレッシュ |
| `security` | `bcrypt_cost` / `login_error_limit` / `login_error_ttl` / `sign_static_secret` | bcrypt cost、ログイン失敗ロック閾値/期間、ログイン前の静的署名鍵 |
| `cors` | `allow_origins` / `allow_credentials` | 許可するフロントエンドオリジン、ローカル開発時のデフォルトは `http://localhost:5175` |
| `log` | `level` / `format` | ログレベル（debug/info/warn/error） / 形式（console/json） |
| `bootstrap` | `super_admin_account` / `super_admin_password` / `super_admin_name` | 起動時にシードするスーパー管理者（既にあればスキップ） |
| `upload` | `dir` / `url_prefix` / `max_size` / `allow_ext` | アップロード先、URL プレフィックス、1ファイル上限、拡張子ホワイトリスト |
| `server` | `key_prefix` / `addr` / `http_admin_token` | Server 連携: Redis キープレフィックス（Server と一致必須）、Server HTTP フォールバック先、Server の admin_token |

### Server との連携設定

Admin は2つの経路で Server に通知します。

1. **Redis Pub/Sub（主経路）**: チャネル名 = `server.key_prefix + "admin:kick_user"`（デフォルト `clipsync:admin:kick_user`）。追加設定不要で最も安定。Admin と Server が同じ Redis インスタンスに接続している必要があります。
2. **HTTP フォールバック**: Redis が利用できない場合、`server.addr + "/server-admin/kick"` にコマンドを POST し、`Authorization: Bearer <server.http_admin_token>` を付与します。

端末一覧/ユーザー作成/リネームなどの**クエリと書き込み**は常に HTTP で Server を呼びます（`server.addr` が必要）。Server が `devices` テーブルとメモリ Hub の権威です。

---

## 🔌 API リファレンス

すべてのエンドポイントは `/api/admin` プレフィックスで、署名ミドルウェアを通過する必要があります。

### 公開エンドポイント

| パス | メソッド | 説明 |
|---|---|---|
| `/api/admin/health` | GET | ヘルスチェック |
| `/api/admin/auth/login` | POST | 管理者ログイン（JWT + 動的署名鍵を返却） |

### ログイン後のエンドポイント（JWT）

| パス | メソッド | 説明 |
|---|---|---|
| `/api/admin/auth/logout` | POST | ログアウト（jti を失効） |
| `/api/admin/auth/me` | GET | 現在の管理者情報 |
| `/api/admin/auth/menus` | GET | 現在の管理者に表示可能なメニュー |
| `/api/admin/auth/password` | PUT | 自身のパスワード変更 |
| `/api/admin/auth/profile` | PUT | 自身のプロフィール更新 |
| `/api/admin/upload/image` | POST | 画像アップロード |

### RBAC エンドポイント（JWT + 権限チェック）

| リソース | 操作 |
|---|---|
| ダッシュボード | `GET /dashboard` |
| ユーザー | `GET/POST /users`、`GET/PUT/DELETE /users/:id`、`PUT /users/:id/status`、`POST /users/:id/reset-password`、`POST /users/:id/kick` |
| ユーザー端末 | `GET /users/:id/devices`、`PUT /users/:id/devices/:did`（有効/無効）、`PUT /users/:id/devices/:did/name`（リネーム）、`POST /users/:id/devices/:did/kick` |
| 全端末 | `GET /devices`（全ユーザー横断の検索/フィルタ/ページング） |
| 管理者 | `GET/POST /rbac/admins`、`PUT/DELETE /rbac/admins/:id`、`PUT /rbac/admins/:id/status`、`PUT /rbac/admins/:id/password`、`GET /rbac/admins/:id/roles` |
| ロール | `GET/POST/PUT/DELETE /rbac/roles`、`PUT /rbac/roles/:id/menus`、`GET /rbac/roles/:id/menus` |
| メニュー | `GET/POST/PUT/DELETE /rbac/menus`、`PUT /rbac/menus/:id/perms` |
| 権限 | `GET/POST/PUT/DELETE /rbac/perms` |

### 署名ルール

署名対象文字列（`\n` 区切り）:

```
METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_MD5
```

- `PATH` は `/api/admin` プレフィックスを除いた相対パス
- `QUERY` はキーの辞書順にソート
- `TIMESTAMP` はミリ秒、`NONCE` は16バイトのランダム hex
- `BODY_MD5` はリクエストボディの MD5（GET/ボディなしは空文字）

署名 = `HMAC-SHA256(secret, 署名対象文字列)` の小文字 hex。フロントエンド実装は [ClipSync-Admin-Web/src/utils/sign.ts](https://github.com/JH-Clipsync/ClipSync-Admin-Web) を参照。

---

## 🔐 セキュリティ

| 観点 | 設計 |
|------|------|
| 管理者パスワード | bcrypt（デフォルト cost 10）、業務ユーザーとは独立 |
| 業務ユーザーパスワード | scrypt（N=32768/r=8/p=1）、ClipSync-Server と完全互換。パスワードリセット時は Admin が直接 DB に書き込み |
| JWT 失効 | トークンごとに jti を付与し、Redis に jti→adminID を保存。ログアウト時に削除して失効を実現 |
| ブルートフォース対策 | 同一アカウントで連続失敗が閾値（デフォルト5回）を超えると一定時間（デフォルト15分）ロック |
| リクエスト署名 | 全エンドポイントで HMAC-SHA256 署名を必須化し、タイムスタンプ + nonce で改ざん/リプレイを防止。ログイン前は静的鍵、ログイン後は動的鍵を発行 |
| CORS | オリジンをホワイトリストで制御し、`*` は使用しない |
| テーブル分離 | Admin 専用の RBAC テーブルはすべて `admin_` プレフィックスで業務テーブルとの衝突を回避 |
| DB 権限 | 本番では Admin が使う MySQL アカウントに `clipsync` DB の SELECT/UPDATE/INSERT/DELETE のみ付与し、DDL/DROP 権限を与えないことを推奨 |
| イメージ強化 | distroless nonroot（uid 65532）、シェルなし・パッケージマネージャなし |

---

## 🐳 デプロイ構成

### Nginx リバースプロキシのパス設計

[deploy/nginx.clipsync.conf](deploy/nginx.clipsync.conf) に同一オリジン配下の完全なパス設計例があります。

```
/clipsync/admin/api/       → 127.0.0.1:28002/api/admin/   （Admin API）
/clipsync/admin/static/    → 127.0.0.1:28002/static/      （Admin アップロード）
/clipsync/admin/           → /app/ClipSync/admin/web/      （Admin SPA）
/clipsync/ws               → 127.0.0.1:28001/ws            （Server WebSocket）
/clipsync/                 → 127.0.0.1:28001/              （Server 他 API）
```

[deploy/deploy.sh](deploy/deploy.sh) はデプロイ時に正しい location ブロックを nginx 設定へ書き込み、nginx を reload します。

### Server との実行関係

```
          ブラウザ (Vue 3 SPA)
                 │  HTTPS
                 ▼
        ┌── Nginx (/clipsync/admin/) ──┐
        │                              │
        ▼                              ▼
  ClipSync-Admin (:28002)       ClipSync-Server (:28001)
        │                              │
        │  ① Redis Pub/Sub             │
        ├──────────────┬───────────────┤
        │              │               │
        ▼              ▼               ▼
   MySQL (clipsync)   Redis db=0/2   メモリ Hub（端末オンライン）
   users/devices 共有   Pub/Sub チャネル
```

- Admin と Server は **MySQL と Redis を共有**します;
- Admin が `users`/`devices` を書き換えた後、Pub/Sub で Server に実際の切断/ステータス更新を実行させ、二重書き込みの不整合を回避します;
- 端末オンライン状態の権威は Server で、Admin はまず HTTP で Server を呼び出します。

---

## 📁 プロジェクト構成

```
ClipSync-Admin/
├── main.go                       # エントリ: 設定読み込み → DB/Redis/JWT 初期化 → Gin 起動
├── config.example.yaml           # 設定テンプレート
├── Dockerfile                    # マルチステージビルド → distroless nonroot
├── internal/
│   ├── config/                   # viper 設定ローダー
│   ├── db/                       # MySQL / Redis 接続初期化
│   ├── auth/
│   │   ├── jwt.go                # JWT 発行/検証
│   │   ├── password.go           # 管理者 bcrypt + 業務ユーザー scrypt
│   │   └── sign.go               # HMAC-SHA256 リクエスト署名
│   ├── model/
│   │   ├── base.go               # 共通カラム（status/is_del/c_by/...）
│   │   ├── biz.go                # User（Server の users テーブルにマップ）
│   │   └── rbac.go               # Admin/Role/Menu/Perm とジョイントテーブル
│   ├── result/                   # 統一レスポンスとエラーコード
│   ├── middleware/
│   │   ├── sign.go               # グローバル署名検証
│   │   ├── rbac.go               # JWT + エンドポイント権限チェック
│   │   └── common.go             # CORS / TraceID / AccessLog
│   ├── service/
│   │   ├── auth_service.go       # 管理者ログイン/セッション/動的署名鍵
│   │   ├── rbac_service.go       # RBAC CRUD
│   │   ├── data_service.go       # ユーザー/端末/ダッシュボード
│   │   └── server_notifier.go    # Redis Pub/Sub + HTTP 二経路 Server 通知
│   ├── handler/                  # HTTP ハンドラ（auth/data/rbac/upload）
│   ├── bootstrap/                # AutoMigrate + スーパー管理者/メニュー/権限シード
│   ├── router/                   # ルート組み立て
│   └── logger/                   # zap ロガー
├── deploy/
│   ├── deploy.sh                 # ワンショットデプロイ（CI から SSH 実行）
│   ├── docker-compose.yml
│   ├── nginx.clipsync.conf       # 完全なリバースプロキシ例
│   └── .env.example
└── .github/workflows/docker-image.yml
```

---

## 🐛 トラブルシューティング

| 現象 | 確認すること |
|------|--------------|
| ログインで署名エラー | フロントの `VITE_SIGN_STATIC_SECRET` がバックエンドの `security.sign_static_secret` と一致しているか。システム時刻が大きくずれていないか |
| キックが効かない | Server は起動しているか。`server.key_prefix` が Server の `redis.key_prefix` と一致しているか。Server に `server.admin_token` が設定されているか。Server ログに「管理端末イベント購読が切断されました」が出ていないか |
| 端末オンライン状態が不正確 | Admin が Server HTTP API に到達できない場合ローカル Redis にフォールバックします。`server.addr` と `http_admin_token` を確認。最も権威があるのは Server のメモリ Hub です |
| ユーザー作成に失敗する | Admin のコンテナ/プロセスから `server.addr` で Server に到達できるか。ユーザー作成はローカル DB 挿入ではなく HTTP で Server を呼びます |
| コンテナの時刻がおかしい | docker-compose で `TZ=Asia/Shanghai` を設定済み。ホストのタイムゾーンを確認 |
| RBAC テーブルがない | MySQL アカウントに CREATE TABLE 権限があるか。初回起動時に `admin_rbac_*` テーブルを自動マイグレーションします |
| 画像アップロードが 413 | nginx の `client_max_body_size` はデフォルト1m、サンプル設定で12m に変更済み。外側のゲートウェイで制限されていないか |

ログは stdout（コンテナ）に出力されます。`docker compose logs -f admin` で確認してください。

---

## 🤝 関連プロジェクト

| プロジェクト | 技術スタック | リンク |
|------|--------|------|
| 中継サーバー | Go + gorilla/websocket | [JH-Clipsync/ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) |
| 管理画面フロント | Vue 3 + Element Plus | [JH-Clipsync/ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web) |
| Android クライアント | Kotlin + OkHttp | [JH-Clipsync/ClipSync-Android](https://github.com/JH-Clipsync/ClipSync-Android) |
| macOS クライアント | Swift + SwiftUI | [JH-Clipsync/ClipSync-Mac](https://github.com/JH-Clipsync/ClipSync-Mac) |
| Windows クライアント | .NET 8 + WPF | [JH-Clipsync/ClipSync-Windows](https://github.com/JH-Clipsync/ClipSync-Windows) |

---

## 📄 License

個人利用のプロジェクトです。コードの参照・改変は自由に行ってください。

---

**Made with ❤️ · 全プラットフォーム自作 · データはあなたのもの**
