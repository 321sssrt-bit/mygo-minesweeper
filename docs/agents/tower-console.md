# 派车阵容

本仓库用 tower-console 派车时按下表选车。表里的 ID 已对过 T3 模型目录（`orchestrator_capabilities`），与全局技能写法不同时以本表为准。

## 轨道

票的标题前缀就是轨道：【后端】→ 后端代码车，【前端】→ 前端代码车，【UI】→ UI 变动车。三条轨写的都是 Go：前端是 mygo `ui` 的状态、接线与读屏，UI 是画布绘制与样式，截图由 `TestSnapshot` 出。`copy` 票走 UI 车，措辞由 UI 车定稿。

## 主力档

| 车 | providerInstanceId | model | options |
|---|---|---|---|
| 后端代码车 | `acpRegistry_devin` | `swe-2-high` | `thought_level: "high"` |
| 前端代码车 | `antigravity` | `gemini-4-hydrogen` | `thinking_level: "xhigh"` |
| UI 变动车 | `claudeAgent` | `claude-opus-5-5` | `effort: "xhigh"` |
| 纯看图汇报车 | `antigravity` | `gemini-4-carbon` | `thinking_level: "high"` |

## 车内委派

子车自己调 `delegate_task`，塔台不另开车：

| 步骤 | providerInstanceId | model | options |
|---|---|---|---|
| 双路审查 ① | `codex` | `auto/gpt-6.1-sol` | `reasoningEffort: "xhigh"` |
| 双路审查 ② | `antigravity` | `gemini-4-carbon` | `thinking_level: "high"` |
| 扫除无头代码 | `codex` | `spark1/group/deepseek-v4-1-flash` | `reasoningEffort: "high"` |

## 省额档

后端代码车不变；前端代码车改用 `codex` / `spark1/group/k3-1-pro`（`reasoningEffort: "high"`）；不发 UI 车。
