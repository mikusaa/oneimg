# 初春图床系统

一个功能完整的现代化图床管理系统，基于 Vue.js 3 + Go 构建，支持剪贴板上传等功能。

## 开发者

- [onexru](https://github.com/onexru)
- [雾创岛](https://www.tr0.cn)
- [打赏赞助](https://www.cv0.cn/donate)
- [QQ群](https://qm.qq.com/q/lzT9IDkKVG)

## API文档
- [API文档](https://www.tr0.cn/oneimgapi/)

## Demo
[初春图床v3.0](https://www.ip6s.com)

## 预览
![18bce5ad46f261fb288.webp](https://eta.im/uploads/2026/06/18bce5ad46f261fb288.webp)
![18bce5d81ad53298181.webp](https://eta.im/uploads/2026/06/18bce5d81ad53298181.webp)
![18bce5e035f58315808.webp](https://eta.im/uploads/2026/06/18bce5e035f58315808.webp)
![18bce6194f707b25401.webp](https://eta.im/uploads/2026/06/18bce6194f707b25401.webp)
![18bce624c55e939e703.webp](https://eta.im/uploads/2026/06/18bce624c55e939e703.webp)

**默认账号密码**

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

## 功能特性

### 多存储支持
- 本地存储
- S3 兼容存储（R2、OSS等）
- WebDAV 存储
- FTP 存储

### 安全认证
- Session 会话管理
- 密码加密存储
- Passkey 登录与多设备凭据管理
- 会话超时保护

### 图片上传
- **剪贴板粘贴直接上传** - 支持 Ctrl+V 粘贴上传
- 拖拽上传支持
- 批量文件选择上传
- 支持多种图片格式 (JPEG, PNG, GIF, WebP, SVG, BMP)
- 文件大小限制和格式验证
- 上传进度显示

### 图片管理
- 图片预览和详情查看
- 复制链接功能
- 图片信息展示

### 数据统计
- 仪表板概览
- 存储空间统计
- 实时数据更新

### 用户界面
- 现代化设计风格
- 响应式布局 (支持移动端)
- 深色/浅色主题
- 流畅的动画效果
- 直观的操作体验
