# MyGO!!!!! 迷子扫雷 (`egoist/mygo` Native UI 重制版)

> 「迷子でもいい、迷子でも進め。」

基于 **[github.com/egoist/mygo](https://github.com/egoist/mygo)** 原生 GPU UI 组件体系从头重构的 MyGO!!!!! 主题扫雷桌面应用（Windows Direct3D 11 / macOS Metal / Linux OpenGL，无 HTML/WebView，无 `cgo`）。

## 复用的 MyGo 原生组件

- **`ui.Segmented` & `ui.ToggleGroup` / `ui.Toggle` & `ui.Switch`**：曲目难度切换、乐队/经典模式切换、标旗模式、`?` 标记、名场面图鉴开关、夜间/日间主题切换。
- **`ui.Avatar` & `ui.Badge` & `ui.Tooltip`**：5 位 MyGO!!!!! 成员头像徽章、剩余雷数/用时/技能次数胶囊、悬停技能说明。
- **`ui.Progress` & `ui.Meter`**：排练完成度进度条 + 乐队重力值（Gravity）警戒仪表。
- **`ui.Themed` & `ui.Scroll` & `ui.Grid`**：跟随当前领队成员代表色变化的互动面板、实时「MyGO!!!!! 排练室群聊」、以及支持双向滚动的扫雷棋盘。
- **`c.Toast` / `c.ToastAction` & `ui.AlertDialog`**：技能与成就弹出提示、炸团一键「假装重开」Toast、以及「春日影 Live」踩雷专属弹窗质问。

## 运行、测试与跨平台打包

```powershell
# 运行全部引擎与 Native UI 内存交互测试
go test -v ./...

# 启动 Windows 桌面端
go run .

# 打包 Windows / macOS Universal (.app)
go run github.com/egoist/mygo/cmd/mygo@v0.9.2 build -platform darwin/universal -o build
```
