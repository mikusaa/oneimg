# OneImg

基于 [onexru/oneimg](https://github.com/onexru/oneimg) 维护的个人部署分支，面向个人图床和受控多账号使用。

## 主要功能

- 支持本地文件、剪贴板、拖拽和图片 URL 上传，可批量处理上传任务
- 支持本地、S3/R2、WebDAV 和 FTP 存储；本地可配置 CDN 域名，S3/R2 可配置图片直链域名
- 支持 WebP 转换、主图压缩、缩略图生成和动态 WebP 尺寸识别
- 使用内容哈希跳过重复图片，记录原始文件名并支持文件名、URL 和哈希搜索
- 支持标签筛选与管理、批量复制 URL/Markdown/HTML/BBCode，以及历史本地图片导入
- 支持多账号、开放注册和管理员细粒度权限，数据统一存储在 SQLite
- 支持密码与 Passkey 登录，一个账号可绑定多个 Passkey
- 支持浅色、深色和跟随设备主题，可在桌面端和移动端使用

## 默认账号

- 用户名：`admin`
- 密码：`123456`

首次登录后请立即修改默认密码。

## Docker 部署

### 环境要求
- Docker 20.10.0 或更高版本
- Docker Compose v2.0.0 或更高版本

### 使用 Docker Compose 部署

1. **克隆项目**
```bash
git clone https://github.com/mikusaa/oneimg.git
cd oneimg
```

2. **创建 Compose 配置**

```bash
cp docker-compose.yml.example docker-compose.yml
```

3. **启动服务**

```bash
docker compose up -d --build
```

4. **访问系统**

- `http://localhost:8080`

5. **停止服务**

```bash
docker compose down
```

### 直接使用镜像

```bash
docker run -d \
  --name oneimg \
  --restart unless-stopped \
  -p 8080:8080 \
  -e TZ=Asia/Shanghai \
  -e APP_URL=http://localhost:8080 \
  -v /data/oneimg/data:/app/data \
  -v /data/oneimg/uploads:/app/uploads \
  ghcr.io/mikusaa/oneimg:latest
```

GHCR 镜像同时提供 `linux/amd64` 和 `linux/arm64` 架构。

### 数据持久化

系统数据和上传的图片通过 Docker 数据卷保持持久化：

- 上传的图片存储在 `./uploads` 目录
- SQLite 数据库和应用配置存储在 `./data` 目录

备份时应同时备份 `data` 和 `uploads`。其中 `data/.env` 内的 `CONFIG_SECRET` 用于加密 Passkey 凭据和其他敏感配置，丢失或改变后已有加密数据将无法读取。

### 导入历史图片

`import-local` 可以将 `uploads` 中已有的图片登记到图库。建议先执行预览：

```bash
docker compose run --rm oneimg ./main import-local --root /app/uploads --dry-run
```

确认统计结果后正式导入：

```bash
docker compose run --rm oneimg ./main import-local --root /app/uploads
```

默认归属用户和存储桶均为 `1`，图片时间取文件修改时间。可使用 `--user-id`、`--bucket-id`、`--date-source` 和 `--update-existing-date` 调整；导入过程不会移动或重命名原图。

### 自定义配置

Compose 模板会从项目根目录的 `.env` 读取变量并传入容器。例如生产环境可创建以下配置：

```dotenv
APP_URL=https://oneimg.example.com
PASSKEY_RP_ID=oneimg.example.com
PASSKEY_ORIGINS=https://oneimg.example.com
PASSKEY_RP_NAME=OneImg
```

项目根目录的 `.env` 用于 Docker Compose 变量替换；容器内持久化的 `data/.env` 是 OneImg 自身配置。首次启动会自动创建 `data/.env`，并生成稳定的 `SESSION_SECRET` 和 `CONFIG_SECRET`。

不要在 Compose 中传入空的 `CONFIG_SECRET`，否则它会覆盖 `data/.env` 中的持久化密钥并禁用 Passkey。已有部署如果尚无 `CONFIG_SECRET`，应先运行以下命令生成一个固定值，再将结果写入 `data/.env`：

```bash
openssl rand -base64 32
```

修改配置后重启容器：

```bash
docker compose up -d
```

### Passkey 登录

Passkey 登录使用以下环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_URL` | `http://localhost:8080` | 对外访问地址；生产环境必须使用 HTTPS |
| `PASSKEY_RP_ID` | 从 `APP_URL` 提取 | Passkey 绑定的域名，修改后旧凭据将无法使用 |
| `PASSKEY_ORIGINS` | `APP_URL` 的 Origin | 允许的来源，多个来源使用英文逗号分隔 |
| `PASSKEY_RP_NAME` | `OneImg` | 系统在 Passkey 提示中的名称 |
| `CONFIG_SECRET` | 首次启动随机生成 | Passkey 凭据加密密钥，保存在 `data/.env`，必须保持不变 |

使用 Passkey 前需满足以下条件：

- 生产环境必须通过 HTTPS 访问，`APP_URL` 必须填写浏览器实际访问的公开地址，而不是容器地址。
- `APP_URL` 和 `PASSKEY_ORIGINS` 必须是精确的 Origin，包含协议以及非标准端口，不能包含路径。
- `PASSKEY_RP_ID` 只填写域名，不包含协议和端口；留空时从 `APP_URL` 自动推导。
- 反向代理必须对外提供与配置一致的 HTTPS 地址。
- RP ID 应长期保持不变。修改为其他根域后，已经注册的 Passkey 将不能继续使用。

`http://localhost:8080` 可用于本机测试；在 localhost 注册的 Passkey 不能用于生产域名。

配置有效时，启动日志会显示：

```text
Passkey 功能已启用 (RP ID: oneimg.example.com)
```

配置或密钥无效时，密码登录不会受影响，启动日志会显示 `Passkey 功能已禁用` 及具体原因。

登录后可在“账户设置”中添加、重命名和删除 Passkey。添加和删除时需要验证当前密码，每个账号最多绑定 10 个 Passkey；密码始终保留作为恢复方式。登录页会在服务端配置有效且浏览器支持 WebAuthn 时显示“使用 Passkey”。具有对应权限的管理员也可以在用户管理页撤销其他用户的全部 Passkey。
