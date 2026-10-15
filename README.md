# OneImg

[![Build](https://github.com/mikusaa/oneimg/actions/workflows/build.yaml/badge.svg?branch=main)](https://github.com/mikusaa/oneimg/actions/workflows/build.yaml)
[![GHCR 镜像](https://img.shields.io/badge/GHCR-ghcr.io%2Fmikusaa%2Foneimg-2496ED?logo=docker&logoColor=white)](https://github.com/mikusaa/oneimg/pkgs/container/oneimg)
![镜像架构](https://img.shields.io/badge/arch-amd64%20%7C%20arm64-475569?logo=linux&logoColor=white)
![Go 版本](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)
![Vue 版本](https://img.shields.io/badge/Vue-3-42B883?logo=vuedotjs&logoColor=white)
![SQLite](https://img.shields.io/badge/database-SQLite-003B57?logo=sqlite&logoColor=white)

OneImg 是面向个人图床和受控多账号使用的自托管媒体管理工具。本项目是基于 [onexru/oneimg](https://github.com/onexru/oneimg) 维护的个人部署分支，使用 Go、Vue 3 和 SQLite。

## 主要功能

- **上传与处理**：文件选择、拖拽、剪贴板图片和图片 URL 导入；批量上传、内容哈希去重、WebP 转换、压缩与缩略图。
- **视频**：可选开启 MP4 / WebM 原文件上传和站内播放，不转码、不依赖 FFmpeg。
- **存储**：本地、S3/R2、WebDAV 和 FTP；支持配置 CDN 或图片直链域名。
- **整理与分享**：标签、文件名 / URL / 哈希搜索、图片与视频筛选、批量复制多种外链格式，以及历史本地图片导入。
- **账号与集成**：多账号与细粒度权限、密码和 Passkey 登录、带 scope 的个人访问令牌（PAT）及 API v1。
- **界面**：适配桌面与移动端，支持浅色、深色和跟随设备主题。

## 快速启动

已安装 Docker 的新部署可直接使用固定版本镜像。以下命令在当前目录创建持久化目录，映射端口 `8080`：

```bash
mkdir -p data uploads

docker run -d \
  --name oneimg \
  --restart unless-stopped \
  -p 8080:8080 \
  -e PUID=1000 \
  -e PGID=1000 \
  -e TZ=Asia/Shanghai \
  -e APP_URL=http://localhost:8080 \
  -v "$(pwd)/data:/app/data" \
  -v "$(pwd)/uploads:/app/uploads" \
  ghcr.io/mikusaa/oneimg:v4.1.3
```

访问 `http://localhost:8080`。镜像支持 `linux/amd64` 和 `linux/arm64`；`data` 保存数据库、配置和本地缩略图，`uploads` 保存本地媒体主文件，两者都需要持久化和备份。

示例使用 `1000:1000` 作为文件属主；如需更换用户、使用 Compose、从源码构建或通过公网域名访问，请先阅读[部署指南](docs/deployment.md)。公网部署需设置实际的 `APP_URL`，配置 HTTPS 和可信代理。

## 首次登录

默认用户名为 `admin`，不再提供共享默认密码。上述新部署首次启动后，通过服务器读取随机初始密码：

```bash
docker exec oneimg cat /app/data/.initial_admin_password
```

首次登录必须修改密码，之后重新登录；即使显式设置了初始密码，也需要完成首次改密。初始密码文件权限为 `0600`，不要将其放入截图、日志或公开文档。

已有正常账号不会因升级被重置。历史默认密码账号的恢复流程见[升级指南](docs/upgrading.md#初始化与账号恢复)。

## 文档导航

| 你想做什么 | 文档 |
| --- | --- |
| 安装、配置持久化、反向代理或 Passkey | [部署指南](docs/deployment.md) |
| 上传、整理、复制外链、导入历史图片 | [使用指南](docs/usage.md) |
| 创建 PAT、调用接口、处理错误 | [API 指南](docs/api.md) |
| 备份、升级、恢复与检查兼容性变化 | [升级指南](docs/upgrading.md) |
| 本地开发、生成 SDK、测试与发布检查 | [开发指南](docs/development.md) |

## 升级与发布

升级前应备份一致的数据库、配置和媒体。升级至 v4.1.3 后需重新登录；改密或管理员重置会使旧会话和 PAT 失效。历史默认密码账号、可信代理、SVG 出口与缓存、图像限制及单实例运行要求，请按[升级指南](docs/upgrading.md)检查，不要直接用旧数据库撤销已完成的凭证撤销。

查看 [v4.1.3 发布说明](https://github.com/mikusaa/oneimg/releases/tag/v4.1.3)或[所有 Releases](https://github.com/mikusaa/oneimg/releases)。
