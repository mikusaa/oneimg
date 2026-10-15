# 九项安全修复的部署与验收

## 上线前

1. 备份一致的 SQLite、持久化配置及本地媒体。远端对象使用运营者已有备份机制；应用不访问生产存储执行测试。
2. 在备份副本验证启动迁移。新增用户认证版本、受限状态和挑战哈希，新增 PAT 认证版本；迁移幂等，失败停止启动。正常历史 PAT 保留，缺少版本的旧浏览器会话需要重新登录。
3. 启动会报告历史共享物理对象数量，不自动复制、移动、重命名或删除。已经同名覆盖的原内容只能从备份恢复。
4. 默认单实例。不要让新旧版本同时写同一数据库或存储；对象引用锁是进程内协调，不提供跨实例互斥。
5. 发布前运行本文全部自动检查，完成真实隔离存储条件创建验收、目标容器内存测量，并确认每个历史 SVG 出口的响应策略和缓存清理。未完成安全验收不得发布。

## 初始化、恢复与凭证

- `DEFAULT_PASS` 默认为空。新安装生成 32 字节随机秘密并安全落盘至数据库目录 `.initial_admin_password`，权限 `0600`。显式配置非默认初始密码也必须首次改密。已有正常账户不会重置。
- 历史 ID 1 密码匹配 `123456` 时，启动建立 `.admin_recovery_code`，数据库仅存 SHA-256 摘要。有效挑战重启不变；文件缺失、损坏或权限不安全时重建，失败停止启动。
- 受限账户只可用浏览器会话读取/修改自身账户或退出。恢复必须提供当前密码、新密码和本地校验码。不能使用旧默认密码自行恢复管理权限或注册 Passkey。
- 成功恢复同时更改密码、消耗挑战、递增版本、撤销所有 PAT 并删除历史 Passkey；之后重新登录和注册可信 Passkey。本地恢复文件在提交后尽力删除，即使删除失败也不能重放。
- 普通改密及管理员重置会撤销目标用户全部浏览器会话和 PAT；普通改密保留 Passkey。只改用户名不批量撤销 PAT。改密接口限每用户 5 次/5 分钟。
- PicGo 协议未改变，插件源码未修改。改密/重置后的旧 PAT 失效，需要创建新 PAT 并重新配置客户端。
- 不轮换 `SESSION_SECRET`、`CONFIG_SECRET` 或已有存储凭证。不要通过恢复旧数据库撤销已完成的密码修改或令牌撤销，否则可能重新激活攻击者凭证。

## 可信代理与限流

`TRUSTED_PROXIES` 是逗号分隔的明确 IP/CIDR，默认为空，不信任转发身份。拒绝主机名、非法地址及全网 CIDR，错误配置在监听前失败。

```dotenv
# 示例仅适用于反向代理实际位于本机的部署，不要直接套用到其他拓扑。
TRUSTED_PROXIES=127.0.0.1,::1
IMAGE_MAX_PIXELS=40000000
IMAGE_MAX_DIMENSION=16384
```

只填写真实入口地址范围。入口必须覆盖客户端转发头，不能盲目透传；不自动信任 Docker 网段、私网或所有地址。限流表每分钟清理过期窗口，最多保留 10,000 个身份；满表且无过期项时拒绝新身份，返回 `429`，不淘汰有效窗口。

## 上传、图像预算与对象身份

- 新上传仅支持真实 JPEG/PNG/GIF/WebP 栅格配置。禁止 SVG 及伪装 SVG，禁止解码失败保存未知字节。普通上传、URL 导入和本地导入都遵循此策略。
- 像素预算默认 40,000,000，单边默认 16,384，仅可通过进程配置降低。数据库后台设置不能提高预算；全局处理并发固定为 1，繁忙立即返回可重试错误，不无限排队。
- 原图模式、跳过压缩及有效动画 WebP 均需通过配置/尺寸验证。有效动画保留原字节；无尺寸或截断容器不再接受。
- 预算在完整解码前验证，不将 goroutine 超时当作 native 转换的强制终止。像素和并发限制不是进程级硬内存隔离，也不是完整 DoS 防护。
- 原名仍作元数据保留；新对象名称包含 128 位随机身份，主图和缩略图共用身份。可读部分最多 80 UTF-8 字节，目录模板中的文件名变量也只能成为单个安全名称片段。
- 本地使用排他创建，S3/R2 和 WebDAV 使用 `If-None-Match: *` 条件创建，冲突最多尝试三次。不支持条件创建的后端必须拒绝，不能降级覆盖。S3 兼容服务是否真正执行条件语义需要隔离验收。
- FTP 使用随机名称并列目录确认不存在后写入，无法确认时拒绝。这不是原子条件写，不能宣称与本地/S3/WebDAV 相同保证。
- 配额或数据库事务失败只清理本次明确成功创建的对象。网络写入结果不确定时记录待核查孤儿 key，不盲删；监控并人工核查这些记录。
- 删除前按物理身份检查其他记录的主图与缩略图交叉引用。共享对象只删除当前记录和关联，计量继续按记录口径；最后引用删除时才清理物理对象。删除整个桶也适用。
- 本地导入保留原始主文件 URL，新生成缩略图使用独立随机 key 和排他创建。原始文件和缩略图均不因新导入同名文件被覆盖。

## 历史 SVG 的所有出口

应用不删除历史 SVG，也不改变其 URL。应用代理的 GET、HEAD、部分响应及缓存验证响应使用：

```text
Content-Type: image/svg+xml
X-Content-Type-Options: nosniff
Content-Security-Policy: sandbox; default-src 'none'; script-src 'none'; style-src 'unsafe-inline'; img-src data:; object-src 'none'; base-uri 'none'; form-action 'none'
Cache-Control: no-cache, max-age=0, must-revalidate
```

普通内联图形继续展示。外部脚本、字体、图片、表单、嵌入页面不保证原表现。

若使用 `CDNDomain`、`PublicImageDomain`、静态服务器、对象存储直链或其他绕过应用的出口，必须在该出口配置等价策略并清除已有 SVG 缓存。应用不能替外部服务补头；浏览器中已有长期缓存的旧响应仍有过渡风险，不能声称只升级应用就立即撤销旧缓存执行能力。

## 自动与隔离验证

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run openapi:check
pnpm run build
cd ..
go test ./...
go test -race ./...
go vet ./...
cd frontend
pnpm exec playwright install --with-deps chromium firefox webkit
pnpm run test:security
```

干净检出时先构建前端，再执行 Go 检查；`main.go` 使用 `go:embed` 嵌入 `frontend/dist`，不能依赖开发机上残留的产物。

浏览器测试使用 `internal/securityfixture`：仅监听 `127.0.0.1:18089`，临时 SQLite/媒体，固定测试账号，禁止作为生产入口。用真实前端构建验证 Home/Gallery 恶意历史元数据、预览操作、移动端和历史 SVG。不会访问生产凭证或真实远端。

真实隔离 bucket 需经明确授权另行验证：创建新对象、同 key 条件创建冲突后字节保持原样、Range/HEAD、失败清理与未知网络结果核查。不要使用生产 bucket。

在目标容器用正常 24MP/40MP 图像分别记录配置、输入/输出大小、峰值 RSS/cgroup `memory.peak` 和是否触发 OOM。测量值绑定镜像、架构和容器限制；不要将开发机或另一镜像的数字当作生产保证。

### 无外网的 Linux 容器验证

`Dockerfile.security` 复用生产 Dockerfile 的构建和运行库，并固定与测试依赖一致的 Playwright 版本。以下镜像仅供本地验证，不推送；重新验证时必须从当前候选源码重建三个基础镜像，不能混用旧标签内容。

```bash
docker build -t oneimg:security-check-20261015 .
docker build --target backend-builder -t oneimg:security-backend-20261015 .
docker build --target frontend-builder -t oneimg:security-frontend-20261015 .
docker build -f Dockerfile.security --target fixture -t oneimg:security-fixture-20261015 .
docker build -f Dockerfile.security --target browsers -t oneimg:security-browsers-20261015 .
docker build -f Dockerfile.security --target memory-probe -t oneimg:security-memory-20261015 .
```

浏览器与临时服务共享无外网的网络命名空间，只通过容器内 loopback 通信；不映射宿主端口，不挂载生产数据。容器名称必须未被占用；测试后只清理本次创建的容器。

```bash
docker run -d --name oneimg-security-fixture-20261015 \
  --network none --memory 512m oneimg:security-fixture-20261015
docker run --rm --network container:oneimg-security-fixture-20261015 \
  --shm-size=1g --memory 2g oneimg:security-browsers-20261015
docker stop oneimg-security-fixture-20261015
docker rm oneimg-security-fixture-20261015
```

内存探针仅接受正常 24MP 或 40MP JPEG 输入，在 Linux 运行库中执行真实解码、主图处理及缩略图生成；输出 JSON 含输入/输出大小、处理耗时、峰值 RSS、cgroup 峰值和 Go/CPU 架构。以下资源限制只是可重复的测试条件，不代表生产配置建议，也不代表最坏输入的内存上界。运行环境需支持 cgroup v2 的 `memory.peak`。

```bash
docker run --rm --network none --memory 2g --cpus 2 \
  oneimg:security-memory-20261015 -mp 24
docker run --rm --network none --memory 2g --cpus 2 \
  oneimg:security-memory-20261015 -mp 40
```

## 发布后与回滚

关注 `405`、`password_change_required`、无效旧凭证、SVG/格式拒绝、尺寸超限、处理忙碌、对象冲突及孤儿清理日志；日志不得记录密码、完整认证头或解密配置。

功能回滚只能使用保留安全边界的构建，不能直接重新暴露原漏洞版本，也不能用旧数据库恢复已撤销凭证。慢请求体专项、独立图像进程隔离、依赖 CVE 评估、远端密钥轮换及上游功能合并不在本次范围内；本次不宣称消除全部 DoS 或完成全仓安全认证。
