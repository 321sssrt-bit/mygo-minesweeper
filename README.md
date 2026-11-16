# MyGO!!!!! 迷子扫雷 (egoist/mygo Native UI)

> 「迷子でもいい、迷子でも進め。」

使用 **[github.com/egoist/mygo](https://github.com/egoist/mygo)** 原生 GPU 渲染 UI（`github.com/egoist/mygo/ui`，纯 Go、无 `cgo`、无 HTML/WebView）构筑的 MyGO!!!!! 主题桌面端扫雷。

## 运行与构建

```powershell
# 直接运行桌面端应用
go run .

# 运行引擎与 Native UI 视图测试（无窗口内存渲染测试）
go test -v ./...

# 编译成独立可执行文件
go build -o mygo-minesweeper.exe .
```

## 特色玩法

- **纯正 Native UI**：Windows 上由 Direct3D 11 直接渲染，秒开低内存。
- **三种难度**：
  - `羽丘中庭`（9×9 · 10 雷）
  - `RiNG Live`（16×16 · 40 雷）
  - `春日影舞台`（30×16 · 99 雷）
- **双模式切换**：
  - **乐队应援模式**：5 位 MyGO!!!!! 成员各自拥有专属台词与技能（高松灯「捡来的石头」、千早爱音「SNS 高光」、长崎爽世「温柔的微笑」被动护盾、椎名立希「精准节拍」、要乐奈「随性巡演」）。
  - **纯正经典模式**：关闭技能，记录各难度最快通关用时。
- **完整扫雷操作**：首击必安全（尽可能保留 3×3 安全区）、右键标旗（或开启 `F` 标旗模式）、点击已翻开数字触发 Chord 周围快开、快捷键 `R` 重开 / `F` 标旗模式 / `E` 发动技能。
