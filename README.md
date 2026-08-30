# picgo-plugin-oneimg

把 PicGo 或 PicList 上传的图片存到 OneImg。支持批量上传、指定存储桶和标签，也可以在删除本地历史时同步删除 OneImg 原图。

插件只支持 PicGo/PicList 3.x，通过本地目录安装。

## 安装

```bash
git clone --branch picgo-plugin-oneimg --single-branch \
  https://github.com/mikusaa/oneimg.git picgo-plugin-oneimg
```

### PicList

1. 打开“插件”，点击“导入本地插件”。
2. 选择刚克隆的 `picgo-plugin-oneimg` 目录。
3. 在“图床”中添加 `OneImg` 配置。

### PicGo

1. 打开“插件设置”，选择“安装本地插件”。
2. 选择刚克隆的 `picgo-plugin-oneimg` 目录。
3. 在“图床设置”中配置 `OneImg`。

请选择包含 `package.json` 的插件根目录，不要选择 `dist`。仓库已经带有构建产物，正常安装不需要运行 `pnpm install`。

## 配置

- **OneImg 地址**：实例地址，例如 `https://img.example.com`。填写到 `/api/v1` 也可以。
- **个人访问 Token**：在 OneImg 的账户设置中创建。上传需要 `images:write`，同步删除还需要 `images:delete`。
- **存储桶 ID**：可选；留空时使用 OneImg 的默认存储桶。
- **标签 ID**：可选；多个 ID 用英文逗号分隔，例如 `2,3`。
- **同步删除远端图片**：默认关闭。
- **请求超时**：默认 120 秒，可填写 5 到 600。

插件菜单中的“测试 OneImg 连接”可以检查地址和 Token，并查看当前实例的上传限制。

PicGo 和 PicList 都会把 Token 明文保存在本地配置中，PicList 3.5.0 的设置页面也不会遮挡 Token。请为插件单独创建 Token，不要复用账号密码或其他凭据。

## 同步删除

开启后，从 PicGo/PicList 历史记录中删除图片时，插件会同时删除 OneImg 中的原图。

PicGo/PicList 会先移除本地历史，再请求 OneImg 删除原图。如果远端删除失败，本地历史不会自动恢复，请根据通知和日志手动处理。旧版通用上传插件产生的历史记录没有 OneImg 图片 ID，不会触发同步删除。

## 开发

```bash
pnpm install --frozen-lockfile
pnpm typecheck
pnpm test
pnpm build
```

构建产物为 `dist/index.cjs`，提交前请一并更新。
