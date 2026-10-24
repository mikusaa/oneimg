# PostgreSQL 支持恢复评估

本文保留实施前的评估背景。当前源码的配置、部署和迁移步骤以[部署指南](deployment.md#数据库选择)、[升级指南](upgrading.md#sqlite-迁移到-postgresql)和[开发指南](development.md#双数据库测试)为准；文中的原始代码与测试状态对应评估时点。

评估日期：2026-10-22。代码基线：`a3cbb3e630fd93403ecc0b650647f19a80ca8827`。

**评估时状态：待实施，当时版本仅支持 SQLite；下文中的配置、迁移命令方向和工期均为实施前判断。** 当前操作步骤与支持范围见上方指南。

## 结论与范围

建议继续使用 GORM，保留 SQLite 为默认后端，增加 PostgreSQL 作为可选后端。现有数据库入口和服务层可复用，实施难度中等，主要成本是查询兼容、初始化、数据迁移及双数据库回归。

按一名熟悉项目的开发者估算，完成 PostgreSQL 新安装支持和回归约需 **4–6 人日**；增加当前 SQLite 数据的停机迁移工具后，合计约 **6–10 人日**。工期基于静态检查，尚未通过 PostgreSQL 实测收敛。

适合的使用场景是接入已有 PostgreSQL 运维体系、统一备份或使用托管数据库。上传性能收益需要单独测量，不能仅从数据库类型推断。

首期延续单实例部署约定。[存储与物理对象协调](../backend/services/object_identity.go)依赖进程内锁；多实例还需设计跨进程协调、本地文件和认证状态管理，应另行评估。

## 现有基础

- 历史提交 `3fc18c7240d137305f54dadab156891bf27ec975`（2026-07-29，收敛数据库支持为 SQLite）删除了 PostgreSQL 配置、驱动和连接代码，可用于理解历史行为。
- [数据库入口](../backend/database/database.go)的 `NewDB` 仍接收 `gorm.Dialector`，服务层主要使用 `*gorm.DB`，无需整体重写数据访问层。
- 当前使用 GORM `v1.31.2`。恢复支持时应选择兼容的 PostgreSQL 驱动并锁定依赖，不直接恢复历史旧 pgx 版本，也不顺带升级无关依赖。
- 表结构由 `AutoMigrate` 维护，业务数据兼容迁移分布在 `backend/app/` 和存储计量服务中；这些路径都需在 PostgreSQL 上验证。

## 适配清单

| 领域 | 当前实现与风险 | 实施方向 |
| --- | --- | --- |
| 配置与连接 | [配置](../backend/config/config.go)和 `InitDB` 仅支持 SQLite；历史配置测试还明确排除已移除的数据库变量 | 增加后端选择、连接校验、超时和连接池限制，同步默认模板与测试；连接错误不得静默回退到 SQLite |
| 时间统计 | [统计服务](../backend/services/stats.go)的 `countBetween` 使用 SQLite 专属 `julianday()`，且忽略 `.Count` 的错误；直接接 PostgreSQL 可能将查询失败表现为零 | 集中封装时间范围查询；PostgreSQL 使用时间参数比较，SQLite 保留历史时区兼容；向上传递统计查询错误 |
| 图库分页 | [图库查询](../backend/services/images.go)只选择 `DISTINCT images.id`，却允许按创建时间、大小和文件名排序，不满足 PostgreSQL 对 DISTINCT 排序表达式的要求 | 调整为两种数据库都适用的查询；验证所有排序、筛选、分页及同值排序稳定性 |
| 初始化与序列 | [默认存储初始化](../backend/app/Init.go)显式插入 ID `1`，[历史存储迁移](../backend/app/Migrate.go)也写入固定 ID；迁入数据需要保留原 ID | 正确同步相关 PostgreSQL 序列，覆盖空表、已有数据、重复启动及导入后下一次自动插入；同步不能回退已推进的序列 |
| 本地状态目录 | [认证初始化](../backend/app/Authentication.go)和[服务装配](../backend/services/services.go)从 `SqlitePath` 推导初始密码、恢复码及头像目录 | 使用独立的本地状态目录解析；默认兼容现有 SQLite 路径，PostgreSQL 默认使用 `./data`，仍需持久化本地状态和媒体 |
| 搜索语义 | 图库和[用户搜索](../backend/services/users.go)使用 `LIKE`，两种数据库的大小写匹配行为存在差异 | 明确大小写、通配符和转义规则；如 PostgreSQL 使用 `ILIKE`，验证 locale 及与现有行为的兼容范围 |
| 字段与约束 | [权限](../backend/models/User.go)使用自定义 JSON 扫描，[PAT](../backend/models/PersonalAccessToken.go)作用域和[存储配置](../backend/models/Buckets.go)也有序列化；容量使用 `uint64` | 验证 JSON、空值、布尔默认值、时间精度、数值范围和唯一约束；统一重复键错误识别，避免依赖错误字符串 |
| 测试与 CI | [服务测试](../backend/services/service_test.go)、数据库测试和[浏览器 fixture](../internal/securityfixture/main.go)使用 SQLite；现有工作流没有 PostgreSQL 服务 | 抽出可选择数据库的测试初始化，隔离每组测试数据，建立真实 PostgreSQL 回归；保留 SQLite 覆盖 |

## 实施方案

### 第一阶段：新安装支持

1. 增加数据库选择。可采用 `DB_TYPE=sqlite|postgres`，PostgreSQL 使用一个连接配置入口，例如 `DATABASE_URL`。这些名称属于待定设计，SQLite 默认值和已有配置继续兼容；未知类型直接报错。
2. 新增兼容当前 GORM 的 PostgreSQL 驱动，补齐连接超时、连接池上限和日志脱敏。解析及连接错误均不得输出密码或完整 DSN。
3. 集中处理时间查询、搜索与序列差异，修改图库分页查询。保持业务服务复用，避免散布后端判断。
4. 验证八个现有模型的新建、重复启动和结构迁移。数据修复、固定 ID 与序列处理使用显式且可重复执行的步骤。
5. 配置远端 TLS，验证 pgx 实际行为。远端部署采用证书和主机名校验，例如正确配置 `verify-full`；开发容器的连接方式单独说明，不继承旧实现中的证书跳过逻辑。
6. 选择并声明经过 CI 验证的 PostgreSQL 主版本，补充独立 Compose 示例。用户配置与备份写入[部署指南](deployment.md)，升级与回滚写入[升级指南](upgrading.md)，开发和测试命令写入[开发指南](development.md)。

预计主要修改后端、测试、CI 和文档，通常无需调整前端功能或公共 API。如果实际实现改变接口请求、响应或错误契约，仍须同步 OpenAPI、生成客户端及回归。

### 第二阶段：当前 SQLite 停机迁移

提供独立迁移入口：源库只读，目标数据库必须为空，任一前提不满足即停止；保留所有业务 ID 和关联关系，完成验证后才允许切换。

[当前程序入口](../main.go)在命令分流前执行 `app.Init()`，会初始化管理员、设置和默认存储。数据库迁移入口应在常规应用初始化之前分流，避免提前污染目标库。

迁移需要满足以下条件：

- 停止应用和其他写入进程，备份一致的数据库、配置、本地状态及媒体；迁移过程不操作生产存储对象。
- 明确保留 `false`、`0`、空字符串及 `NULL`，避免模型默认值改变原始设置。
- 保留用户、媒体、存储、标签、PAT 和 Passkey 数据。认证版本、撤销状态和受限账号状态沿用既有兼容规则，不恢复已撤销凭据。
- 携带原有 `CONFIG_SECRET` 和必要配置，保留加密字段；迁移及校验输出不泄露密钥、哈希或其他敏感字段。头像、缩略图和本地媒体另行保留。
- 校验表行数、关键字段、标签关联、资源归属、媒体 URL、共享对象引用和存储计量；同步序列后验证下一次自动插入。
- 明确失败恢复与切换边界。保留旧 SQLite 备份只能恢复到切换前状态；PostgreSQL 接受新写入后，回滚还需处理新增数据。

迁移工具的具体命令和配置格式在实现时确定，本文不提供可直接执行的迁移操作。

### 可选阶段：早期 PostgreSQL 升级

早期 PostgreSQL 数据库直接升级与当前 SQLite 迁入是两条独立路径。需要先声明支持的历史版本，再基于对应结构建立夹具，验证权限、认证撤销、存储结构与计量迁移；不能仅以新库成功启动推断旧库兼容。

## 工作量估算

以下为有效开发人日，不含部署排期，也不承诺上线日期。

| 工作 | 估算 |
| --- | --- |
| 配置、驱动、连接池及本地状态目录 | 0.5–1 人日 |
| 查询、初始化、类型及序列适配 | 1–2 人日 |
| PostgreSQL 服务/API 回归、CI 和部署文档 | 2–3 人日 |
| 新安装支持合计 | 约 4–6 人日 |
| SQLite 停机迁移工具与验收 | 额外 2–4 人日，累计约 6–10 人日 |
| 早期 PostgreSQL 直接升级兼容 | 额外约 1–3 人日，取决于承诺支持的历史版本 |

建议按第一阶段、第二阶段分别验收。多实例支持、在线迁移、双写切换和性能优化需要单独评估。

## 验收与验证边界

实施后的核心验收范围：

- SQLite 原有配置、新安装和历史数据兼容。
- PostgreSQL 首次建库、重复启动、迁移失败中止、固定 ID 和序列；指定历史版本的升级夹具另行验证。
- 图库全部排序、搜索、标签和用户范围筛选、分页；统计的时区、日/月边界、图片/视频和空数据。
- 密码、PAT、Passkey、认证撤销与受限恢复；数据库切换后的头像和本地状态目录。
- 上传、配额条件更新、标签事务、失败清理、共享对象保护、存储更新/删除及本地导入。
- 迁移的空目标检查、零值/空值、数据关联、序列、重复执行防护和切换恢复。

自动测试使用临时数据库、目录和隔离容器。真实远端存储与目标部署仍按[开发指南](development.md#真实远端存储验证)和[升级指南](upgrading.md#部署环境验收)单独验收，自动测试不替代部署验证。

本轮已完成源码、历史提交和官方资料核对，未修改运行时代码，未运行 Go、浏览器或 PostgreSQL 集成测试。Docker 只读镜像查询因自动审批检查连接中断而未执行，因此以上问题属于静态分析结论，尚无 PostgreSQL 实测结果。

## 参考资料

- [GORM 数据库连接与 PostgreSQL 驱动](https://gorm.io/docs/connecting_to_the_database.html)
- [GORM 迁移与 AutoMigrate 范围](https://gorm.io/docs/migration.html)
- [SQLite 日期函数与 julianday](https://www.sqlite.org/lang_datefunc.html)
- [PostgreSQL DISTINCT 排序检查源码](https://doxygen.postgresql.org/parse__clause_8c_source.html)
- [PostgreSQL 序列函数](https://www.postgresql.org/docs/current/functions-sequence.html)
- [PostgreSQL 模式匹配](https://www.postgresql.org/docs/current/functions-matching.html)
- [PostgreSQL TLS 校验模式](https://www.postgresql.org/docs/current/libpq-ssl.html)
