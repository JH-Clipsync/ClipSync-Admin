# ClipSync-Admin

ClipSync 管理后台后端，基于 Go + Gin + GORM + MySQL + Redis。

## 功能

- 仪表盘：用户总数、活跃用户、管理员数、角色数
- 用户管理：列表（搜索/过滤/分页）、详情、更新、启用禁用、重置密码、删除
- RBAC：管理员/角色/菜单/权限接口的完整 CRUD
- 认证：登录/登出/me/菜单/改密码/改资料
- 图片上传

## 与 ClipSync-Server 的关系

- 复用同一个 `clipsync` MySQL 数据库
- 直接读写 ClipSync-Server 的 `users` 表（不 AutoMigrate，表结构由 ClipSync-Server 管理）
- 业务用户密码使用 scrypt 哈希（格式 `scrypt$N$r$p$<salt>$<dk>`，与 ClipSync-Server 兼容）
- 管理员相关表加 `admin_` 前缀（`admin_rbac_*`），避免冲突
- 管理员密码使用 bcrypt
- Redis 使用 db 2，避免与 ClipSync-Server（db 0）冲突

## 快速开始

```bash
# 1. 复制配置
cp config.example.yaml config.yaml
# 按需修改 config.yaml 中的数据库/Redis 连接信息

# 2. 安装依赖
go mod tidy

# 3. 编译运行
go run .

# 或指定配置文件
go run . -c /path/to/config.yaml
```

默认监听 `:28002`，超级管理员账号 `admin` / `Admin**8`。

## 配置

环境变量前缀：`CLIPSYNC_ADMIN_`（如 `CLIPSYNC_ADMIN_APP_ADDR`）。

详见 `config.example.yaml`。

## 目录结构

```
ClipSync-Admin/
  main.go
  go.mod
  config.example.yaml
  .gitignore
  internal/
    config/       # 配置加载
    db/           # MySQL/Redis 连接
    logger/       # zap 日志
    auth/         # JWT + 密码哈希（bcrypt 管理员 / scrypt 业务用户）
    model/        # GORM 模型（base/rbac/biz）
    result/       # 统一响应
    middleware/   # CORS/TraceID/AccessLog/JWTAuth/RBAC
    service/      # 业务逻辑（auth/rbac/data）
    handler/      # HTTP 处理器（auth/data/rbac/upload）
    bootstrap/    # AutoMigrate + 种子数据
    router/       # 路由组装
  data/uploads/   # 上传文件目录
```
