# OCR 增强待办

> 状态：活跃
> 创建：2026-10-30
> 基线：`OCR-vs-CPF实测对比.md` §8（11 PR / 60 条 CPF 评论，三阶段实测）
> 当前分支 HEAD：`941cee2`（`feat/cpf-kb-review-enhancements`）
> 维护约定：每完成一项，更新本文档的状态、commit、验证命令与残留风险；未验证的内容不得标记为完成。

## 0. 维护约定（过程透明）

### 0.1 状态定义

| 状态 | 含义 | 使用规则 |
|---|---|---|
| 未开始 | 尚未改动 | 默认状态 |
| 进行中 | 已开始，未达到验收标准 | 必须写明当前卡点 |
| 待验证 | 改动完成，等待 replay/测试 | 必须附验证命令与预期结果 |
| 已完成 | 验收标准全部通过 | 必须附 commit 与验证证据 |
| 阻塞 | 依赖外部条件 | 必须写明阻塞项与解除条件 |

### 0.2 估时口径

- 单位为净工时，按“熟悉本仓库的 1 名工程师”估算，不含 review 等待与返工。
- 分项估时只估算实现本身；阶段合计（P0 约 1 天、P1 约 2-3 天、P2 约 1 周）包含 replay、fixture、评审与缓冲，二者不要求严格相加。
- 单项估时与阶段合计冲突时，以“含验证的实际耗时”为准，并在完成记录中更新。
- 涉及外部目标仓（`rule.json`）的改动，不含跨仓评审与合入时间。

### 0.3 每项完成的最低记录

每个任务完成时，在 §5 追加一行，至少包含：

1. 日期与 commit。
2. 改动文件列表。
3. 执行的验证命令与结果（`make test` / `make check` / replay 命令）。
4. replay 对比结果（命中、误报、耗时、token）。
5. 残留风险与未覆盖项。

### 0.4 证据口径

- 能力差距以 `OCR-vs-CPF实测对比.md` §8 为准，不以单条评论的主观感受为准。
- “真实 miss”指平台/领域知识本体缺口；通用工程规则、规则已载入未触发、结构性缺口分别归因，不混入同一分母。
- 增强效果以同一模型、同一预算、同一基准 revision 的 replay 结果为准；更换模型或预算时必须注明，不能直接对比。

## 1. 背景与目标

### 1.1 背景

基于 `OCR-vs-CPF实测对比.md` §8 的能力差距评估：

- 样本：11 个 PR、60 条 CPF 评论，分首轮、Stage A、Stage B 三阶段。
- 知识库覆盖度：真实 miss 6 条，以 60 条全样本计约 10%；有效 recall 严格 40.0%（14/35），宽口径 65.7%（23/35）。
- 真实 miss 分布：pr408 4 条（linker/target capability）、pr447#3（`prctl` 前置条件）、pr438#2（跨平台 dump 格式）。
- 结论：**CPF 在检查深度、输出质量、结构性能力三方面更强；OCR 在 OCR-only 发现与不确定性表达上有互补优势。**

### 1.2 目标

增强 OCR 在以下三方面的能力：

1. **检查深度**：跨文件穷尽性、触发路径推理、基线/上游对照。
2. **输出质量**：4 级级别体系 + 处置位、5 要素证据、规则 ID 目录。
3. **结构性能力**：平台特定规则、领域路由、PR 流程检查。

### 1.3 原则

- **复用 agent-skills 知识库，不重复造轮子**：已有 20 个 catalog、168 条 reference 条目，优先做格式转换与映射。
- **prompt 优先，代码其次**：先用低成本 prompt 改动验证收益，再决定是否改 schema/架构。
- **每项可独立验证**：每项任务都要有明确的 replay 或单测验收，不把多项改动混在一个不可归因的提交里。
- **不追求 CPF 完全对齐**：只补高价值差距；OCR 的独立发现与 `needs_confirmation` 机制保留。

### 1.4 范围与非目标

**范围内**：

- `main_task_system.md` 等 prompt 的检查深度与输出结构。
- severity/disposition、`rule_id`、catalog 等 schema 与加载逻辑。
- 目标仓 `.opencodereview/rule.json` 的平台特定规则。
- PR metadata 输入层与 PR 流程检查。

**非目标**：

- 不更换底层模型，不以模型差异解释 recall 差距。
- 不重写 review 主流程，不引入新的 agent 框架。
- 不在本任务中实现完整的 CPF 规则库同步服务。

## 2. 任务清单（按优先级）

### 2.0 总览

| ID | 优先级 | 任务 | 维度 | 估时 | 状态 | 依赖 |
|---|---|---|---|---|---|---|
| 1.1 | P0 | 级别体系统一为 4 级 + 处置位 | 输出质量 | 2 小时 | 未开始 | 无 |
| 1.2 | P0 | 证据要求从 3 要素扩到 5 要素 | 输出质量 | 30 分钟 | 未开始 | 1.1（处置位语义） |
| 1.3 | P0 | 建立规则 ID 目录 `catalog.json` | 输出质量 | 4 小时 | 未开始 | 1.1 |
| 2.1 | P0 | 加强跨文件穷尽性检查 prompt | 检查深度 | 30 分钟 | 未开始 | 无 |
| 2.2 | P0 | 加强触发路径推理 prompt | 检查深度 | 30 分钟 | 未开始 | 1.2 |
| 2.3 | P0 | 加入基线对照 prompt | 检查深度 | 1 小时 | 未开始 | 无 |
| 3.1 | P1 | 补充 OHOS linker 等平台特定规则 | 结构性能力 | 2-4 小时 | 未开始 | 1.3 |
| 3.2 | P1 | 验证 pr408 重跑 | 验证 | 30 分钟 | 未开始 | 3.1 |
| 3.3 | P1 | 建立领域路由机制 | 结构性能力 | 4-8 小时 | 未开始 | 1.3 |
| 4.1 | P2 | 架构改动支持 PR body | 结构性能力 | 1-2 天 | 未开始 | 无 |
| 4.2 | P2 | 集成 PR body 到 prompt | 结构性能力 | 1 天 | 未开始 | 4.1 |

统计：P0 0/6，P1 0/3，P2 0/2。

**勾选清单**：

- [ ] 1.1 级别体系统一为 4 级 + 处置位（P0，2 小时）
- [ ] 1.2 证据要求从 3 要素扩到 5 要素（P0，30 分钟）
- [ ] 1.3 建立规则 ID 目录 `catalog.json`（P0，4 小时）
- [ ] 2.1 加强跨文件穷尽性检查 prompt（P0，30 分钟）
- [ ] 2.2 加强触发路径推理 prompt（P0，30 分钟）
- [ ] 2.3 加入基线对照 prompt（P0，1 小时）
- [ ] 3.1 补充 OHOS linker 等平台特定规则（P1，2-4 小时）
- [ ] 3.2 验证 pr408 重跑（P1，30 分钟）
- [ ] 3.3 建立领域路由机制（P1，4-8 小时）
- [ ] 4.1 架构改动支持 PR body（P2，1-2 天）
- [ ] 4.2 集成 PR body 到 prompt（P2，1 天）

### 2.1 P0（立即可做，约 1 天）

#### 1.1 级别体系统一为 4 级 + 处置位（2 小时，改 Go 代码）

**目标**：把当前 6 级 severity（`critical/high/medium/low/suggestion/needs_confirmation`）改为 CPF 对齐的 4 级（`critical/high/medium/suggestion`），把 `needs_confirmation` 从“级别”改为独立的“处置位”。

**建议方案**：

- `severity`：`critical` / `high` / `medium` / `suggestion`，对应致命/严重/一般/建议。
- 新增 `disposition`：`confirmed` / `needs_confirmation`，默认 `confirmed`；仅表示证据状态，不参与严重度排序。
- 删除 `low`：按影响并入 `suggestion`，或在证据充分时并入 `medium`；迁移映射需在实现前定稿（见 §7.3 D1）。
- 旧数据兼容：读取旧评论时保留渲染；新输入不得再产生 `low` / `needs_confirmation` 作为 severity。

**改动点**：

| 文件 | 改动 |
|---|---|
| `internal/config/template/prompts/main_task_system.md:39-52` | 重写 Comment Structure 与 Severity Definitions；明确处置位不得软化已确认问题 |
| `internal/config/toolsconfig/tools.json:70-81` | severity enum 改为 4 项；新增 `disposition` 字段与描述 |
| `internal/tool/code_comment.go:27-32,48-53,157-167,205-210` | 常量、合法值表、解析、归一化；未知值回落策略 |
| `internal/model/review.go:18-30` | `Severity` 注释与 `Disposition` 字段 |
| `internal/viewer/server.go:198-203,270-284,425-433` | 计数、过滤器、展示；删除 `low` 分支 |
| `cmd/opencodereview/sarif.go:169-185` | SARIF level 映射；`needs_confirmation` 不再作为 severity |
| `cmd/opencodereview/output.go:173-185` | 终端颜色与 badge |
| `internal/config/template/prompts/review_filter_task_system.md:14` | 过滤规则改为按 `disposition` 处理，不按 severity 特判 |
| `internal/viewer/*_test.go`、`internal/tool/*_test.go` | 枚举、迁移、旧数据渲染测试 |

**验收标准**：

- `tools.json` 的 severity enum 只有 4 项；新输出不再出现 `low`。
- `needs_confirmation` 只出现在 `disposition`，不出现在 severity。
- 旧会话（含 `low` / `needs_confirmation`）仍可加载、渲染、导出。
- `make test`、`make check`、`make coverage` 通过；SARIF 输出级别符合预期。
- replay 一组历史评论，确认严重度分布变化可解释。

**风险**：

- 2 小时可能偏乐观：viewer、SARIF、导出、过滤器和测试都是下游消费者，需逐一排查。
- 迁移映射会改变历史统计口径，必须在 PR 描述中说明。

**状态**：未开始。

#### 1.2 证据要求从 3 要素扩到 5 要素（30 分钟，改 prompt）

**目标**：把评论证据从 `Problem / Impact / Suggestion` 扩为 CPF 的 5 要素，且不新增 schema 字段。

**5 要素映射**：

| 要素 | 落点 | 要求 |
|---|---|---|
| Location | `path` + `content` 的 `file:line` | 精确到文件、行、符号 |
| Introduction | `content` | 新增代码、回归、配置默认值变化，或“存量问题”及依据 |
| Trigger | `content` / `impact` | 前置条件、调用链、线程/状态、平台分支；非显而易见时写 `entry → ... → defect` |
| Verification | `content` | 已用哪些工具/命令/文件验证；仍缺什么证据；缺证据时用 `disposition=needs_confirmation` |
| Fix & verification | `suggestion_code` / `content` | 可执行修复 + 如何验证修复（测试、命令、预期结果） |

**改动点**：

- `internal/config/template/prompts/main_task_system.md:39-44`：把 3 条改为 5 条，保留英文标签。
- `internal/config/toolsconfig/tools.json` 的 `content` / `impact` 描述同步，避免 prompt 与 schema 描述冲突。
- 不新增字段，不扩大 JSON schema；控制 token 增长。

**验收标准**：

- 抽样 3 条 high/critical 评论，均能指出 5 要素；medium 可压缩，但必须说明“已验证/仍缺证据”。
- 不出现编造的验证结果；未验证时必须显式说明。
- prompt 改动后 replay 的评论长度增长在可接受范围（建议 <15%），不因篇幅挤占 review 预算。

**风险**：

- 模型可能机械套标签；需要在 prompt 中说明“合并叙述，不必逐字列标题”。
- 与 1.1 的处置位语义耦合，建议同一天完成。

**状态**：未开始。

#### 1.3 建立规则 ID 目录（4 小时，从 agent-skills 和 kotlin-pr-review 整理 `catalog.json`）

**目标**：建立唯一规则目录，使 `rule_id` 只能来自目录，解决“规则无编号、无法审计、无法映射”的问题。

**两个 ID 空间要分开**：

| 来源 | 内容 | 规模（2026-10-30 核对） | 用途 |
|---|---|---|---|
| `kotlin-pr-review` | 规则 ID：`A/V/B/G/I/R/P` + `KC/CXX/C/J` | 207 个唯一 ID，11 个前缀 | 作为 `rule_id` 的权威来源 |
| `agent-skills` | reference catalog ID（如 `ohos-platform.kmp-platform-overview`） | 20 个 catalog，168 条 reference | 作为知识文档索引，映射到领域/规则，不直接当规则 ID |
| `规则映射表.md` | 反推规则 70 条（来自 130 条评论） | 70 条 | 作为 OCR 现有规则的对照与缺口清单 |

**建议 schema**：

```json
{
  "schema": "ocr-rule-catalog/v1",
  "version": "2026-10-30",
  "sources": [
    {"name": "kotlin-pr-review", "commit": "9aafffb", "license": "Apache-2.0"}
  ],
  "rules": [
    {
      "id": "P1-001",
      "prefix": "P",
      "title": "Konan target Family must be OHOS",
      "domain": "ohos-platform",
      "severity": "critical",
      "source": {"repo": "kotlin-pr-review", "path": "references/custom/kotlin-ohos-platform.md", "line": 9},
      "applies_to": {"paths": ["kotlin-native/konan/**"], "keywords": ["KonanTarget", "Family.OHOS"]},
      "check": "新增/修改 target 时校验 Family 与 dynamicSuffix",
      "remediation": "改为 Family.OHOS 并保持 dynamicSuffix=so",
      "status": "active"
    }
  ]
}
```

**实施步骤**：

1. 写生成脚本（建议 `scripts/build-rule-catalog.*`），解析 `references/custom/*.md` 与 `references/*-standard.md` 的表格行，不按章号机械生成。
2. 校验 ID 唯一性、前缀合法性、来源行存在；跳号不补造。
3. 合并 `规则映射表.md` 的 70 条反推规则，标注 `confirmed/corrected/reverse-only`。
4. 把 agent-skills 的 168 条 reference 作为 `references[]` 索引，按 domain 关联到规则。
5. 生成内置 catalog，并支持目标仓 `.opencodereview/rules/catalog.json` 覆盖。
6. prompt 增加“不得自造 ID；未知 ID 放 `disposition=needs_confirmation`”；`tools.json` 的 `rule_id` 描述同步。

**验收标准**：

- 每条规则有 `id`、`source`、`domain`、`severity`、`check`；无来源的规则不得进入目录。
- 207 个规则 ID 与源文件一一对应；无重复、无自造。
- catalog 可由脚本重复生成；手工编辑会被 CI 检查发现。
- 未知 `rule_id` 不会导致评论丢失，而是降级为处置位。
- 许可证与来源保留：agent-skills / kotlin-pr-review 均为 Apache-2.0，保留 commit 与 NOTICE/归属信息。

**风险**：

- 4 小时依赖脚本一次写对；若手工整理，工作量会明显超。
- 源仓库更新会导致漂移；需要记录 `knowledge_version` 与 commit，按版本刷新。
- `agent-skills` 的 reference ID 不是规则 ID，不能混用；文档和 schema 必须写清楚。

**状态**：未开始。

#### 2.1 加强跨文件穷尽性检查 prompt（30 分钟）

**目标**：把“跨文件一致性”从提示性检查变成可执行的穷尽性清单，稳定命中 pr408#14 类似问题。

**建议 prompt 段**（追加到 `main_task_system.md` 的 Cross-File Consistency Checks）：

1. 先列出本次变更的“变更面”：枚举、sealed class、`when`、映射表、注册表、生成表、平台分支、构建配置。
2. 对每个新增值/键/开关/目标，搜索所有消费者，而不是只看同文件。
3. 检查生成文件与手写文件是否同步；同一语义是否有两份真相。
4. 在评论中写明“搜索词 + 已检查文件 + 仍未覆盖项”；未穷尽时不得声称“全部检查”。
5. 对跨平台代码，对照其他平台/其他 target 的同类实现。

**改动点**：

- `internal/config/template/prompts/main_task_system.md:21-27`。
- 可同步到 scan 模式的 prompt/template（若 scan 复用同一 system prompt，则无需重复改）。

**验收标准**：

- replay pr408，确认 #14（HWADDRESS 新枚举未同步 4 处 `when`）仍命中，且评论包含搜索范围。
- 构造一个“新增枚举只改一处”的 fixture，能稳定命中。
- 不因穷尽性要求产生大量低价值评论；误报率不高于当前基线。

**风险**：

- 增加工具调用与 token；需要在 replay 中记录耗时。
- 模型可能把“已搜索”写成套话；prompt 要求给出具体搜索词与文件。

**状态**：未开始。

#### 2.2 加强触发路径推理 prompt（30 分钟）

**目标**：让 high/critical 评论稳定输出“从入口到缺陷”的调用链，而不是“这里可疑”。

**建议 prompt 规则**：

- high/critical 必须给出触发路径：`entry → step A → step B → defect`。
- 路径必须包含：前置条件、线程/状态、数据形状、平台分支（适用时）。
- 无法建立路径时：降级为 medium，或 `disposition=needs_confirmation`，并写明还缺哪一段证据。
- 不得用函数名猜测路径；路径中的每一跳都要有代码依据。

**改动点**：

- `internal/config/template/prompts/main_task_system.md:39-44`（与 1.2 同一段，建议一起改）。
- `internal/config/toolsconfig/tools.json` 的 `impact` 描述可补充“trigger path”。

**验收标准**：

- replay 一组 high/critical 评论，路径完整率可量化（建议按“入口、关键跳、缺陷点”三项打分）。
- 不出现无法从代码验证的路径；无法验证时按规则降级。
- 评论长度可控，不因路径重复代码摘录。

**风险**：

- 与 1.2 的 5 要素重叠，需避免重复叙述。
- 路径推理会增加工具调用；需观察 token/耗时。

**状态**：未开始。

#### 2.3 加入基线对照 prompt（1 小时）

**目标**：识别“相对 base/上游/其他平台的回归”，而不是只描述当前 diff。

**建议 prompt 规则**：

- 当改动修改既有行为时，对照 base 行为、上游 JetBrains 行为、其他平台同类实现，说明是否回归。
- 明确区分“本次引入”与“存量问题”；存量问题不得伪装成回归。
- 对照不到 base 时，必须写“baseline unavailable”，不得猜测。
- 对照结论要给出依据：base 文件/行、其他平台文件/行、或 diff 中的删除行。

**工具层限制（必须记录）**：

- 当前 OCR 的上下文工具在 review ref 上读取文件；`file_read` / `code_search` 没有独立的 base ref 参数。
- 因此 1 小时版本只能做到“基于 unified diff 的删除行 + 同仓其他平台/上游同类实现”的 prompt 对照，**不是真正的 base revision 读取**。
- 若 replay 证明收益明显，再评估新增 base-ref 读取工具（预计 4-8 小时，未计入本项 1 小时）；该后续项需单独立项，不与本项混在一起。

**改动点**：

- `internal/config/template/prompts/main_task_system.md` 新增 Baseline Comparison 段。
- 如已有 Cross-File Consistency Checks，可合并到同一段，避免重复。

**验收标准**：

- 选 2-3 个“修改既有行为”的样本，评论能指出 base 行为与回归风险。
- base 不可得时，评论明确说明限制，不编造对照结论。
- 不把存量问题误报为本次回归。

**风险**：

- prompt-only 的能力上限受工具限制；验收时必须区分“prompt 生效”与“真正读到 base”。
- 可能增加评论篇幅；建议只对 medium 及以上启用。

**状态**：未开始。

### 2.2 P1（中期可做，约 2-3 天）

#### 3.1 补充 OHOS linker 等平台特定规则（2-4 小时，真实 miss 6 条）

**目标**：补齐 6 条真实 miss 对应的规则，优先落在目标仓 `rule.json`，并同步到内置规则目录/知识文档。

**6 条真实 miss 与规则来源**：

| Miss | 问题 | 类型 | 参考来源 |
|---|---|---|---|
| pr408#2 | `crtn.o` 链接顺序错误 | OHOS linker | `规则映射表.md` R-xplat-5；`kotlin-pr-review` B3 |
| pr408#7 | `useCompilerDriverAsLinker` 与 `ld.lld` 矛盾 | OHOS linker | 同上 |
| pr408#10 | 链接命令缺失 `dynamicLibraries` | OHOS linker | 同上 |
| pr408#11 | `OHOS_X64` 不应列 HWADDRESS | target capability | `规则映射表.md` R-xplat-4；`R3-003` |
| pr447#3 | `prctl(PR_SET_VMA_ANON_NAME)` 去掉 `useMmap` 短路 | 系统调用前置条件 | `OCR-vs-CPF实测对比.md` §4.3；agent-skills GC/runtime 资料 |
| pr438#2 | 非 OHOS `DumpMemoryAsync` 改为 gzip、移除 `isStrip` 拒绝 | 跨平台 dump | `OCR-vs-CPF实测对比.md` §4.5；agent-skills `ohos-native-linking-and-abi.md` 等 |

**建议做法**：

- 把 pr408 的 4 条并入 `R-xplat-5` 的逐项 checklist，而不是只写一句“检查链接顺序”。
- 每条规则写清：触发条件、必须对照的文件/字段、反例、修复方向、验证方式。
- `pr408#11` 增加 target capability 矩阵校验（sanitizer/CPU 特性按 target 能力表）。
- `pr447#3` 增加“系统调用前置条件/平台分支短路”规则。
- `pr438#2` 增加“平台分支切换输出格式时必须核对所有消费方，并保留或显式说明原有能力”。
- 规则写入目标仓 `.opencodereview/rule.json`，同时把可复用部分沉淀到 OCR 内置 catalog（1.3）。

**验收标准**：

- 6 条 miss 各有对应规则与规则 ID；规则可在 catalog 中检索到。
- 用 pr408 / pr447 / pr438 分别 replay，6 条至少命中 4 条；未命中的必须给出原因（输入过滤、预算、规则未触发）。
- 不引入明显误报；新增规则的误报样本需记录并修正。
- 目标仓改动单独提交，附 replay 结果；不把外部仓改动混入 OCR 主仓提交。

**风险**：

- 2-4 小时可能只够 pr408 的 4 条；pr447/pr438 需要额外查资料与 replay，实际可能接近 1 天。
- 目标仓 `rule.json` 是外部路径（见 §4），改动不在 OCR 仓，需要单独评审。
- `pr438#2` 的漏报还受评审超时影响，规则补上后仍需确认不是预算问题。

**状态**：未开始。

#### 3.2 验证 pr408 重跑（30 分钟）

**目标**：用 3.1 的新规则重跑 pr408，确认 4 条残余 miss 的命中变化，并记录误报/耗时/token。

**基准信息**（来自 `OCR-vs-CPF实测对比.md` §9.1）：

| 项 | revision/说明 |
|---|---|
| CPF 评论基准 | `bd101f9`（pr408 PR 中间状态） |
| pr408 最终 merge | `c4244058501` |
| 最终修复提交 | `52fcab` |
| OCR 重跑基准 | `c4244058501` + 目标仓 `rule.json` `include: ["**/konan/target/**"]` |

**执行要求**：

- 使用与上次重跑相同的 OCR 版本、模型、预算、rule.json；任何变化都要记录。
- 重跑后逐条对照 #2/#7/#10/#11，给出“命中/未命中/弱重合”和证据。
- 同时记录误报数、总评论数、耗时、token；若 partial/timeout，必须注明未完成文件。
- pr447/pr438 的 2 条 miss 不在本项范围，需在 3.1 完成后单独 replay 或并入本项扩展验证。

**验收标准**：

- 输出一张对比表：miss → 上次结果 → 本次结果 → 证据/原因。
- 至少确认“规则生效”与“规则未生效”的差异可解释；不以“模型随机性”一句带过。
- 结果写回本文档 §5 和 `OCR-vs-CPF实测对比.md`（如需要）。

**风险**：

- 30 分钟仅指一次重跑与记录；若模型排队或 40+ 文件预算不足，实际会更久。
- 单次重跑有随机性；关键结论建议至少两次或固定温度/种子（若支持）。

**状态**：未开始。

#### 3.3 建立领域路由机制（4-8 小时，按领域映射到文件路径）

**目标**：根据变更路径/关键词选择领域知识，只注入相关规则与参考文档，避免“全量规则 + 全量知识”导致 token 膨胀和注意力稀释。

**建议路由表**：

| 领域 | 路径/关键词触发 | 注入内容 |
|---|---|---|
| architecture | `compiler/fir/`、`compiler/frontend/`、`core/descriptors/`、`compiler/backend/`、`analysis/` | `kotlin-architecture.md`；agent-skills 架构类 reference |
| ohos-platform | `kotlin-native/konan/`、`platformLibs/`、`ohos`、`OHOS_ARM64`、`OHOS_X64`、`Family.OHOS` | `kotlin-ohos-platform.md`；`kmp-ohos-platform` |
| native-compiler-runtime | `compiler/ir/backend.native/`、`kotlin-native/backend.native/`、`Linker`、`LLVM`、`konan.properties` | `kotlin-compiler-native-backend.md`；`kmp-native-compiler-runtime`、`kmp-gc` |
| interop | `kotlin-native/platformLibs/`、`Interop/`、`.def`、N-API/TSFN | `kotlin-interop-platformlibs.md`；`kmp-ffi-akinterop` |
| native-runtime | `kotlin-native/runtime/`、`stdlib/native-wasm/` | `kotlin-native-runtime.md`；`kmp-gc` |
| gradle-kmp | `kotlin-gradle-plugin*` | `kotlin-gradle-kmp.md` |
| build-release | `gradle.properties`、`konan.properties`、`dependencies/`、`pom.xml` | `kotlin-build-release.md` |
| render | Compose/ArkUI/渲染相关路径 | `kmp-fusion-render` |
| devtools | 设备日志、trace、调试路径 | `kmp-devtools` |

**建议机制**：

- 路由表配置化（JSON/YAML），不要写死在 Go 代码里；路径 glob + 关键词 + diff token 三类触发。
- 每次 review 记录“命中的领域 + 注入的文档 + token 估算”；未命中时明确“未加载领域知识”。
- 设 token 上限，最多注入 1-2 个领域；冲突时按“更具体的路径优先”。
- 先做离线路由脚本 + 单测（类似 CPF 的 `route-review-docs.sh`），再接入 prompt，降低回归风险。
- 与 1.3 的 catalog 共用 domain 字段，避免两套映射。

**验收标准**：

- 对 11 个实测 PR 的路由结果人工核对，至少覆盖所有真实 miss 所在领域。
- 单测覆盖：路径命中、关键词命中、无命中、冲突优先级、token 上限。
- 路由可关闭；关闭后行为与当前版本一致。
- replay 对比：token 下降或持平，关键发现不下降。

**风险**：

- 4-8 小时是脚本 + 配置 + 单测的估算；接入 prompt 与 replay 可能追加 1 天。
- 关键词路由容易误触发；需要以路径为主、关键词为辅。
- agent-skills 文档在仓外，OCR 工具无法直接读取；需先 vendoring 或只注入摘要（见 §3 方案 B 的限制）。

**状态**：未开始。

### 2.3 P2（长期可做，约 1 周）

#### 4.1 架构改动支持 PR body（1-2 天，支持 GitCode/GitHub API）

**目标**：让 OCR 在代码 diff 之外拿到 PR 描述、提交、CI、评审线程等元数据，为 PR 流程检查提供输入。

**建议架构**：

- 新增 `internal/prmeta`（建议名）包，定义 `Provider` 接口与统一 `Metadata` 结构。
- 实现 GitCode / GitHub 两个 provider；用 `net/http` + `httptest` fixture 测试，不引入重量级 SDK（除非仓库已有）。
- `Metadata` 至少包含：title、body、author、base/head SHA、commits、changed files、checks/status、reviews/comments、linked issues、DCO。
- CLI 增加显式开关（如 `--pr` / `--pr-url`）与 token 来源（环境变量/配置），默认关闭，避免影响现有 diff-only 用法。
- 失败降级：元数据拉取失败不阻塞代码评审，只记录 warning；不得把“未提供”当成“缺失”报 process finding。
- 安全：token 不落日志、不写入产物；URL 校验；超时与重试；缓存可选。

**验收标准**：

- GitCode/GitHub fixture 均能解析为统一结构；字段缺失时有明确零值语义。
- 端到端：PR body 能进入 prompt 上下文；无 token 泄漏；失败时 review 正常完成。
- 单测覆盖：成功、401/403、404、超时、部分字段缺失、超长 body 截断。
- 文档记录开关、token 配置与隐私影响。

**风险**：

- 两个平台的 API 字段差异大，1-2 天可能偏紧；建议先做 GitHub，再适配 GitCode。
- 认证与权限模型不同；不要把平台 token 写进仓库配置。
- PR body 可能含敏感信息；需要截断/脱敏策略。

**状态**：未开始。

#### 4.2 集成 PR body 到 prompt（1 天）

**目标**：把 PR 元数据接入 `Process Checks`，让 OCR 能检查模板四要素、测试报告、DCO/CI 等流程问题。

**建议做法**：

- 复用 `main_task_system.md:29-34` 的 Process Checks，把“任务上下文包含 PR 元数据”改为显式输入块。
- 明确输入块结构：`<pr_metadata>` + 字段；缺失字段标注 `not provided`，不要留空让模型猜。
- 规则：PR 描述四要素、issue 关联、DCO、CI 结果、Release Notes、验证证据；测试完整性；上游合并回归；整套测试禁用。
- 输出：`category=process`；严重度按影响，不一律 high；证据不足用 `disposition=needs_confirmation`。
- 明确边界：只根据提供的元数据判断；不推断 CI 通过，不虚构测试报告。

**验收标准**：

- 用 431#3 类样本 replay，能识别 PR 描述/测试报告缺口。
- 元数据缺失时，不产生 process finding；只提示“未提供，无法检查”。
- 不把代码问题误分类为 process；不把 process 问题重复报成多条。
- 单测/fixture 覆盖：完整元数据、缺 body、缺 CI、DCO 缺失、超长描述。

**风险**：

- 依赖 4.1；PR body 质量差时可能产生大量低价值评论，需要阈值与去重。
- process 规则的严重度校准需要人工样本，不能只靠 prompt。

**状态**：未开始。

## 3. 复用 agent-skills 方案

### 3.1 三种方案对比

| 方案 | 做法 | 工作量 | 优点 | 缺点 | 建议 |
|---|---|---|---|---|---|
| A 格式转换 | 从 `references/*.md` 抽取规则到 `catalog.json`，保留内容只改格式 | 约 2-4 小时 | 可审计、可离线、token 可控 | 需要 schema 与生成脚本；源更新需重跑 | 推荐作为基础 |
| B 引用知识库 | prompt 写 agent-skills 路径，模型按需读取 | 约 1 小时 | 改动最小、保留全文 | 当前 `file_read` 受 repo root 限制，仓外路径不可读；依赖模型自觉读取 | 不单独使用 |
| C 混合 | 核心规则进 `catalog.json`，详细文档 vendoring 后作为引用；规则 ID + 文档索引 | 约 4-8 小时（含 A） | 兼顾精度、上下文与可维护性 | 需要目录规划与同步机制 | **最实用，推荐** |

### 3.2 方案 B 的关键限制

- OCR 的 `file_read` / `code_search` 以仓库根为边界，`pathutil.WithinBase` 会拒绝仓外路径（见 `internal/tool/fileread.go:101-112`）。
- 因此 prompt 里写 `/tmp/agent-skills-...` 或 GitCode URL，模型无法直接读取；只能：
  1. 把需要的文档 vendoring 到仓库内（如 `docs/knowledge/` 或 `.opencodereview/knowledge/`），或
  2. 扩展工具白名单/外部根目录（安全风险更高），或
  3. 只注入摘要/规则，不依赖全文读取。
- 结论：**B 的 1 小时只适用于“把路径写进 prompt”，不等于模型能读到内容。**

### 3.3 推荐落地步骤（方案 C）

1. **建目录**：`catalog.json`（规则/索引）+ `knowledge/`（按 domain 存放 vendoring 的 reference 摘要或全文）。
2. **抽取**：脚本解析 `kotlin-pr-review` 的 207 条规则 + `agent-skills` 168 条 reference，生成 catalog；保留 `source`、`commit`、`license`。
3. **映射**：用 domain 字段把规则、reference、路径触发条件关联起来；与 3.3 的路由表共用。
4. **注入**：prompt 只加载“目录摘要 + 命中领域的核心规则”；详细文档按需加载，受 token 上限约束。
5. **校验**：CI 校验 catalog schema、ID 唯一性、来源存在、生成文件与脚本一致；禁止手改生成文件。
6. **刷新**：记录 `knowledge_version`（agent-skills 2026-09）与 commit；每次刷新单独提交并附 diff 说明。

### 3.4 许可证与归属

- `agent-skills`：Apache-2.0（本地 `LICENSE` 已核对）。
- `kotlin-pr-review`：Apache-2.0（`AIReviewer/LICENSE` 已核对）。
- 复用规则/文档时保留来源、commit、许可证信息；不要把外部全文直接复制进源码而不带归属。
- 若只抽取“规则思想”并重写为 OCR 自己的规则，仍需在来源字段记录参考出处。

## 4. 关键资源

### 4.1 白盒材料

| 资源 | 用途 | 版本/commit | 地址 |
|---|---|---|---|
| agent-skills | 知识库、20 个 catalog、168 条 reference | `0c3dc40`，knowledge 2026-09 | https://gitcode.com/CPF-KMP-CMP/agent-skills |
| kotlin-pr-review | 207 条规则 ID、评论格式、路由脚本 | 分支 `kotlin-pr-review`，`9aafffb` | https://gitcode.com/LPJ_2026/CPF-KMP-CMP-AI |
| Harness | 评审策略、A/B/C/D 维度、流程证据 | `426d82c` | https://gitcode.com/LPJ_2026/CPF-CMP-Harness |

### 4.2 本地路径（2026-10-30 核对）

| 资源 | 本地路径 | 备注 |
|---|---|---|
| agent-skills | `/tmp/agent-skills-ocr-read-20261029/` | 临时目录，可能被清理；缺失时按 URL + commit 重新拉取 |
| kotlin-pr-review | `/tmp/cpf-kmp-ai-review/` | 规则在 `AIReviewer/skills/kotlin-pr-review/references/` |
| Harness | `/tmp/cpf-harness/` | 本次核对时临时目录已不存在；按 URL + commit `426d82c` 重新拉取 |
| 目标仓 rule.json | `/Users/zhangxinyu/oh/hello/kotlin/.opencodereview/rule.json` | 目标仓路径，非 OCR 仓库；按机器实际为准 |
| OCR prompt | `/Users/issuser/code/open-code-review/internal/config/template/prompts/main_task_system.md` | 主评审 system prompt |
| OCR 自仓 rule.json | `/Users/issuser/code/open-code-review/.opencodereview/rule.json` | 与本任务的目标仓文件区分 |

### 4.3 关联文档

- `OCR-vs-CPF实测对比.md`：§8 能力差距评估、§4 归因、§9 pr408 基准、§10 Stage A/B。
- `规则映射表.md`：130 条评论 → 70 条反推规则、P0/P1/P2 修正清单。
- `CPF评审知识库反推.md`：CPF 知识库结构反推。
- `CPF平台AI评审对比报告.md`：平台能力对比。

### 4.4 快速核对命令

```bash
# 核对本地材料 commit（缺失时先重新拉取）
git -C /tmp/agent-skills-ocr-read-20261029 rev-parse HEAD
git -C /tmp/cpf-kmp-ai-review rev-parse HEAD
git -C /tmp/cpf-harness rev-parse HEAD   # 若目录不存在，先按 URL + 426d82c 重新拉取

# 核对规则 ID 数量（去重后应为 207）
grep -rhoE '\| [A-Z]{1,3}[0-9]+-[0-9]{3} \|' \
  /tmp/cpf-kmp-ai-review/AIReviewer/skills/kotlin-pr-review/references/*.md \
  /tmp/cpf-kmp-ai-review/AIReviewer/skills/kotlin-pr-review/references/custom/*.md \
  | sed 's/| //;s/ |//' | sort -u | wc -l
```

## 5. 进度跟踪

### 5.1 完成记录

- [x] 事项 6：白盒化（规则映射表 + P0 修正）—— 2026-10-29，commit `6953175`
- [x] 事项 4：pr408 重跑（营救生效 + 规则巡检有效）—— 2026-10-29，commit `13a358c`
- [x] 事项 5：Stage A + Stage B（11 PR 详细匹配分析）—— 2026-10-29，commit `3d329b0`、`616b747`
- [x] 能力差距评估（§8 已添加到文档）—— 2026-10-30，commit `2bfae0e`
- [x] 创建本文档（OCR 增强待办）—— 2026-10-30，未提交，待 review

**提交哈希说明**：以上为记录时的分支提交。当前分支重建后存在 patch-id 等价的提交，核对结果如下（2026-10-30）：

| 记录提交 | 当前分支等价提交 | patch-id |
|---|---|---|
| `6953175` | `3bedcd6` | `2f5b41f` |
| `13a358c` | `5510ebf` | `8493c09` |
| `3d329b0` | `ef3ecec` | `2cf53cc` |
| `616b747` | `07436e5` | `6e5df2a` |
| `2bfae0e` | `941cee2` | `53c2a41` |

### 5.2 当前状态

| 优先级 | 已完成 | 进行中 | 未开始 | 阻塞 |
|---|---|---|---|---|
| P0 | 0 | 0 | 6 | 0 |
| P1 | 0 | 0 | 3 | 0 |
| P2 | 0 | 0 | 2 | 0 |

**下一步检查点**：

- 完成 1.1 + 1.2 后，记录一次样例 replay（严重度分布、5 要素覆盖）。
- 完成 1.3 后，记录 catalog 规则数与来源校验结果。
- 完成 P0 后，做一次整体 replay，再决定是否进入 P1。

### 5.3 更新日志

| 日期 | 变更 | commit/说明 |
|---|---|---|
| 2026-10-30 | 创建文档，整理 P0/P1/P2 任务、复用方案、资源与验收标准 | 未提交 |

**本次创建核对记录（2026-10-30）**：

- 读取 `OCR-vs-CPF实测对比.md` §8（行 389-463）以及 §6、§9；`规则映射表.md` §3；`main_task_system.md` 的检查与严重度段落。
- 核对 agent-skills `0c3dc40`、kotlin-pr-review `9aafffb`、Harness `426d82c`；本地路径状态见 §4.2（Harness 临时目录本次核对时已缺失）。
- 统计 kotlin-pr-review 规则 ID 207 个（去重）、agent-skills catalog 20 个 / reference 168 条。
- 本文档只做待办规划，未执行代码改动，未运行 `make test`；所有“已完成”以 §5.1 的 commit 为准，估时以任务执行时的实际记录为准。

## 6. 关键洞察

1. **知识库沉淀比模型差异更决定 recall**：OCR 的规则库经过 P0 修正后已相当完善，真实 miss 只有 10%；双方在资源生命周期/并发/NAPI 边界上的逐字级重合也印证了这一点。
2. **真实 miss 主要在 OHOS 特定规则**（linker、ABI、target capability）：可以通过继续补充规则库解决，不需要模型或架构改动。
3. **结构性缺口（PR 流程检查）需要架构改动**：不是规则库能解决的，是当前 CPF 相对 OCR 最明确的护城河。
4. **复用 agent-skills 知识库**：不用重复造轮子，只需要格式转换和适配；但要注意仓外文档不可直接读取的限制（§3.2）。
5. **输入层与预算的影响不亚于规则本身**：pr408 首轮受 `**/target/**` 误杀影响，398/411/438 受 token 预算/超时影响；增强效果必须用 replay 验证，不能只看 prompt 改动。
6. **prompt 改动便宜但稳定性有限，schema 改动影响面大**：先 prompt、再 replay、最后 code/schema，是风险最低的顺序。

## 7. 下一步行动

### 7.1 建议顺序

1. 先做 P0 输出质量和检查深度（约 1 天，最容易增强且效果最明显）。
2. 再做 P1 平台特定规则和领域路由（约 2-3 天）。
3. 最后做 P2 PR 流程检查（约 1 周，需要架构改动）。

### 7.2 立即可做

- 从 agent-skills 抽取 OHOS linker 特定规则到 `rule.json`（3.1 的第一批）。
- 建立规则 ID 目录 `catalog.json`（1.3），先把 207 条规则与 168 条 reference 索引跑通。
- 加强跨文件穷尽性检查和触发路径推理 prompt（2.1、2.2），改完立即 replay pr408。

### 7.3 待定决策（实现前必须定稿）

| 编号 | 决策 | 选项 | 建议 |
|---|---|---|---|
| D1 | `low` / `needs_confirmation` 的迁移映射 | low→suggestion 或 low→medium；needs_confirmation→medium + disposition 或保留旧值渲染 | 先保留旧数据渲染，新输入只产生 4 级 + disposition；映射表写进 PR |
| D2 | catalog 存放位置 | 内置 `internal/config/template/rules/catalog.json`；目标仓 `.opencodereview/rules/catalog.json`；两者叠加 | 内置默认 + 目标仓覆盖，脚本生成 |
| D3 | agent-skills 复用方式 | A 格式转换、B 引用、C 混合 | 采用 C，核心规则进 catalog，文档 vendoring |
| D4 | 基线对照是否加 base-ref 工具 | prompt-only；新增 base-ref 读取工具 | 先做 2.3 的 1 小时版本，replay 有收益再立项 |
| D5 | PR metadata 首期平台 | GitHub 优先；GitCode 优先；两者同时 | 先 GitHub（API 稳定、fixture 容易），再 GitCode |

### 7.4 第一周执行建议

| 时间 | 任务 | 产出 |
|---|---|---|
| 第 1 天上午 | 1.1 + 1.2 | 4 级 + 处置位、5 要素 prompt；单测通过 |
| 第 1 天下午 | 2.1 + 2.2 + 2.3 | 三段 prompt；pr408 replay 记录 |
| 第 2 天 | 1.3 | `catalog.json`、生成脚本、来源校验 |
| 第 3-4 天 | 3.1 + 3.2 | 6 条 miss 规则、pr408/447/438 replay |
| 第 5 天 | 3.3 | 路由表、脚本、单测、11 PR 路由核对 |
| 第 2 周 | 4.1 + 4.2 | PR metadata provider、prompt 集成、fixture |

### 7.5 完成判定（DoD）

每个任务完成必须同时满足：

- [ ] 代码/prompt 改动已 review，未混入无关重构。
- [ ] 新增/修改逻辑有单测或 replay 证据；`make test`、`make check` 通过。
- [ ] replay 结果记录：命中、误报、耗时、token；partial/timeout 已注明。
- [ ] 文档已更新（本文档 + 相关对比/映射文档）。
- [ ] 残留风险与未覆盖项已写明，未用“应该没问题”代替验证。
- [ ] 如涉及外部仓（`rule.json`），单独提交并附 replay 结果。

### 7.6 风险与缓解

| 风险 | 影响 | 缓解 |
|---|---|---|
| 1.1 下游消费者多（viewer/SARIF/export/filter） | 改动超时、旧会话不兼容 | 先列消费者清单，加旧数据兼容测试；必要时拆成两个提交 |
| 1.3 规则目录与源漂移 | `rule_id` 失效、审计失真 | 记录 commit/knowledge_version；CI 校验生成文件；禁止手改 |
| 2.x prompt 变长 | token 挤占 review 预算、评论啰嗦 | 设定长度上限；只对 medium+ 启用详细证据；replay 监控 token |
| 3.1 目标仓改动不可控 | 规则无法及时生效 | 目标仓单独提交；先在内置 catalog 验证，再同步 |
| 3.2 单次 replay 随机性 | 结论不可复现 | 固定模型/预算/温度；关键结论至少两次；记录 partial 文件 |
| 3.3 路由误触发 | 注入无关知识、token 膨胀 | 路径优先、关键词为辅；token 上限；可关闭开关 |
| 4.x API 认证/隐私 | token 泄漏、评审失败 | 环境变量、日志脱敏、失败降级、fixture 测试 |

## 附录 A：任务完成记录模板

```markdown
### <任务 ID> <任务名>

- 日期：
- commit：
- 改动文件：
- 验证命令：
- 验证结果：
- replay 对比：命中 / 误报 / 耗时 / token
- 残留风险：
- 后续项：
```

## 附录 B：常用验证命令

```bash
# 单元测试（项目标准命令）
make test

# 格式化、tidy、vet 等
make check

# 覆盖率（90% 阈值）
make coverage

# 英文检查（涉及源码/注释时）
make english-check

# 规则 ID 去重计数（预期 207）
grep -rhoE '\| [A-Z]{1,3}[0-9]+-[0-9]{3} \|' \
  /tmp/cpf-kmp-ai-review/AIReviewer/skills/kotlin-pr-review/references/*.md \
  /tmp/cpf-kmp-ai-review/AIReviewer/skills/kotlin-pr-review/references/custom/*.md \
  | sed 's/| //;s/ |//' | sort -u | wc -l
```
