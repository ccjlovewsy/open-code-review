# CPF 平台 AI 评审 vs OpenCodeReview(OCR)对比报告

> 状态:初版。CPF 侧证据来自远程观测(GitCode API + PR 评论),OCR 侧证据来自本仓库源码(可检验)。
> 日期:2026-10-16

## 0. 结论先行

- 这两者不是同类物:**OCR 是开源评审工具,CPF 仓库是被评审对象**。真正可比的是 OCR 的评审引擎 vs CPF 仓库上跑的 GitCode/华为平台级 AI 评审。
- CPF 侧闭源,但**评审结果可见**(PR 评论、标签、门禁状态),可通过历史 PR 评论黑盒反推其评审知识库(见 §5、§6)。
- OCR 侧全开源白盒:54 个语言规则文档、按路径挂载的 rule.json、多阶段确定性流水线 prompt 全部可审计。

## 1. 定位

| | OCR(alibaba/open-code-review) | CPF 侧(CPF-KMP-CMP/kotlin @ GitCode) |
|---|---|---|
| 本质 | AI 代码评审 CLI 工具(Go,Apache-2.0) | OpenHarmony 的 Kotlin Multiplatform fork(kotlin 编译器 2.2.21/2.4.20-OH,12.8 万 commits,华为系贡献者) |
| 评审能力来源 | 自带引擎:diff 解析 → 文件分组 → 规则匹配 → agent 评审 → 定位修正 → 反思过滤 | GitCode 平台 + OpenHarmony DCP 门禁(dcp.openharmony.cn)自带,不在仓库代码里 |
| 可检验性 | 白盒:源码、规则、prompt 全开放 | 黑盒:评审配置在平台侧;但**评审结果(评论/标签)公开可见** |

## 2. 已确认的重大发现(CPF 侧)

1. **`AI已检视` 标签**:PR(如 #452)上由平台自动打上,说明 GitCode 平台在跑 AI 评审流水线。
2. **`cpf-manager` 机器人**:以 `【AI-Review】【严重】【语言规范】【中文内容】` 三段式(问题/影响/建议)发结构化评审评论。样本:指出测试数据文件第 45 行中文注释违反「仓库语言约定为英文」,并明确「testData 目录但中文出现在注释中(非字符串字面量),不属于测试数据豁免范围」→ 推断其知识库含**语言规范规则 + 测试数据字符串豁免规则**。
3. **`CPF-KMP-CMP/agent-skills` 仓库**:org 下存在,疑似评审知识库/技能仓库(尚待抓取内容确认)。
4. **门禁体系**:`openharmony_ci` 机器人 + DCP 门禁:静态检查(codeCheck,未过即打回)+ 三平台编译(mac/win/linux,约 60 分钟)+ DCO 检查(`dco检查成功` 标签)。

## 3. 评审流水线对比

| 维度 | OCR(源码可验证) | CPF 平台 AI 评审(仅可从 PR 观测) |
|---|---|---|
| 触发 | 本地 CLI / CI(GitHub Action、GitLab、Gerrit)/ coding agent 插件 | PR 提交自动触发,打 `AI已检视` 标签 |
| 文件选择 | 确定性工程逻辑(`internal/diff`) | 不可见 |
| 文件分桶 | LLM 分组:语义相关文件一组(接口+实现、i18n 变体),每组 ≤10 文件,每组独立子 agent(`prompts/grouping_task_system.md`) | 不可见 |
| 规则匹配 | glob 路径首条命中(`internal/config/rules/system_rules.go`),54 种语言规则文档(含 `kotlin.md`) | 不可见 |
| 主评审 agent | 带上下文工具(读全文件/搜索);硬约束:每个文件必须单独过、不评论 `review_files` 之外、忽略注释与 `@Generated`(`main_task_system.md`) | 不可见 |
| 定位修正 | 独立 re_location 模块,修评论行号漂移 | 不可见 |
| 反思过滤 | 独立 fact-checker;非对称哲学:**错杀真发现 > 留下误报**,证据不足即放行(`review_filter_task_system.md`) | 不可见 |
| 静态检查 | 无(纯 AI) | DCP:codeCheck 静态检查 + 三平台编译,未过打回 |
| 输出 | 行级结构化评论,JSON 输出,session 可 resume、可视化回放 | `cpf-manager` `【AI-Review】【级别】【类别】【主题】` + 问题/影响/建议(中文) |
| 质量验证 | AACR-Bench:50 仓/200 PR/10 语言/1505 条标注,80+ 工程师交叉标注;同模型 Precision/F1 高于通用 agent,token ≈ 1/9,Recall 刻意换精度 | 无公开基准,仅门禁通过/不通过 |

## 4. 知识库形态对比

**OCR(白盒,已克隆本仓库可查):**
- 内置:`internal/config/rules/rule_docs/` 54 个语言规则文档。`kotlin.md`(134 行)覆盖 9 类:空安全、简洁性、集合操作、协程、类设计、资源管理/作用域函数、性能陷阱、Java 互操作、其他。
- 仓库级:`.opencodereview/rule.json` 按文件路径挂规则;本仓库 dogfood 了一条(providers.go:内联字面量、字段顺序、文档同步、回归测试强制)。
- 用户规则与内置规则可叠加。

**CPF(黑盒,结果可见):**
- 评审配置在 GitCode/DCP 平台侧,仓库内无可见规则文件。
- 知识库只能从 bot 评论反推:批量收集 `cpf-manager` 的 `【AI-Review】` 评论 → 按「级别/类别/主题」聚类 → 还原规则集、豁免逻辑与输出规范。

## 5. 反推 CPF 知识库(已完成,详见《CPF评审知识库反推.md》)

基于 51 个带 `AI已检视` 标签的 PR、130 条 `cpf-manager` 评论(原始数据见 `cpf-ai-review-comments.json`):

- **级别 5 级**:致命 2 / 严重 59 / 一般 56 / 建议 13 / 需人工确认 2
- **类别 8 × 主题 24**:基础代码问题(代码逻辑错误 48、稳定性 16)、语言规范(中文内容 13)、软件设计、安全编码、业务功能、编码规范
- **输出规范**:`【AI-Review】【级别】【类别】【主题】` + ● 问题(行号证据+29% 含触发路径调用链) + ● 影响(含级别理由) + ● 建议(可执行修复)
- **深层能力**:触发路径推理 29%、跨文件一致性对比 16%、基线/上游对照 10%、显式豁免逻辑 7%、PR 流程检查 13%、证据不足降级为「需人工确认」
- **格式已被团队采纳**:5 条同格式评论由真人发出

## 6. OCR 增强方向(基于反推结论)

### 6.1 立即可做(不改编译产物,写规则文件)— 已完成

已写入 `/Users/zhangxinyu/oh/hello/kotlin/.opencodereview/rule.json`(CPF 反推规则沉淀,5 条,均 merge_system_rule):
1. `kotlin-native/konan/konan.properties` — 跨 target flags 一致性 + 基线对照
2. `kotlin-native/runtime/**` — GC STW 热路径纪律、并发、资源生命周期、NAPI 边界
3. `**/{Bitcode,OptimizationPipeline,TopLevelPhases,CAPIExtensions}.{kt,cpp,h}` — 编译管线并行/临时资源/输出死锁
4. `scripts/**` — 凭据、清理、工具可用性、向后兼容
5. `**` — 兜底:英文约定(+testData 字符串豁免)、证据+触发路径要求、注释准确性、失败路径、死代码

**注意:OCR 层内规则按声明顺序首条命中即胜,具体规则必须排在 `**` 之前**。已用 `ocr delegate rule` 验证 5 条规则分别正确绑定对应路径。

### 6.2 Prompt 层增强 — 已完成

- `main_task_system.md`:新增「Cross-File Consistency Checks」(跨 target 配置、穷尽分支、同变更双实现、硬编码常量、注释/文档准确性)、「Comment Structure」(问题=证据+触发路径 / 影响=impact 字段 / 建议)、「Severity Definitions」(含 suggestion 与 needs_confirmation 语义)
- `review_filter_task_system.md`:新增 needs_confirmation 评论保护——缺乏证实证据永远不构成"证明错误",不得移除

### 6.3 Schema 层增强 — 已完成

改动文件(均已通过对应包测试):
- `internal/model/review.go`:LlmComment 增加 `topic`/`impact`/`rule_id`(omitempty),Category/Severity 文档注释更新
- `internal/tool/code_comment.go`:category 增加 `process`,severity 增加 `suggestion`/`needs_confirmation`;解析新字段
- `internal/tool/comment_args_repair.go`:修复 allow-list 同步纳入新字段(测试 TestKnownCommentFieldsCoversTheSchema 抓到的真实缺口)
- `internal/config/toolsconfig/tools.json`:code_comment schema 枚举扩展 + 三个新属性 + content 描述强化(证据/触发路径)
- `internal/viewer/`:server.go 计数与 CSS class、store.go 摄取新字段、session.html 展示 topic/rule_id 徽章与 impact 段、style.css 新增 severity-suggestion/needs-confirmation 与 cat-process 样式(含暗色主题)
- `cmd/opencodereview/`:sarif.go(9 条规则映射,suggestion/needs_confirmation → note)、output.go(终端配色:suggestion 绿、needs_confirmation 品红)、session_cmd.go(过滤补全枚举)

验证:`go build ./...` 通过;`internal/tool`、`internal/model`、`internal/viewer`、`internal/agent`、`internal/config/*` 测试全部通过;`cmd/opencodereview` 仅剩 3 个与本次改动无关的预存失败(TestResolveBackground_AllCases、TestGetCommitMessage×2,已在干净树上复现,为沙箱 git 子进程环境问题)。

### 6.4 输入层增强 — 未做(需立项)

CPF 13% 的检查依赖 **PR body**(模板四要素、测试报告完整性),OCR 输入只有 diff。GitHub Action(`action.yml`)已能拿到 PR body,可将其作为上下文注入 review input,并新增 PR 级规则类型。这是 OCR 与 CPF 能力差距中唯一需要动架构的部分。

### 6.5 输出语言 — 未做

CPF 全中文输出;OCR prompt 全英文。可提供输出语言配置(prompt 已模板化,成本低)。

## 7. 待办(明确暂不做)

- OCR 实测评审 CPF 的 PR。
- OCR 评审结果与 CPF 历史 PR 评论逐条对比(用于量化 Precision/Recall 差异,需实测后做)。

## 附:证据来源

- OCR:本仓库克隆(`~/code/open-code-review/`),README、`internal/config/rules/`、`internal/config/template/prompts/`。
- CPF:GitCode API(匿名,共享账户限流 10W/h,持续 429 时改用网页端);PR #452 评论(openharmony_ci 门禁、cpf-manager AI-Review);org 仓库列表(CPF-KMP-CMP/agent-skills 等)。
