# 塔台本仓约定

本仓库是纯 Go 桌面程序（基于 egoist/mygo `ui` 包），用 `tower-console` 与 `tower-tickets` 时按下列约定落地；模型阵容与推理档位完全沿用全局技能 `docs/routing.md`（`gemini-4-neon` 一律用 `thought_level: "high"`）。

## 四轨划分

三条代码轨写的都是 Go，按交付效果切轨：

- **【后端】后端代码车**：游戏引擎、求解与模拟、源码扫描与守卫测试，可见界面零变化。
- **【前端】前端代码车**：mygo `ui` 的状态、数据接线、键盘焦点与读屏语义，修改前后 `TestSnapshot` 截图一致。
- **【UI】UI 变动车**：画布绘制、样式、布局、图标与玩家可见文字（`copy` 票走 UI 车，最终措辞由 UI 车定稿）。
- **只读车**：只查事实、交报告，用项目根目录，不开 worktree、不出 PR。

## 联动文件、门禁与看图

- **字集联动**：任何会改动界面可见字符串的票，任务单「可改」清单须一并纳入 `resources/fonts/LXGWWenKai-Regular.subset.ttf` 与 `resources/fonts/LXGWWenKai-Medium.subset.ttf`，门禁前运行 `go generate`；多张改字票会在此处撞文件，不可并行发车。
- **门禁命令**：`go test ./...`（含字集改动时先 `go generate`）。纯本地测试，无需端口或临时库等隔离资源。
- **看图岗位**：桌面程序不启服务端口；`viewer.md` 在 `base SHA` 与最终 `HEAD` 上直接运行 `MYGO_SNAPSHOT_DIR=<目录> go test -run TestSnapshot .` 产出 `<场景>-<视口>-<day|night>.png` 实拍图并对照差异。
