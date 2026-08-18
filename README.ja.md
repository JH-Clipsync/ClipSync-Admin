<h1 align="center">ClipSync-Admin</h1>

<p align="center">
  <b>ClipSync 管理バックエンド</b><br/>
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

---

ClipSync-Admin は、[ClipSync](https://github.com/JH-Clipsync) のセルフホスト型クロスデバイスメッセージ同期システムの管理バックエンドです。**Go + Gin + GORM + MySQL + Redis + JWT** で開発されています。運用 / カスタマーサポート / 管理者向けに、ユーザー管理、デバイス管理、ダッシュボード統計、RBAC ロール権限、管理者認証などの機能を提供します。Redis Pub/Sub と HTTP の2つのチャネルで [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) と連携し、「パスワード変更で即ログアウト、デバイス無効化で即切断」を実現します。デフォルトでは **28002** ポートで待ち受けます。

付属するフロントエンドは [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web)（Vue 3 + Element Plus）です。

---

## ✨ 主な機能

| モジュール | 説明 |
|------------|------|
| 📊 **ダッシュボード統計** | ユーザー総数、アクティブユーザー数（`disabled=0`）、管理者数、ロール数 |
| 👤 **ユーザー管理** | ユーザー一覧（ユーザー名検索 / ステータスフィルタ / ページネーション）、詳細、作成、編集、有効/無効化、パスワードリセット（10文字のランダム平文を生成して返却）、物理削除、強制ログアウト。ユーザーごとにデバイス総数と現在のオンライン数を集計 |
| 📱 **デバイス管理** | ユーザーの全デバイス一覧（ロール / プラットフォーム / カスタム名 / 最終 IP / オンライン状態）、ユーザーをまたいだページネーション検索（ユーザー名 / デバイス ID / 名前 / IP のあいまい一致）、デバイスの有効/無効化、リネーム、1台のみキック |
| 🔌 **リアルタイムオンライン状態** | デバイス一覧は**Server への HTTP 呼び出しを優先**し、`GET /server-admin/users/{id}/devices` でリアルタイムのオンライン状態を取得（Server のメモリ Hub を正とします）。Server が利用できない場合はローカルの MySQL + Redis に自動フォールバック |
| 📣 **Server 連携通知** | パスワードリセット / ユーザーBAN / ユーザー削除 / デバイス無効化 / デバイスキック時、Redis Pub/Sub（チャネル `clipsync:admin:kick_user`）経由で Server に即時切断を通知。Redis が不通の場合は HTTP `POST {server.addr}/server-admin/kick` にフォールバックする二重構成 |
| 🛡️ **RBAC 権限** | 管理者 / ロール / メニュー / API 権限の完全な CRUD。ロール-メニュー、メニュー-権限、管理者-ロールの多対多リレーション。インターフェースレベルのインターセプト。内蔵スーパー管理者 `admin` は保護され削除不可 |
| 🔐 **管理者認証** | JWT（HS256、TTL 2時間、スライディングリフレッシュ）、bcrypt 管理者パスワードハッシュ（cost 設定可）、ログイン失敗回数によるロック、失効可能なトークン |
| ✍️ **API リクエスト署名** | すべてのリクエストに HMAC-SHA256 署名（method / path / query / timestamp / nonce / bodyMD5）が必要です。ログイン前は静的鍵 `sign_static_secret` を使用し、ログイン後はセッションレベルの動的鍵に切り替わり、リプレイや改ざんを防止します |
| 🖼️ **画像アップロード** | 管理者アバターなどの画像アップロード。拡張子ホワイトリスト + サイズ制限 + 日付別ディレクトリ保存。外部には静的ディレクトリとして公開 |
| 🗄️ **Server とのデータベース共有** | Server の `users` / `devices` テーブルを直接読み取り。管理者関連テーブルはすべて `admin_` プレフィックスを使用し、互いに干渉しません |
| 🐳 **Docker ネイティブ** | マルチステージビルド → distroless nonroot イメージ。host ネットワークでホストの MySQL / Redis に直接接続。`deploy.sh` が Server 設定から admin 設定を自動生成可能 |

---

## 🚀 クイックスタート

### 前提条件

- [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) がデプロイ済みで、同じ MySQL / Redis に接続できること
- MySQL に `clipsync` データベースが既に存在すること（Server の起動時に `users` / `sessions` / `devices` テーブルが自動作成されます）
- Docker（推奨）または Go 1.22 以上

### 方法1：Docker Compose（推奨）

```bash
# 1. デプロイディレクトリの準備
mkdir -p /app/ClipSync/admin/api/config /app/ClipSync/admin/api/uploads
cd /app/ClipSync/admin/api

# 2. compose ファイルのコピー
cp deploy/docker-compose.yml .
cp deploy/.env.example .env

# 3. Server 設定から admin config.yaml を自動生成
#    スクリプトは以下の順序で Server 設定を探します：
#      /app/ClipSync/server/config/config.yaml
#      /app/ClipSync/server/config.yml
#      /app/ClipSync/config/config.yaml
#      /app/Clipsync/server/config/config.yaml
#      /opt/clipsync/config/config.yaml
bash deploy/deploy.sh
```

`deploy.sh` は以下を行います：

1. Server の `config.yaml` から MySQL / Redis / `key_prefix` / `admin_token` を自動読み取り
2. ローカルの `config/config.yaml` を生成。JWT 秘密鍵は `/dev/urandom` でランダム生成
3. 過去の移行スクリプトによる設定破損（例：`redis.addr` が誤って http URL に変更されている等）を検出して修復
4. `docker compose pull && up -d --force-recreate`
5. nginx 設定（API パス、静的リソース location）を確認して修復

デフォルトのスーパー管理者アカウント：

```
アカウント：admin
パスワード：Admin**8
```

> 初回ログイン後、すぐにパスワードを変更してください。

### 方法2：Docker ワンライナー

```bash
docker run -d --name clipsync-admin \
  --network host \
  --restart unless-stopped \
  -v $(pwd)/config:/data/config:ro \
  -v $(pwd)/uploads:/data/uploads \
  -e TZ=Asia/Shanghai \
  ghcr.io/jh-clipsync/clipsync-admin:latest \
  -c /data/config/config.yaml
```

### 方法3：ソースから実行

```bash
git clone https://github.com/JH-Clipsync/ClipSync-Admin.git
cd ClipSync-Admin

# 設定の準備
cp config.example.yaml config.yaml
vim config.yaml   # MySQL / Redis / JWT 秘密鍵 / Server 連携情報を記入

# 実行
go run .

# またはビルド
CGO_ENABLED=0 go build -ldflags "-s -w" -o clipsync-admin .
./clipsync-admin -c config.yaml
```

起動時に自動で以下を実行します：

- すべての `admin_` プレフィックスの RBAC テーブルを移行
- スーパー管理者アカウント `admin / Admin**8` をシード（既に存在する場合はスキップ）
- デフォルトメニューと権限ノードをシード

---

## ⚙️ 設定

設定ファイルの例は [config.example.yaml](config.example.yaml) を参照してください。viper で読み込み、環境変数をサポートします（プレフィックス `CLIPSYNC_ADMIN_`）。

```yaml
app:
  name: clipsync-admin
  addr: ":28002"        # 待ち受けポート
  mode: debug           # gin モード：debug / release / test

mysql:
  # ClipSync-Server と同じデータベースを共有
  dsn: "clipsync:clipsync@tcp(127.0.0.1:3306)/clipsync?charset=utf8mb4&parseTime=True&loc=Local"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 3600

redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 2                 # 競合回避のため Server とは別の db を推奨（プレフィックスはありますが）

jwt:
  secret: "change-me-clipsync-admin-secret"   # ⚠️ 本番環境では必ず変更
  header: "Authorization"
  scheme: "Bearer"
  ttl: 7200             # トークン有効期間（秒）、デフォルト2時間
  refresh_on_access: true

security:
  bcrypt_cost: 10       # 管理者パスワードの bcrypt cost
  login_error_limit: 5  # ログイン失敗何回でロックするか
  login_error_ttl: 900  # 失敗カウントのウィンドウ（秒）
  # ログイン前エンドポイントの署名鍵（フロントエンドにも同じ文字列をハードコード。ログイン成功後は動的鍵に切り替わります）
  sign_static_secret: "clipsync-admin-static-sign-secret-v1"

cors:
  allow_origins:
    - "http://localhost:5175"
  allow_credentials: true

log:
  level: "info"
  format: "console"     # console / json

bootstrap:
  super_admin_account: "admin"
  super_admin_password: "Admin**8"
  super_admin_name: "スーパー管理者"

upload:
  dir: "./data/uploads"
  url_prefix: "/static"
  max_size: 10485760    # 10MB
  allow_ext: [".jpg", ".jpeg", ".png", ".webp", ".gif"]

# ClipSync-Server との連携チャネル
server:
  key_prefix: "clipsync:"         # Server の redis.key_prefix と一致する必要があります
  addr: "http://127.0.0.1:28001"  # Server の HTTP アドレス（HTTP フォールバック用）
  http_admin_token: ""            # Server の server.admin_token と一致する必要があります
```

### 主な設定の注意点

| 設定 | 説明 |
|------|------|
| `mysql.dsn` | Server と同じデータベースに接続する必要があります。Admin は `admin_*` テーブルのみ自動移行し、`users` / `devices` テーブル構造は変更しません |
| `redis.db` | Server とは異なる db を選択することを推奨（Server デフォルト db=0、Admin 例では db=2）。`key_prefix` は Pub/Sub メッセージを受信するために Server と完全に一致する必要があります |
| `jwt.secret` | `openssl rand -hex 32` または `head -c 32 /dev/urandom \| base64` で生成 |
| `server.key_prefix` | Pub/Sub チャネル名 `{prefix}admin:kick_user` を決定します。Server と一致する必要があります |
| `server.addr` | デバイス一覧の HTTP 取得、ユーザー作成、デバイスリネームのフォールバックチャネル。内网 / ローカルアドレスの指定を推奨 |
| `server.http_admin_token` | HTTP フォールバック時の Bearer Token。Server の `server.admin_token` と完全に一致する必要があります |

---

## 🏗️ プロジェクト構成

```
┌────────────────────┐         ┌────────────────────────────┐
│ ClipSync-Admin-Web │ ──API──▶│     ClipSync-Admin         │
│   (Vue 3 + EP)     │ ◀────── │  Gin + GORM + JWT + HMAC   │
└────────────────────┘         └──────────┬─────────────────┘
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    │                     │                     │
                    ▼                     ▼                     ▼
             ┌────────────┐       ┌────────────┐       ┌────────────────┐
             │   MySQL    │       │   Redis    │       │ ClipSync-Server│
             │ users/     │       │ オンライン /│       │  (HTTP フォール│
             │ devices/   │       │ Pub/Sub /  │       │  バック)       │
             │ admin_*    │       │ JWT/jti    │       │ /server-admin/*│
             └────────────┘       └────────────┘       └────────────────┘
```

### レイヤー構造

```
internal/
├── main.go
├── config/           # 設定構造体 + viper 読み込み
├── router/           # ルーティング組み立て、ミドルウェア登録
├── middleware/       # JWT 認証、RBAC、署名検証、CORS、TraceID、アクセスログ
├── handler/          # HTTP コントローラ（auth / data / rbac / upload）
├── service/          # ビジネスロジック
│   ├── auth_service.go
│   ├── data_service.go       # ユーザー / デバイス / ダッシュボード
│   ├── rbac_service.go
│   └── server_notifier.go    # Redis Pub/Sub + HTTP 2重チャネル配信
├── model/            # GORM モデル（biz は Server テーブルを共有、rbac は admin_ プレフィックス）
├── auth/             # JWT、bcrypt + scrypt パスワードハッシュ、HMAC 署名
├── bootstrap/        # テーブル移行、スーパー管理者とメニューのシード
├── db/               # MySQL / Redis 接続初期化
├── logger/           # zap ログ
└── result/           # 統一レスポンス構造とエラーコード
```

### データモデル

- **Server と共有するテーブル**：`users`、`devices`（Admin は一部フィールドの書き込みのみ行い、テーブル構造は Server が管理）
- **RBAC テーブル（すべて `admin_` プレフィックス）**：
  - `admin_rbac_admin`：管理者
  - `admin_rbac_role`：ロール
  - `admin_rbac_admin_role`：管理者 ↔ ロール
  - `admin_rbac_menu`：メニュー / ボタン / データ列
  - `admin_rbac_perm`：API 権限（route + method でユニーク）
  - `admin_rbac_role_menu`：ロール ↔ メニュー
  - `admin_rbac_menu_perm`：メニュー ↔ 権限

### Server との連携メカニズム

| トリガーアクション | Redis Pub/Sub | HTTP フォールバック |
|--------------------|---------------|---------------------|
| ユーザーパスワードリセット | ✅ `kick_user` / `password_reset` | ✅ |
| ユーザー無効化 | ✅ `kick_user` / `user_disabled` | ✅ |
| ユーザー削除 | ✅ `kick_user` / `user_deleted` | ✅ |
| ユーザーを強制ログアウト | ✅ `kick_user` / `device_kicked` | ✅ |
| デバイス無効化 | ✅ `disable_device` | ✅ |
| デバイス有効化 | ✅ `enable_device` | ✅ |
| 1台のデバイスをキック | ✅ `kick_device` | ✅ |
| デバイスリネーム | — | ✅ `PUT /server-admin/users/{id}/devices/{did}/name` |
| ユーザー作成 | — | ✅ `POST /server-admin/users`（パスワードは Server が scrypt でハッシュ化） |
| デバイス一覧 / オンライン状態 | — | ✅ `GET /server-admin/users/{id}/devices` と `GET /server-admin/devices` |

デバイス一覧は **Server HTTP を優先**します。オンライン状態は Server のメモリ Hub が最も権威的だからです。Server に到達できない場合にのみ、ローカルの MySQL + Redis クエリにフォールバックします。

---

## 🌐 デプロイ

### Docker Compose（host ネットワーク）

[deploy/docker-compose.yml](deploy/docker-compose.yml) は host ネットワークを使用し、コンテナはホストの `:28002` を直接待ち受けます。`127.0.0.1` で Server や MySQL / Redis に直接接続できます：

```yaml
services:
  admin:
    image: ghcr.io/jh-clipsync/clipsync-admin:${ADMIN_TAG:-latest}
    container_name: clipsync-admin
    restart: unless-stopped
    network_mode: host
    command: ["-c", "/data/config/config.yaml"]
    volumes:
      - ./config:/data/config:ro
      - ./uploads:/data/uploads
    environment:
      - TZ=Asia/Shanghai
```

### Nginx リバースプロキシ

完全な例は [deploy/nginx.clipsync.conf](deploy/nginx.clipsync.conf) を参照してください：

```nginx
# Admin バックエンド API
location ^~ /clipsync/admin/api/ {
    proxy_pass http://127.0.0.1:28002/api/admin/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# Admin アップロードファイル
location ^~ /clipsync/admin/static/ {
    proxy_pass http://127.0.0.1:28002/static/;
}

# Admin フロントエンド静的ファイル
location /clipsync/admin/ {
    alias /app/ClipSync/admin/web/;
    index index.html;
    try_files $uri $uri/ /clipsync/admin/index.html;
}

# ClipSync-Server WebSocket
location = /clipsync/ws {
    proxy_pass http://127.0.0.1:28001/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;
}
```

### ディレクトリ構成（本番環境）

```
/app/ClipSync/
├── server/              # ClipSync-Server
│   └── config/config.yaml
└── admin/
    ├── api/             # 本プロジェクト
    │   ├── config/config.yaml
    │   ├── uploads/
    │   └── docker-compose.yml
    └── web/             # ClipSync-Admin-Web ビルド成果物（静的ファイル）
        ├── index.html
        └── assets/
```

---

## 🔐 セキュリティについて

- **管理者パスワード**：bcrypt ハッシュ、cost デフォルト10（`security.bcrypt_cost` で調整可）
- **業務ユーザーパスワード**：Admin がユーザーパスワードをリセットする際、Server と完全に同一の **scrypt** ハッシュ（N=32768, r=8, p=1）を使用し、共有 DB で読み取れることを保証
- **JWT**：HS256 署名、ペイロードに `aid`（管理者 ID）+ `acc`（アカウント）+ `jti` を含む。Redis による失効とスライディングリフレッシュをサポート
- **ログインロック**：デフォルト15分以内に連続5回失敗するとアカウントをロック
- **API 署名（HMAC-SHA256）**：
  - 署名対象文字列：`METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_MD5`
  - `PATH` は `/api/admin` プレフィックスを除いた相対パス
  - `QUERY` は key の辞書順にソート
  - ログイン前は静的鍵 `sign_static_secret` を使用。ログイン成功後、サーバーがセッションレベルのランダム `signSecret` を発行し、ログアウトで失効
  - 署名比較は `hmac.Equal` を使用し、定数時間でタイミング攻撃を防止
- **CORS**：デフォルトでは設定されたオリジンのみ許可。本番環境では `cors.allow_origins` を実際のフロントエンドドメインに変更してください
- **Server 通信**：HTTP フォールバックには `Authorization: Bearer <server.admin_token>` が必須。空の場合は Server に拒否されます。Redis Pub/Sub は内网経由で、公衆網には露出しません
- **コンテナセキュリティ**：distroless nonroot イメージ、シェルなし、非 root ユーザーで実行
- **デフォルトパスワード**：スーパー管理者のデフォルトパスワード `Admin**8` は初回起動専用です。**デプロイ後は必ずすぐに変更**してください。`jwt.secret` と `sign_static_secret` も必ず差し替えてください

---

## 🤝 関連プロジェクト

- [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server)：3端末同期リレーサーバー（Go + gorilla/websocket）
- [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web)：管理フロントエンド（Vue 3 + Element Plus）
- [ClipSync-Windows](https://github.com/JH-Clipsync/ClipSync-Windows)：Windows クライアント
- [ClipSync-Mac](https://github.com/JH-Clipsync/ClipSync-Mac)：macOS クライアント
- [ClipSync-Android](https://github.com/JH-Clipsync/ClipSync-Android)：Android クライアント
