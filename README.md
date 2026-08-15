<div align="right"><sub><a href="./README.en.md">English</a>&nbsp;&nbsp;⇄&nbsp;&nbsp;<b>简体中文</b></sub></div>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./assets/hero-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="./assets/hero-light.svg">
  <img src="./assets/hero-light.svg" width="880" alt="Survmap — agent 会话存活归因">
</picture>

<p align="center"><sub>把每个 agent 会话回放成「有效 vs 白改」红绿热力图，数据不出本机。</sub></p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/SuperMarioYL/survmap?color=5E5CE6&style=flat" alt="license"></a>
  <a href="https://github.com/SuperMarioYL/survmap/releases"><img src="https://img.shields.io/github/v/release/SuperMarioYL/survmap?color=10A37F&label=release" alt="release"></a>
  <a href="https://github.com/SuperMarioYL/survmap/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/SuperMarioYL/survmap/ci.yml?branch=main&label=ci&color=0071E3" alt="ci"></a>
  <a href="https://goreportcard.com/report/github.com/SuperMarioYL/survmap"><img src="https://goreportcard.com/badge/github.com/SuperMarioYL/survmap" alt="go report"></a>
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white" alt="go">
  <a href="https://gitee.com/SuperMarioYL/survmap"><img src="https://img.shields.io/badge/Gitee-mirror-C71D23?logo=gitee&logoColor=white" alt="gitee"></a>
</p>

> **survmap 把每个会话回放成一张存活地图：哪几轮的编辑活到了当前 git HEAD，哪几轮是被回滚 / 改写掉的「白改」。**

跑完一个几十上百轮的 Claude Code / Codex 会话，你只看到最终 diff——但说不清哪几轮真正产出了持久价值、哪几轮是反复推翻才稳定下来。Survmap 在会话结束后按每轮编辑是否存活到 git HEAD 打分，oracle 是 `git blame` / `git log`（机器可检验，不是事后合理化），把回放变成一张红绿存活热力图：绿轮活下来，红轮被 churn。全程本机运行，会话日志与 repo 内容不离开宿主（信创 / 私有化友好）。

<h2><img src="https://api.iconify.design/tabler:topology-star-3.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 架构</h2>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./assets/atlas-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="./assets/atlas-light.svg">
  <img src="./assets/atlas-light.svg" width="880" alt="Survmap 架构：session.jsonl → 解析器 → 归因核心 → go-git blame → 渲染器，repo at HEAD 喂入归因核心">
</picture>

一个二进制，三个进程内模块，无微服务、无 Kubernetes：

- **Session parser**（`internal/session`）—— 摄取 Claude Code / Codex 会话 JSONL transcript，提取每轮文件编辑与新增行。
- **Attribution core**（`internal/attribution`）—— 被拥有的原语：对每轮新增行在 HEAD 跑 `go-git` blame，把存活 / 白改归因到 turn 级，算出每轮分数。
- **Renderer**（`internal/render`）—— 终端红绿热力图 + 导出。

新原语是 **turn→HEAD-survival 归因**。给定一个已完成会话（turn 序列，每个 turn = 一组文件编辑）与 repo 当前 HEAD，算法把每轮新增行映射到 git blame 输出，给该轮打分 `survived` / `churned` / `partial`。被拥有的是 turn→HEAD 映射层，底层 substrate（git）稳定且开放，不是寄生。

```go
type Turn struct {
    ID    string
    Ts    time.Time
    Edits []FileEdit   // {Path, Tool, AddedLines, LinesAdded, LinesRemoved}
}

type SurvivalScore struct {
    TurnID         string
    Status         Status   // Survived | Churned | Partial | Skipped
    SurvivedLines  int
    ChurnedLines   int
    Evidence       []BlameEvidence  // {Path, LineNo, CommitSHA, Survived}
}

type SessionSurvivalMap struct {
    Turns           []SurvivalScore
    ProductiveTurns int
    WastedTurns     int
    SurvivalRatio   float64
}
```

<h2><img src="https://api.iconify.design/tabler:bulb.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 为什么是现在</h2>

- agent 编码会话已成常态且越跑越长：单会话动辄数十至上百轮，「最终 diff 掩控全貌」的能力失效。
- 「Why does Opus 5 feel worse to work with?」式的 churn 焦虑在发酵——开发者越来越频繁经历「改了又改、推翻重来」，于是「哪几轮真正有效」才从无所谓变成必须知道。
- oracle 已存在且稳定：git HEAD 存活是机器可检验信号。存活归因第一次可落地、可量化。

> 诚实声明：存活 ≠ 绝对有效。一行可能只是因为没人动才「活下来」。Survival 是高提示性指标，不是「价值」的裁决；refactor / force-push / squash merge 会抹平单轮信号，m2 会叠加 churn 深度二级信号。

## 目录

- [架构](#架构)
- [为什么是现在](#为什么是现在)
- [安装](#安装)
- [快速开始](#快速开始)
- [用法](#用法)
- [Demo](#demo)
- [路线图](#路线图)
- [付费](#付费)
- [FAQ](#faq)
- [许可证](#许可证)

<h2><img src="https://api.iconify.design/tabler:download.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 安装</h2>

需要 Go 1.24（或直接下载 release 二进制，免装 Go）。

```bash
go install github.com/SuperMarioYL/survmap@latest
```

或从源码构建：

```bash
git clone https://github.com/SuperMarioYL/survmap.git
cd survmap && go build -o survmap .
```

> 国内信创机器：go-git 是纯 Go 实现，二进制单文件，**不依赖宿主 git CLI**——锁死的机器也能跑。

<h2><img src="https://api.iconify.design/tabler:rocket.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 快速开始</h2>

3 步从冷启动到第一张存活图（用仓库自带的示例会话）：

```bash
go build -o survmap .                                           # 1. 构建
cd examples/repo && git init -q && git add . && git commit -qm init && cd ../..  # 2. 准备示例 repo 到 HEAD
survmap score examples/sample.jsonl --repo examples/repo        # 3. 每轮存活 / 白改表
```

<details><summary>示例输出</summary>

```
Survmap — survival attribution for examples/sample.jsonl against HEAD 35aeb55

TURN   FILE            TOOL     ADD  SURV  CHRN  STATUS
─────────────────────────────────────────────────────────────────────
1      main.go         Write      8     6     2  partial
2      main.go         Edit       2     0     2  churned
─────────────────────────────────────────────────────────────────────
Session: 2 turns · 6/10 lines survived (60.0%) · 1 wasted turns (50.0% of scored)
=> 40% of added lines were churn (1 of 2 turns all-churned).
```

</details>

<h2><img src="https://api.iconify.design/tabler:terminal-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 用法</h2>

真实会话通常在 `~/.claude/projects/<project>/<session>.jsonl`。

```bash
# 每轮存活 / 白改打分（核心命令）
survmap score ~/.claude/projects/myproj/abc.jsonl --repo ./myrepo

# 红绿存活热力图（终端彩色）
survmap heatmap ~/.claude/projects/myproj/abc.jsonl --repo ./myrepo

# 可选：最小交互式 viewer（bubbletea，q/esc 退出）
survmap heatmap ~/.claude/projects/myproj/abc.jsonl --repo ./myrepo --interactive
```

| 子命令 | 作用 | m1 状态 |
|---|---|---|
| `survmap score <session> --repo <path>` | 每轮 survived/churned 表 | ✅ 已交付 |
| `survmap heatmap <session> --repo <path>` | 终端红绿热力图 | ✅ 已交付 |
| `survmap heatmap ... --interactive` | bubbletea 交互 viewer | 🟡 m1 最小版，m2 增强 |
| `survmap heatmap ... --html out.html` | 可分享 HTML 热力图 | ⏳ m2 |

常用 flag：`-r, --repo`（git 仓库 worktree 根，默认 `.`）。`score` 只读 repo 与会话日志，不写、不上传。

<h2><img src="https://api.iconify.design/tabler:photo.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Demo</h2>

对示例会话跑 `score` 再跑 `heatmap`，绿块=存活，红块=白改，底部一行 punchline。

![demo](assets/demo.gif)

完整终端录像（asciinema cast）：`assets/demo.cast`（本地 `asciinema play assets/demo.cast` 回放）。

<h2><img src="https://api.iconify.design/tabler:map-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 路线图</h2>

- [x] **m1 — 摄取 + blame + 归因核心**：解析 Claude Code 会话 JSONL，提取每轮文件编辑，在 HEAD 跑 go-git blame，把每轮新增行归因为存活/白改，`survmap score` 打印每轮表。
- [ ] **m2 — 热力图 TUI + HTML**：把存活图渲染成红绿热力图（bubbletea 全交互 TUI）+ `--html` 导出；「上个会话 X% 白改」star 时刻；10 分钟可装 demo 跑在真实会话上。
- [ ] **m3 — 团队 ROI 仪表盘**：按会话/开发者/repo 汇总存活比 + 可选本地国产 LLM（Qwen/DeepSeek）自然语言摘要 + SRE GPU 花费对账 hook。付费面，post-v0.1。

未来：跨平台 agent 支持（Codex 等）、多 repo / monorepo 级归因、会话内实时打分。

<h2><img src="https://api.iconify.design/tabler:credit-card.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 付费</h2>

**v0.1 OSS，本机永久免费。** Survmap Team 是给团队的私有化变现面，不是会泄露数据到云的 SaaS：

- **托管私有化部署**：Helm chart / 离线安装包，数据不出内网（信创客户的硬约束）。
- **团队会话 ROI 仪表盘**：按会话 / 开发者 / repo 汇总存活比，给 CTO 一份「agent 会话花了多少、活下来多少」的对账报告。
- **SRE 对账报告**：把白改轮次折算成私有 LLM 的 ¥/卡时——私有 GPU 算力的可解释账本。

参考价位（年付买断，不走 per-seat 云计费）：

| 档位 | 适用 | 价格（年） |
|---|---|---|
| 团队版 ≤ 20 席 | 单团队私有化 + ROI 仪表盘 | ¥12,000 |
| 企业版 ≤ 100 席 | + SRE 对账 hook 定制 | ¥48,000 |

预约 30 分钟 demo：跑你团队真实 repo 的 5 个会话，出样例 ROI 报告（存活比 + ¥/卡时估算）→ 离线安装包试用 7 天 → 合同。计费走国内对公转账（信创客户不接海外账单）。

<h2><img src="https://api.iconify.design/tabler:help-circle.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> FAQ</h2>

**survmap 算的是哪个 git？会话改了未提交的文件怎么办？**
oracle 是 repo 当前 HEAD（`git blame` / `git log`）。会话里的编辑不必是 commit——Survival 问的是「这一轮加的行，在当前 HEAD 的文件里还在不在」。未提交的工作区改动不参与：只有 HEAD 才是机器可检验的锚点。

**存活归因和 git blame 有什么区别？**
git blame 给每行最后一次修改的 commit；Survmap 把每个 agent turn 的整批新增行映射到 HEAD 存活/白改，输出 turn 级存活比，不是 line 级 blame。turn ≠ commit，所以是内容存活匹配（高提示性指标），不是把 turn 硬绑到 commit。

**Claude Code/Codex 原生上线每轮存活打分后 Survmap 怎么办？**
这是头号风险。护城河是跨平台 OSS 中立位（同时支持多个 agent）+ 早发快迭代 + Team 付费面的私有化部署——平台原生只覆盖自家会话。若窗口关闭，kill criteria 见仓库记录。

**我的会话日志 / 代码会离开本机吗？**
不会。v0.1 纯本机，会话日志 + repo 不离开宿主；Team 付费面也是私有化部署，不是云 SaaS。

<h2><img src="https://api.iconify.design/tabler:license.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> 许可证</h2>

MIT — 见 [LICENSE](./LICENSE)。欢迎提 issue / PR；商业支持见上方[付费](#付费)。

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
