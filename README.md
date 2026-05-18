# Openxhh

Openxhh 是一个面向小黑盒的 AI 自动回复机器人，基于 OpenAI 兼容接口生成回复，并提供 Windows 图形化控制台，适合个人在 Windows 或 VPS 上自用部署。

本项目在原版自动回复基础上强化了上下文理解能力：当小黑盒接口返回帖子和评论楼层数据时，AI 不只看到当前 @ 内容，还会读取帖子正文、帖子图片、当前评论楼层文字和楼层图片，从而更准确地处理“这个怎么样”“楼上说得对吗”这类依赖上下文的问题。

## 主要功能

- 自动检查白名单用户的 @ 消息，并调用 OpenAI 兼容接口回复。
- 支持自定义 AI 接口、模型、token 和 prompt。
- 支持小黑盒扫码登录，登录态保存到 `cookie.json`。
- 支持 SQLite / PostgreSQL；个人部署推荐 SQLite。
- 支持 Windows 图形化控制台：配置编辑、扫码登录、启动/停止、日志查看和开机自启。
- 支持评论区 @ 生图触发词：`生图`、`画图`、`生成图片`。
- 支持把生成图片写入 VPS 静态目录，并通过 `imgs=<图片URL>` 发布顶级带图评论。
- 保留小黑盒 COS 上传实验路径；当前推荐优先使用 `image.uploadMode=external`。
- 保留原版 `config.json`、`cookie.json`、`sql.db` 工作方式，方便从旧部署升级。

## 上下文能力

AI 回复时会尽量合并以下信息：

- 帖子标题和正文；
- 帖子正文图片；
- 当前评论楼层文字上下文；
- 当前评论楼层图片，最多附带 4 张；
- 当前 @ 内容和触发用户信息。

评论楼层上下文依赖小黑盒接口返回的 `comments` 字段。部分帖子或楼层接口可能不返回评论区数据，此时机器人仍会正常回复，但只能基于帖子正文和当前 @ 内容判断。

## 推荐安装方式：Windows 图形化安装包

Windows 用户推荐下载 Release 中的 `Openxhh-Setup-x64.exe`，双击安装后从桌面快捷方式打开本地控制台。

安装包会放置：

```text
C:\Program Files\Openxhh\Openxhh.exe
C:\Program Files\Openxhh\Openxhh-webui.exe
C:\ProgramData\Openxhh\config.json
C:\ProgramData\Openxhh\cookie.json
C:\ProgramData\Openxhh\sql.db
C:\ProgramData\Openxhh\log\
```

首次使用流程：

1. 启动 Openxhh 控制台。
2. 保存页面显示的随机控制台密码，并登录本地控制台。
3. 在配置向导中填写小黑盒 UID、AI 接口、模型和 token。
4. 点击“扫码登录”，使用小黑盒 App 扫码。
5. 日志提示 Cookie 已保存后，点击“启动”。

更多 Windows 说明见 [docs/windows.md](docs/windows.md)。

## 快速升级已有 Linux / VPS 部署

如果你已经按原版方式部署在 `/opt/Openxhh`，并且已有：

```text
/opt/Openxhh/config.json
/opt/Openxhh/cookie.json
/opt/Openxhh/Openxhh
```

可以使用更新脚本升级二进制，原有 `config.json`、`cookie.json`、`sql.db` 和日志会保留：

```bash
curl -fsSL https://raw.githubusercontent.com/Www8881313/Openxhh/main/scripts/update-installed.sh | sudo bash
```

脚本会自动完成：

1. 检查并安装构建依赖；
2. 拉取本仓库源码；
3. 编译新的 `Openxhh` 二进制；
4. 停止已有 `Openxhh` systemd 服务，如果存在；
5. 备份旧二进制；
6. 替换为新二进制；
7. 重新启动服务，如果原服务存在。

如果安装目录不是 `/opt/Openxhh`：

```bash
curl -fsSL https://raw.githubusercontent.com/Www8881313/Openxhh/main/scripts/update-installed.sh | sudo env INSTALL_DIR=/你的安装目录 bash
```

如果 systemd 服务名不是 `Openxhh`：

```bash
curl -fsSL https://raw.githubusercontent.com/Www8881313/Openxhh/main/scripts/update-installed.sh | sudo env SERVICE_NAME=你的服务名 bash
```

也可以先下载脚本检查内容后再执行：

```bash
curl -fsSL -o update-installed.sh https://raw.githubusercontent.com/Www8881313/Openxhh/main/scripts/update-installed.sh
sudo bash update-installed.sh
```

## Linux / VPS 全新安装

### 1. 安装依赖并编译

```bash
apt update
apt install -y git curl ca-certificates build-essential libsqlite3-dev snapd
systemctl enable --now snapd.socket
snap install go --classic

git clone https://github.com/Www8881313/Openxhh /opt/Openxhh-src
cd /opt/Openxhh-src
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=sum.golang.google.cn
export GOMAXPROCS=1
go build -p 1 -o Openxhh .
```

### 2. 准备运行目录

```bash
mkdir -p /opt/Openxhh
cp /opt/Openxhh-src/Openxhh /opt/Openxhh/Openxhh
chmod +x /opt/Openxhh/Openxhh
cd /opt/Openxhh
```

### 3. 生成配置文件

首次运行会生成 `config.json` 并退出：

```bash
./Openxhh
```

编辑配置：

```bash
nano /opt/Openxhh/config.json
```

个人部署推荐 SQLite。示例配置：

```json
{
  "xhh": {
    "checkTime": 60,
    "replyTime": 30,
    "owner": "你的小黑盒数字UID",
    "deviceID": "",
    "baseUrl": "https://api.xiaoheihe.cn",
    "webver": "2.5",
    "version": "999.0.4"
  },
  "database": {
    "type": "sqlite",
    "db": "",
    "host": "",
    "port": "",
    "user": "",
    "passwd": ""
  },
  "ai": {
    "model": "你的模型名",
    "prompt": "你的回复策略",
    "baseUrl": "你的 OpenAI 兼容 /v1/chat/completions 地址",
    "token": "你的 AI API Token"
  },
  "image": {
    "model": "gpt-image-2",
    "baseUrl": "你的 OpenAI 兼容 /v1/images/generations 地址",
    "token": "你的图片 API Token",
    "size": "1024x1024",
    "responseFormat": "b64_json",
    "outputDir": "images",
    "uploadMode": "external",
    "externalDir": "/var/www/xhh-images",
    "externalBaseUrl": "http://你的VPS公网IP/xhh-images",
    "promptRefine": false,
    "promptModel": "",
    "promptBaseUrl": "",
    "promptToken": "",
    "promptMaxChars": 0
  }
}
```

配置要点：

- `xhh.owner` 填小黑盒数字 UID，不是昵称。
- `ai.baseUrl` 要填完整的 Chat Completions 地址，例如 `/v1/chat/completions`。
- `image.baseUrl` 要填完整的 Images Generations 地址，例如 `/v1/images/generations`。
- `image.uploadMode=external` 是当前推荐方案，会把图片写入 `image.externalDir`，评论里使用 `image.externalBaseUrl`。
- 不要公开 `config.json`、`cookie.json`、`sql.db`。

### 4. 可选：配置外部图片目录

如果需要评论区生图，建议让 VPS 暴露一个静态图片目录。已安装 Nginx 时，可在站点配置中加入：

```nginx
location /xhh-images/ {
    alias /var/www/xhh-images/;
    add_header Access-Control-Allow-Origin *;
}
```

准备目录并放入测试图：

```bash
mkdir -p /var/www/xhh-images
cp /你的测试图片.png /var/www/xhh-images/test.png
chmod 755 /var/www/xhh-images
chmod 644 /var/www/xhh-images/test.png
nginx -t && systemctl reload nginx
```

确认公网可访问：

```bash
curl -I http://你的VPS公网IP/xhh-images/test.png
```

### 5. 登录小黑盒

```bash
cd /opt/Openxhh
./Openxhh -mode login
```

扫码成功后会生成：

```text
/opt/Openxhh/cookie.json
```

### 6. 前台试运行

```bash
cd /opt/Openxhh
./Openxhh -mode start
```

确认运行正常后，再配置 systemd 后台运行。

### 7. 配置 systemd

```bash
cat >/etc/systemd/system/Openxhh.service <<'EOF'
[Unit]
Description=Openxhh
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/Openxhh
ExecStart=/opt/Openxhh/Openxhh -mode start
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now Openxhh
```

查看状态和日志：

```bash
systemctl status Openxhh --no-pager
journalctl -u Openxhh -f
```

## 常用命令

```bash
systemctl start Openxhh
systemctl stop Openxhh
systemctl restart Openxhh
systemctl status Openxhh --no-pager
journalctl -u Openxhh -f
```

程序模式：

```bash
./Openxhh              # 显示启动项提示
./Openxhh -mode login  # 扫码登录
./Openxhh -mode start  # 启动机器人
./Openxhh -mode test   # 测试模式
```

## 生图验证

验证命令识别和 Form Data，不调用真实生图接口：

```bash
go run ./cmd/dry_run_image_comment \
  -comment_id 123 \
  -link_id 181099114 \
  -root_id 123 \
  -userid 你的ownerUID \
  -text "@机器人 生图 一只赛博朋克猫"
```

调用真实生图接口但不上传、不发评论：

```bash
go run ./cmd/dry_run_image_comment \
  -comment_id 123 \
  -link_id 181099114 \
  -root_id 123 \
  -userid 你的ownerUID \
  -text "@机器人 生图 一只赛博朋克猫" \
  -mock_image=false
```

验证已有图片 URL 能否发带图评论：

```bash
go run ./cmd/test_image_comment 181099114 "图片测试" "http://你的VPS公网IP/xhh-images/test.png"
```

验证本地图片上传到外部图床并可选发布评论：

```bash
go run ./cmd/test_xhh_image_upload_comment \
  -file ./images/example.png \
  -link_id 181099114 \
  -reply_id -1 \
  -root_id -1 \
  -text "图片测试" \
  -publish=true
```

## 安全建议

- `config.json` 包含 AI token，`cookie.json` 是小黑盒登录态，不要上传到 GitHub。
- 不要把 `checkTime` 和 `replyTime` 调得太低，容易触发小黑盒风控；建议保持 `checkTime=60`、`replyTime=30` 或更保守。
- 建议限制运行目录权限：

```bash
chmod 600 /opt/Openxhh/config.json /opt/Openxhh/cookie.json /opt/Openxhh/sql.db 2>/dev/null || true
chmod 700 /opt/Openxhh
```

## 回滚

更新脚本会自动备份旧二进制，文件名类似：

```text
/opt/Openxhh/Openxhh.bak-20260517-120000
```

如需回滚：

```bash
systemctl stop Openxhh
cp /opt/Openxhh/Openxhh.bak-时间戳 /opt/Openxhh/Openxhh
chmod +x /opt/Openxhh/Openxhh
systemctl start Openxhh
```

## 免责声明

本项目仅供个人学习和自用。自动化访问、自动回复和频繁请求可能触发平台风控，请自行控制频率并遵守小黑盒相关规则。

## 致谢

感谢 [SomeOvO/xhhRobot](https://github.com/SomeOvO/xhhRobot) 原项目提供早期基础思路与实现参考。
