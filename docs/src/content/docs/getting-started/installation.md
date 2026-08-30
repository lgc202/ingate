---
title: 安装
description: 使用 Docker Compose 安装、启动和卸载 Ingate
---

当前正式支持 Docker Compose 安装。安装过程使用已经发布的容器镜像，不要求本机安装 Go、Node.js 或仓库源码。

## 环境要求

- Linux 或 macOS
- Bash、curl、tar
- Docker Engine
- Docker Compose v2

确认 Docker 可用：

```bash
docker version
docker compose version
```

## 安装最新版本

```bash
curl -fsSL https://github.com/lgc202/ingate/releases/latest/download/install.sh | bash
```

安装器会完成以下操作：

1. 读取最新正式 Release
2. 下载对应版本的 Compose 安装包和校验文件
3. 校验 SHA-256
4. 生成管理员密码和会话签名密钥
5. 拉取镜像并等待组件就绪

默认安装目录是当前目录下的 `ingate`。安装结束时终端会显示 Console 地址、管理员用户名和一次性展示的密码。

## 访问入口

默认只绑定本机地址：

| 入口 | 地址 |
| --- | --- |
| Console | `http://127.0.0.1:8001` |
| HTTP Gateway | `http://127.0.0.1:8080` |
| HTTPS Gateway | `https://127.0.0.1:8443` |

Gateway 端口只有在创建并成功发布对应 Gateway 后才会承载流量。

## 日常命令

进入安装目录后执行：

```bash
./bin/status.sh
./bin/logs.sh
./bin/stop.sh
./bin/start.sh
```

停止不会删除资源和分析数据。

## 完整卸载

```bash
./bin/uninstall.sh
```

卸载脚本会要求输入 `uninstall`，然后删除容器、网络、持久化 Volume 和安装目录。需要保留 Volume 时使用：

```bash
./bin/uninstall.sh --keep-data
```

:::caution
不带 `--keep-data` 的完整卸载会永久删除声明式资源、证书、Assistant 会话与模型连接、实时额度计数、请求记录和分析数据。
:::

固定版本安装、备份恢复和升级流程见[配置与维护](../../operations/overview/)。
