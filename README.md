# BPB-Tools

CloudflareSpeedTest 和 BPB-Wizard 的整合工具集

## 项目介绍

本项目将两个强大的 Cloudflare 工具整合为一个统一的 CLI 应用，**完整保留原有功能**：

- **CloudflareSpeedTest**: CDN IP 延迟和下载速度测试工具（完整原版）
- **BPB-Wizard**: Cloudflare Workers/Pages 部署向导（完整原版）

## 快速开始

### Windows 用户
直接双击运行 `bpb-tools.exe`

### Linux/Mac 用户
```bash
./bpb-tools
```

### 使用说明

#### 方式一：交互式菜单（推荐）
直接运行程序，无需任何参数，会显示交互式菜单：
```
===============================
    BPB-Tools v1.0.0    
===============================
1. 运行 CloudflareSpeedTest (测速)
2. 运行 BPB-Wizard (部署代理面板)
3. 懒人一键部署 (开发中)
q. 退出程序
===============================
请选择功能 [1/2/3/q]:
```

#### 方式二：命令行子命令
也支持通过命令行参数直接启动功能：

```bash
# 启动 BPB 配置向导
bpb-tools.exe wizard

# 启动 Cloudflare IP 测速
bpb-tools.exe speedtest -n 200 -t 4 -dn 10

# 查看版本
bpb-tools.exe version

# 查看帮助
bpb-tools.exe help
bpb-tools.exe speedtest -h
```

## 重要说明

**本整合版采用调用独立可执行文件的方式，因此：**

✅ **完整保留原项目的所有功能**
✅ **没有任何功能删减或修改**
✅ **所有原始参数和选项都可用**
✅ **使用体验与原版完全一致**

## 后续计划

- [ ] 完善懒人一键部署功能
- [ ] 添加自动更新检查
- [ ] 优化启动速度和内存占用
- [ ] 支持更多实用工具集成

## 许可证

MIT License

## 致谢

- **CloudflareSpeedTest** 项目：https://github.com/XIU2/CloudflareSpeedTest
- **BPB-Wizard** 项目：https://github.com/bia-pain-bache/BPB-Wizard

---

**注意**: 本工具仅供学习和研究使用。请遵守当地法律法规，合理使用。
