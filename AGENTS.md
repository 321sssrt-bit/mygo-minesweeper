# AGENTS.md

MyGO!!!!! 迷子扫雷：纯 Go 桌面扫雷同人作品，界面基于 egoist/mygo 的 `ui` 包。术语以 `GLOSSARY.md` 为准，设计决定见 `docs/adr/`。

## 工作流程

- 新功能或改版：先 `/grill-with-docs` 拷问到共识，再 `/to-spec` 出规格、`/to-tickets` 拆票。
- 每张票开工前，列出票里尚未定的决定：有就先拷问，没有就说明；然后等用户说开工。
- 每张票做完即停：汇报改动、测试结果和截图，用户确认后再进入下一张。

## 分支与 PR

- 默认分支是 `master`；每张票从 `origin/master` 开分支。
- PR 正文写 `Closes #<票号>`，并 `Refs` 它所属的规格 issue。
- 改动可见界面的 PR 附修改前后的多视口实拍截图；用户看图确认前不合入。

## 文案

- 玩家可见文字的最终措辞由 `copy` 票定稿，这类票走 UI 变动轨。其他票需要新文案时先用现有文字或占位符，并在 PR 里列出涉及的字符串（位置、现文案、原因）。
- 梗由作者决定：成员称号、名场面、台词的增删改都打 `meme`，等作者确认。
- 规则与界面里的危险格一律叫「雷」（画面形象是心结）；决斗台词不写死棋盘尺寸和时长。
- 界面只说玩家的语言；技术细节只留在 README 底部的运行说明里。

## 美术

- 美术定调为「灯的笔记本」（ADR 0001）。颜色只从调色板取；成员颜色只用官方代表色及其派生的墨色。
- 优先复用 mygo `ui` 组件、用参数覆盖样式；只在组件画不出的视觉上自绘，并保留组件语义（角色、数值、键盘与焦点）。
- 只用原创绘制和 OFL 授权字体；可能触及官方 IP（立绘、Logo 字形、音频、曲名用法）的改动打 `ip-review`。

## 字体与截图

- 字体是按源码字符串子集化的霞鹜文楷。改了任何界面字符串就运行 `go generate`（需要 `pip install fonttools`），否则 `TestEmbeddedFontCoversCopy` 会失败。
- 样张与 README 截图：`MYGO_SNAPSHOT_DIR=<目录> go test -run TestSnapshot .`

## 测试

- 接缝按优先级：游戏引擎 API（固定种子）→ 无头 `ui.NewTester`（动画用虚拟时钟 `tt.Advance` / `Painter.Now()` 推进）→ 源码字面量扫描（缺字、文案、颜色约束）。
- 只断言外部行为；截图只供人工评审。

## 语言

- issue、规格、文档用中文；代码注释和提交信息用英文。

## Agent skills

### Issue tracker

GitHub Issues，用 `gh` 操作，领域标签与里程碑也在那里。See `docs/agents/issue-tracker.md`.

### Triage labels

默认五个分类角色标签。See `docs/agents/triage-labels.md`.

### Domain docs

Single-context：根目录 `GLOSSARY.md` + `docs/adr/`。See `docs/agents/domain.md`.

### 派车

用 tower-console 派车或给候选车时，阵容与模型 ID 以本仓库为准。See `docs/agents/tower-console.md`.
