# BPB-Tools 快速使用指南

## 文件结构

确保你的目录结构如下：

```
BPB-Tools/
├── BPB-Tools.exe          # 主程序
├── test.bat               # 测试脚本（可选）
├── projects/              # 源代码目录
│   ├── CloudflareSpeedTest/   # CloudflareSpeedTest 源代码
│   │   ├── main.go
│   │   ├── go.mod
│   │   ├── ip.txt
│   │   └── ...
│   └── BPB-Wizard/            # BPB-Wizard 源代码
│       ├── main.go
│       ├── go.mod
│       └── ...
└── README.md              # 说明文档
```

**重要**: `projects` 目录必须与 `BPB-Tools.exe` 在同一目录！

## 使用方法

### 方法一：直接运行

1. 双击 `BPB-Tools.exe`
2. 根据菜单提示输入数字选择功能

### 方法二：命令行运行

```cmd
cd d:\Projects\BPB-Tools
.\BPB-Tools.exe
```

## 功能说明

### 1. CloudflareSpeedTest (功能 1)

**用途**: 测试 Cloudflare CDN IP 的延迟和下载速度

**默认配置文件**:
- `ip.txt` - IPv4 IP 段列表
- `ipv6.txt` - IPv6 IP 段列表

**常用参数**:
```cmd
-n 200        # 延迟测速线程数（默认 200）
-t 4          # 单个 IP 测速次数（默认 4）
-dn 10        # 下载测速 IP 数量（默认 10）
-tl 200       # 平均延迟上限（默认 9999ms）
-dd           # 禁用下载测速
-httping      # 使用 HTTP 测速模式
```

**示例**:
```cmd
# 使用默认配置测速
CloudflareSpeedTest.exe

# 自定义参数
CloudflareSpeedTest.exe -n 300 -t 5 -tl 150 -dn 20
```

**输出**:
- 终端显示最快 IP 列表
- `result.csv` - 完整测速结果（可用 Excel 打开）

### 2. BPB-Wizard (功能 2)

**用途**: 在 Cloudflare 上部署 BPB 代理面板

**支持**:
- Cloudflare Workers 部署
- Cloudflare Pages 部署

**流程**:
1. 选择创建或修改面板
2. 浏览器打开 OAuth 授权页面
3. 授权后返回终端
4. 按提示配置参数或使用默认值
5. 自动部署到 Cloudflare

**输出**:
- 部署成功的访问地址
- 订阅链接
- 配置信息

### 3. 懒人一键部署 (功能 3)

开发中，敬请期待...

## 常见问题

### Q: 运行 CloudflareSpeedTest 时报错 "open ip.txt: The system cannot find the file specified"

**A**: 确保 `projects/CloudflareSpeedTest` 目录包含 `ip.txt` 和 `ipv6.txt` 文件。程序会自动编译并运行该目录下的源代码。

### Q: 如何修改 CloudflareSpeedTest 的配置？

**A**: 直接编辑 `projects/CloudflareSpeedTest` 目录下的源代码或配置文件（如 `ip.txt`），修改后重新运行即可。主程序会自动编译最新代码。

### Q: BPB-Wizard 无法打开浏览器

**A**: 手动复制终端显示的 OAuth URL 到浏览器访问即可。

### Q: 测速结果都是超时或失败

**A**: 
1. 检查网络连接
2. 尝试减少线程数（如 `-n 100`）
3. 更换 IP 段列表

### Q: 如何退出程序？

**A**: 在主菜单输入 `q` 即可退出。

## 高级用法

### 单独运行工具

你也可以直接在 `projects` 目录运行原始工具：

```cmd
cd projects\CloudflareSpeedTest
go run main.go -h
cd ..\..\projects\BPB-Wizard
go run main.go
```

### 自定义 IP 段

编辑 `projects/CloudflareSpeedTest/ip.txt` 添加你自己的 IP 段：

```
1.1.1.0/24
1.0.0.0/24
# 添加更多...
```

修改后重新运行程序，主程序会自动编译并使用新的配置。

### 批处理脚本

创建批处理文件自动化操作：

```batch
@echo off
echo 1 | .\BPB-Tools.exe
pause
```

## 注意事项

1. **网络要求**: 需要能访问 Cloudflare API
2. **代理设置**: 如需代理请先配置好环境变量
3. **权限要求**: BPB-Wizard 需要 Cloudflare 账户授权
4. **文件完整**: 确保 `projects` 目录包含两个子项目的完整源代码
5. **Go 环境**: 系统需要安装 Go 语言环境（程序运行时会自动编译）

## 技术支持

- CloudflareSpeedTest 原版：https://github.com/XIU2/CloudflareSpeedTest
- BPB-Wizard 原版：https://github.com/bia-pain-bache/BPB-Wizard

## 高级用法

### 直接修改源代码

你可以直接编辑 `projects/CloudflareSpeedTest` 或 `projects/BPB-Wizard` 目录下的任何文件：

- 修改参数默认值
- 进行汉化
- 添加新功能
- 调整 UI 显示

修改后重新运行 BPB-Tools.exe，主程序会自动编译最新的源代码。

---

祝你使用愉快！🎉
