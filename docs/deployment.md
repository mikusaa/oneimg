# 部署指南

新部署的最短启动步骤见 [README](../README.md#快速启动)。本篇说明部署方式、持久化、配置和公网访问；已有部署请先阅读[升级指南](upgrading.md)。

## 使用 GHCR 镜像

推荐选择固定版本，便于确认运行内容和制定升级计划：

```bash
docker pull ghcr.io/mikusaa/oneimg:v4.4.1
```

按照 [README 的 `docker run` 示例](../README.md#快速启动)启动，按实际情况调整端口、`PUID`、`PGID`、`APP_URL` 和宿主机挂载目录。镜像提供 `linux/amd64`、`linux/arm64`，由 Docker 选择对应架构。

`latest` 会随主线构建更新，不适合用来确认某个固定版本。升级已有容器不能只执行拉取命令，应先完成备份，再使用原挂载和配置重建容器，见[通用升级流程](upgrading.md#通用升级流程)。

## 使用 Compose 从源码部署

仓库中的 [Compose 模板](../docker-compose.yml.example)使用 `build: .`，会从当前检出的源码构建，并非拉取 GHCR 镜像。需要 Docker Engine 和 Compose v2：

```bash
git clone --branch v4.4.1 --depth 1 https://github.com/mikusaa/oneimg.git
cd oneimg
cp docker-compose.yml.example docker-compose.yml
mkdir -p data uploads
```

如需更换文件属主或使用公开域名，启动前在项目根目录创建供 Compose 插值的 `.env`：

```dotenv
PUID=1000
PGID=1000
APP_URL=https://oneimg.example.com
PASSKEY_RP_ID=oneimg.example.com
PASSKEY_ORIGINS=https://oneimg.example.com
PASSKEY_RP_NAME=OneImg
```

本机试用可不创建此文件，模板的访问地址默认为 `http://localhost:8080`。确认配置后启动：

```bash
docker compose up -d --build
docker compose logs --tail 100 oneimg
```

停止服务使用 `docker compose down`，不会删除绑定挂载的 `data` 和 `uploads`。不要在需要保留数据时额外执行目录清理。

也可以单独从源码构建镜像，再将 README 示例末尾的镜像名替换为 `oneimg:local`：

```bash
docker build -t oneimg:local .
```

生产 Dockerfile 按顺序构建前端和 Go 后端，并提供 CGO / WebP 运行库。需要直接构建二进制或进行本地开发时，见[开发指南](development.md#工具链与项目结构)，不要只复制二进制而忽略运行库和持久化目录。

## 持久化与文件权限

以下是默认容器布局，宿主机目录可改，但容器路径应与应用配置一致：

| 宿主机目录 | 容器目录 | 内容 |
| --- | --- | --- |
| `./data` | `/app/data` | SQLite 数据库、应用 `.env`、本地缩略图、用户头像（数据库所在目录的 `avatars/`），以及初始化 / 恢复文件 |
| `./uploads` | `/app/uploads` | 本地图片主文件和视频原文件 |

默认数据库路径为 `/app/data/data.db`。远端存储的媒体不在这些目录中，需要独立备份；数据库与配置仍需保留。备份应保持数据库、配置和媒体一致，操作见[升级指南](upgrading.md#通用升级流程)。

头像、初始管理员密码和恢复码由 `STATE_DIR` 承载。SQLite 未设置此项时仍使用数据库所在目录；PostgreSQL 默认 `./data`。缩略图仍在 `data/thumbnails`，本地媒体仍在 `uploads`，设置 `STATE_DIR` 不会移动这些文件或改变 URL，也不改变 `.env` 的查找位置。使用 PostgreSQL 仍须持久化应用配置、状态与媒体。

容器默认以 root 执行入口脚本，每次启动会递归调整 `/app/data`、`/app/uploads` 的属主，再以 `PUID:PGID` 运行应用，默认 `1000:1000`。Linux 上可用以下命令确认需要使用的属主：

```bash
id -u
id -g
```

不要同时设置 Compose 的 `user` 或 `docker run --user` 来依赖入口脚本修复权限：非 root 启动时会跳过属主调整。挂载目录必须允许预期的权限操作；如启动时报 `chown` 失败，应先核对宿主目录及文件系统权限，不要将数据库或配置改为全局可写。

## 配置来源与优先级

三种配置位置用途不同，不能混用：

| 位置 | 谁读取 | 实际作用 |
| --- | --- | --- |
| Compose 项目根目录的 `.env` | Docker Compose | 为模板里的 `${变量}` 插值；不会自动把全部变量传入容器 |
| 容器环境变量 | 容器中的应用 / 入口脚本 | 仅 `docker run -e`、Compose `environment` 等显式传入的值可见 |
| 持久化应用配置 `data/.env` | OneImg | 首次启动自动生成，保存应用默认配置和稳定密钥 |

当前 Compose 模板只对 `PUID`、`PGID`、`APP_URL` 和三个 `PASSKEY_*` 变量使用插值，`TZ` 固定为 `Asia/Shanghai`。将 `TRUSTED_PROXIES`、`IMAGE_MAX_PIXELS` 或 `IMAGE_MAX_DIMENSION` 只写进项目根目录的 `.env` **不会生效**；使用此模板时，应修改宿主机持久化目录中的 `data/.env`。

应用选择配置文件的顺序是：`ONEIMG_ENV_FILE` 指定的路径 → 工作目录中已存在的 `.env` → `data/.env`。这是选择一个文件，不是依次合并三个文件。标准镜像工作目录为 `/app`，通常使用 `/app/data/.env`；直接运行源码时要特别注意根目录 `.env` 可能被当成应用配置。

选定文件只补充进程中尚未设置的变量，**进程环境变量优先于文件**。即使显式传入空值，也可能遮蔽持久化值；不要传入空的 `CONFIG_SECRET` 或 `SESSION_SECRET`。Compose 已传入的 `APP_URL` 等字段，应在 Compose 的配置来源修改，而不是期待修改 `data/.env` 覆盖它们。

| 常用应用配置 | 默认值 / 用途 |
| --- | --- |
| `SERVER_PORT` | `8080`，应用监听端口；更改时同步容器端口映射 |
| `APP_URL` | 标准部署为 `http://localhost:8080`；公网填写浏览器实际访问的地址 |
| `SQLITE_PATH` | `./data/data.db`，相对于应用工作目录 |
| `DB_TYPE` | `sqlite`；可选 `postgres`，未知值停止启动 |
| `DATABASE_URL` | PostgreSQL 必填，详见[数据库选择](#数据库选择) |
| `STATE_DIR` | SQLite 默认数据库目录，PostgreSQL 默认 `./data` |
| `DEFAULT_USER` / `DEFAULT_PASS` | 新库初始化使用；默认 `admin` / 空，不用于重置已有账号 |
| `SESSION_SECRET` | 首次生成并持久化，保持稳定 |
| `CONFIG_SECRET` | 首次生成并持久化，用于敏感配置和 Passkey 凭据加密，必须保留 |
| `TRUSTED_PROXIES` | 空，不信任转发身份；仅填写真实代理的 IP / CIDR |
| `IMAGE_MAX_PIXELS` | `40000000`，图像像素预算，只能降低 |
| `IMAGE_MAX_DIMENSION` | `16384`，单边尺寸上限，只能降低 |

上传开关、单文件大小、压缩、存储桶等界面设置保存在数据库中，不等同于上述进程配置；后台不能提高进程级图像预算。

修改配置后的启动方式也不同：

- 只修改持久化应用文件：`docker restart oneimg`，或 `docker compose restart oneimg`。
- 修改 Compose 插值文件 / `environment`：`docker compose up -d`，让 Compose 按新环境重建容器。
- 修改 `docker run -e` 参数：备份并停止旧容器，按原挂载和新参数重建。单纯 `docker restart` 不会更新容器环境变量。

不要为排查配置覆盖而输出完整 `.env` 或 `docker inspect` 环境列表到公开日志。

## 数据库选择

SQLite 仍为默认数据库。包含本次支持的源码构建可选择 PostgreSQL 16、17、18；上面的历史固定版本 `v4.4.1` 不包含此能力。PostgreSQL 首期只支持单个 OneImg 实例直连标准服务器、专用数据库和 `public` schema。连接池代理、多实例、读写分离及早期 PostgreSQL 数据库直接升级尚未支持。

| 配置 | 默认值 | 规则 |
| --- | --- | --- |
| `DB_TYPE` | `sqlite` | 只接受 `sqlite` / `postgres` |
| `DATABASE_URL` | 空 | `postgres://` / `postgresql://`，包含一个主机和数据库名 |
| `SQLITE_PATH` | `./data/data.db` | 仅 SQLite 使用 |
| `STATE_DIR` | 按数据库推导 | 保留头像、初始密码和恢复码 |
| `DB_MAX_OPEN_CONNS` | `10` | PostgreSQL 最大连接数，必须大于 0 |
| `DB_MAX_IDLE_CONNS` | `5` | 非负，不能超过最大连接数 |
| `DB_CONN_MAX_LIFETIME` | `30m` | 正持续时间 |
| `DB_CONN_MAX_IDLE_TIME` | `5m` | 正持续时间 |

SQLite 连接策略保持原有行为。PostgreSQL URL 的 `connect_timeout` 默认 5 秒，支持 1–300 秒；初次连接检查也受此限制。连接失败或结构迁移失败会停止启动，不能通过自动创建 SQLite 绕过失败。已有业务对象但没有受支持迁移标记的 PostgreSQL schema 会被拒绝，应按[停机迁移](upgrading.md#sqlite-迁移到-postgresql)重新准备空目标。

生产连接示意如下，密码中的保留字符须进行 URL 百分号编码：

```dotenv
DB_TYPE=postgres
DATABASE_URL=postgresql://oneimg:URL_ENCODED_PASSWORD@db.example.com:5432/oneimg?sslmode=verify-full&sslrootcert=/run/certs/ca.pem&connect_timeout=5
STATE_DIR=/app/data
```

未指定 `sslmode` 时采用 `verify-full`，验证 CA 和主机名。私有 CA 使用 `sslrootcert`；双向 TLS 可再配置 `sslcert`、`sslkey`，证书路径必须在应用容器内可读，私钥只向运行用户开放。仅显式 `sslmode=disable` 关闭 TLS，用于隔离开发网络；不支持 `require`、跳过主机名验证或旧的跳过证书校验开关。服务器必须使用 UTF-8 编码，应用角色需有 `public` 的建表、建索引和序列使用权限。

PostgreSQL 会话时区固定 UTC；统计日、周、月、年边界使用应用时区，因此切换时保留原 `TZ`。

### PostgreSQL Compose

使用[独立模板](../docker-compose.postgres.yml.example)，从包含支持的当前源码构建。模板固定已验证的 `postgres:17.11`，数据库只接入内部网络，不映射宿主机端口；应用同时接入可访问远端存储的网络。不要用旧 tag 的源码执行以下步骤：

```bash
cp docker-compose.postgres.yml.example docker-compose.yml
mkdir -p data uploads
```

在 Compose 插值文件中设置 `POSTGRES_PASSWORD`，并为 `DATABASE_URL` 填写同一密码的 URL 编码形式，例如 `postgresql://oneimg:URL_ENCODED_PASSWORD@postgres:5432/oneimg?sslmode=disable`。此模板中的显式 `disable` 仅适用于该隔离数据库网络；跨主机连接改为 `verify-full` 并挂载证书。按需要设置 `PUID`、`PGID`、公开地址与 Passkey 参数，再运行：

```bash
docker compose up -d --build
docker compose logs --tail 100 oneimg
```

数据库卷与 `data`、`uploads` 分别持久化。不要使用 `docker compose down -v` 删除仍需保留的数据库卷。已有 SQLite 部署不能只修改 `DB_TYPE` 后启动，应先完成迁移。

### PostgreSQL 备份与恢复

停止 OneImg 和其他写入进程，使数据库、配置和媒体处于同一停机点。使用该数据库版本的工具创建备份；Compose 示例：

```bash
mkdir -p backup
docker compose exec -T postgres pg_dump -U oneimg -d oneimg -Fc > backup/oneimg.dump
```

同时备份应用 `.env`、`STATE_DIR`、`data/thumbnails`、`uploads` 和远端媒体，限制备份权限。`pg_dump` 不包含服务器角色、TLS 文件或本地媒体。先在独立空数据库用 `pg_restore --exit-on-error` 验证恢复，再按原配置和文件路径启动隔离应用；不要向仍在服务的数据库覆盖恢复。备份、恢复时避免把连接密码放入命令历史或日志。

## 反向代理与可信地址

公网部署应使用 HTTPS，将 `APP_URL` 设置为实际访问的公开 Origin（协议、主机及非标准端口），而不是容器地址。限制后端端口的可达范围，让流量经预期入口进入。

`TRUSTED_PROXIES` 接受英文逗号分隔的明确 IP / CIDR；拒绝主机名、非法值和全网 CIDR，配置错误会在监听前终止启动。例如，仅在代理确实从应用所在网络命名空间的 loopback 连接时，可在持久化应用配置中写：

```dotenv
TRUSTED_PROXIES=127.0.0.1,::1
```

Docker 里的代理来源不一定是宿主机的 `127.0.0.1`，应按实际拓扑填写，不要直接照抄，不自动信任 Docker 网段或全部私网。入口还必须覆盖客户端自带的转发头，防止伪造来源影响限流。

代理应正确转发 `GET`、`HEAD` 和单段 `Range`，并保留媒体响应的安全头。使用独立 CDN、静态文件服务或存储直链时，应分别核对 Range 和[历史 SVG 响应策略](upgrading.md#历史-svg-出口与缓存)，不能认为应用内的保护会自动覆盖外部出口。

## Passkey

| 配置 | 默认值 | 要求 |
| --- | --- | --- |
| `APP_URL` | `http://localhost:8080` | 生产使用公开 HTTPS Origin |
| `PASSKEY_RP_ID` | 从 `APP_URL` 提取主机名 | 只写域名，不含协议和端口，长期保持稳定 |
| `PASSKEY_ORIGINS` | `APP_URL` 的 Origin | 多个精确 Origin 用英文逗号分隔，包含非标准端口，不含路径 |
| `PASSKEY_RP_NAME` | `OneImg` | 浏览器提示中的名称 |
| `CONFIG_SECRET` | 首次生成 | 保存在应用配置中；丢失或改变会导致已有加密数据无法读取 |

例如 `https://oneimg.example.com:8443` 是 Origin，RP ID 仍为 `oneimg.example.com`。`http://localhost:8080` 只适合本机测试，其 Passkey 不能直接迁移到生产域名。

新部署无需手工生成密钥。已有部署如果确实缺少 `CONFIG_SECRET`，先生成一个固定值，再安全写入持久化应用配置并备份；不要替换已有有效密钥：

```bash
openssl rand -base64 32
```

按[配置来源与优先级](#配置来源与优先级)重启后，启动日志会显示 Passkey 是否启用及禁用原因；配置无效时仍可使用密码登录。不要用轮换密钥来解决 Origin 或 RP ID 配置错误。

登录后在“账户设置”添加、重命名或撤销 Passkey。添加和撤销需当前密码，每个账号最多绑定 10 个；管理员在具备对应权限时可撤销其他用户的 Passkey。密码始终保留作为登录与恢复方式。
