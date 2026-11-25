# CloudFlareScan V5.0

本项目基于小琳解说的 CloudFlareScan V5.0 Go/Wails 桌面版。 [上游项目](https://github.com/xiaolin-007/CloudFlareScan) · [V5.0 发布页](https://github.com/xiaolin-007/CloudFlareScan/releases/tag/V5.0)

## 使用方法

1. 在横版首行的「端口」菜单逐项勾选，或点击「全选」。默认勾选 443。点击「IPv4 扫描」或「IPv6 扫描」会检查所有选中的端口。
2. 扫描完成后，「完全测速」会测试本次扫描的全部结果。「地区测速」只测试勾选的地区，首行「测速数量」会自动填入这些地区的可测条数，可以手动调低。采样参数在标题右侧的「设置」中调整。
3. 扫描结束后，可以点击「测速结果」旁的「导出结果」。选择 CSV 或 TXT、默认或精简，并勾选要导出的地区。弹窗默认全选可导出的地区。精简样式使用「国家或地区简写-地区码」，例如 `US-SJC`，并省略延迟和下载测速信息。单次最多导出 500 条已识别地区的记录，同一个 IP 的不同端口各算一条。优先按测速排名导出，否则按扫描延迟排名导出。默认文件名包含本机日期、时间和毫秒，例如 `cfs_results_20261125_143507_123.csv`。
4. 扫描日志下方的表格列出符合地区勾选条件的 IP。点击 IP 可以复制地址。测速列按 IP 和端口填入对应速度。点击日志右上方的「隐藏日志」可收起日志区，为表格腾出空间。
5. Windows 版打开窗口时会自动检查直连状态，在「设置」左侧显示绿点「可优选」、红点「请关闭代理」或黄点「状态未知」。切回窗口时会重新检测，也可以点击旁边的「刷新」。绿点表示至少一个 IP 版本可直连，红点表示检测到 TUN/VPN 路由。开始扫描和测速时会再检查本次任务的目标路由。普通 HTTP 代理由程序的直连方式自动绕过。

地区码优先取自 Cloudflare 的 `colo`，缺失时尝试读取同一响应的 `CF-Ray` 后缀。两个来源都缺失时，国家和地区留空。Cloudflare 的同一个 Anycast IP 可能由多个机房响应，第三方 IP 库的位置无法用来确认这次连接的实际机房。

## 在 Windows x64 构建

使用 Go 1.25 或更高版本。运行程序需要 Microsoft Edge WebView2 Runtime。 [Wails Windows 指南](https://v2.wails.io/docs/guides/windows/)

在项目根目录打开 PowerShell 并运行下面的命令。

```powershell
$env:CGO_ENABLED = '0'
go test -mod=readonly -tags 'desktop,production' ./...
New-Item -ItemType Directory -Path 'build/bin' -Force | Out-Null
go build -mod=readonly -trimpath -tags 'desktop,production' -ldflags '-s -w -H windowsgui' -o 'build/bin/CloudFlareScan.exe' .
```

构建完成后运行 `build/bin/CloudFlareScan.exe`。

## 前端文件与许可证

`frontend/dist` 来自官方 V5.0 Windows 发行包中的已编译界面，上游公开仓库未提供原始 Vue 工程。Go 构建会直接嵌入此目录。提交源码时需要包含整个 `frontend/dist`，构建时无需运行 npm。

地区和端口菜单的主要逻辑与样式位于 `frontend/dist/assets/multiselect.js` 和 `frontend/dist/assets/multiselect.css`。V5.0 的桌面入口是 `main.go`，`CloudFlareScan.py` 是独立的 Python V4.1 入口。

项目保留[原作者 MIT 许可证](LICENSE)及[内嵌 Vue 3.5.35 的许可证](licenses/Vue-3.5.35-LICENSE.txt)。

## 上游项目说明

[![Downloads](https://img.shields.io/github/downloads/xiaolin-007/CloudFlareScan/total?style=flat-square&logo=github)](https://github.com/xiaolin-007/CloudFlareScan/releases)
[![Latest Version](https://img.shields.io/github/release/xiaolin-007/CloudFlareScan.svg?style=flat-square)](https://github.com/xiaolin-007/CloudFlareScan/releases)
[![License](https://img.shields.io/github/license/xiaolin-007/CloudFlareScan?style=flat-square)](https://github.com/xiaolin-007/CloudFlareScan/blob/main/LICENSE)

CloudFlareScan （简称CFS)  适配 Windows  macOS  Android  Linux

软件演示视频：https://www.youtube.com/watch?v=Fw2W4B77bts

⚠️免责声明：
本程序仅限用于学习与研究目的
请在下载后24小时内自行删除。使用本程序时，应自行遵守所在地区的法律法规
作者不对使用本程序所产生的任何后果承担责任
下载或使用本程序即视为已阅读、理解并同意上述声明

<img width="611" height="476" alt="14eaeb7504c8db5685f261513fd8ecf2" src="https://github.com/user-attachments/assets/46976c98-f8ce-4d55-948b-05151c0f4d03" />


🚀 高效扫描：自动从 CloudFlare 官方 IP 段生成 IP 地址

📊 双模式测速：完全测速 + 地区测速，满足不同需求

🌍 全球覆盖：内置全球大部分机场IATA代码映射，自动识别地区

⚡ 异步处理：支持高并发测试，快速获取结果

📋 一键复制：双击表格单元格即可复制内容

📈 实时统计：显示扫描进度、速度

使用方法：

win-X64 直接下载使用

Android 直接下载安装  

macOS arm 安装提前 需要将安全性与隐私里-选择允许从任何来源

终端输入命令：sascript -e 'do shell script "sudo spctl --master-disable" with administrator privileges'

Linux系统使用方法：解压后

chmod +x CloudFlareScan

./CloudFlareScan



