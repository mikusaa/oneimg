# picgo-plugin-oneimg

OneImg 的 PicGo 3.x 本地插件，支持上传图片，并可选择在删除 PicGo 图库记录时同步删除 OneImg 原图。

## 安装

1. 打开 PicGo 3，进入插件设置，选择“安装本地插件”，选择本目录。
2. 在图床设置中选择 `OneImg` 并填写配置。

仓库已提交自包含的 `dist/index.cjs`，正常安装不需要先运行 `pnpm install` 或 `pnpm build`。只有修改插件源码时才需要安装开发依赖并重新构建。

插件不发布 npm，也不进入 OneImg Docker 镜像；它只服务于当前仓库的自用部署。

## 配置

- **OneImg 地址**：填写实例根地址，例如 `https://img.example.com`；填写 `/api/v1` 结尾的地址也可以。
- **个人访问 Token**：在 OneImg 账户设置创建 PAT。上传至少需要 `images:write`，同步删除还需要 `images:delete`。
- **存储桶 ID**：可选，留空使用 OneImg 默认存储桶。
- **标签 ID**：可选，多个 ID 使用英文逗号分隔，例如 `2,3`。
- **同步删除远端图片**：默认关闭。开启后，删除 PicGo 历史记录会调用 OneImg 删除接口。
- **请求超时**：5 到 600 秒，默认 120 秒。

建议使用只包含 `images:write`/`images:delete` 的专用、短期 Token。PicGo 会把配置文件中的 Token 明文保存，密码输入框只负责界面遮挡。

“测试 OneImg 连接”菜单会调用 `/me` 和 `/upload-options`，显示账户、单文件和单批上传限制、允许的 MIME 类型、默认桶及可用桶。

## 删除行为

上传成功后，插件会在 PicGo 历史项中保存 OneImg 的 `imageId`、API 地址和配置 ID，不保存 Token。删除时只处理插件自己生成且包含有效 `imageId` 的历史项，不会根据 URL 猜测图片 ID。

PicGo 会先删除本地历史项，再触发 `remove` 事件。因此远端删除失败时本地记录不会恢复；插件会在 GUI 中通知错误并在日志中记录服务器、图片 ID、状态码和 `request_id`。HTTP 404 会视为图片已经删除。

HTTP 401 表示 Token 已失效或不正确；HTTP 403 通常表示 Token 缺少 `images:delete` scope；HTTP 502 表示 OneImg 删除数据库记录前未能删除物理存储对象，通知和日志会保留 `request_id` 供排查。插件不建立删除重试队列。

每条新历史记录都绑定 PicGo 配置 `_id`。存在多个 OneImg 配置时，删除会优先使用上传时的配置；配置已被删除时，只会在当前活动配置的 API 地址与历史记录完全一致时回退，避免跨实例误删。

旧版 `web-uploader` 等通用插件生成的历史项没有 OneImg 图片 ID，不会参与远端删除。

## 开发

```bash
pnpm install --frozen-lockfile
pnpm typecheck
pnpm test
pnpm build
```

插件按每批最多 10 张图片调用 `POST /api/v1/images`，使用 `images`、`tag_ids` 和可选 `bucket_id` multipart 字段。批次中的部分失败会保留失败项数据并报告汇总；不会自动重试上传。

网络中断时服务器是否已收到上传无法可靠判断，因此插件不会自动重试。用户可手工重新上传，OneImg 会通过内容去重返回已有图片，避免重复占用存储。PicGo Core CLI 仅支持上传；联动删除依赖 GUI 的图库 `remove` 事件。
