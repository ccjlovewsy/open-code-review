# OCR 审查问题修复报告与七步执行计划

- 任务 ID：`20261105-110347-ocr-report-plan`
- 日期：2026-11-05
- 工作目录：`/Users/issuser/code/open-code-review`
- 分支：`feat/cpf-kb-review-enhancements`
- 基线：`origin/main...HEAD`，HEAD `5bbde2f`
- 报告性质：中文审查报告兼执行计划。本文只描述问题、修复契约和验收方式，不代表 7 个 Step 已经实施或测试已经通过。

## 1. 审查范围与证据

### 1.1 基线与规模

- 基线命令：`git diff origin/main...HEAD`。
- HEAD：`5bbde2f`（`fix(review): harden PR metadata handling`）。
- 提交数：21（`git log --oneline origin/main..HEAD | wc -l` 实测为 21）。
- 变化文件数：77（`git diff --name-only origin/main...HEAD | wc -l` 实测为 77）。
- OCR 选择审查的条目：37。`files_reviewed=37` 是选择/尝试数，不等于 37 个文件都完整审完；第二次运行的 warning 已明确有文件达到工具轮次上限。

### 1.2 两次 OCR 执行

审查工具链：opencode `1.19.0` 会话调用 OCR `v1.13.5`（`ocr --version` 输出 `open-code-review v1.13.5 (abb7abc)`）；驱动模型为会话中的 `volcengine/deepseek-v4.1-flash`，OCR 产物 `llm.provider` 记录为 `volcano-glm`。

| 次序 | OCR 状态 | findings | 严重度分布 | 耗时 | 总 tokens | 备注 |
|---|---|---|---|---|---|---|
| 第一次 | `complete` | 33 | medium 16 / low 17 | 12m1s | 32,618,484 | 无 warning |
| 第二次 | `partial` | 34 | medium 17 / low 17 | 15m52s | 32,946,194 | 4 个文件因工具轮次上限未完成；`code_search` 有 1 次 `git grep` 正则解析失败 |

第二次运行 warning 的 `file` 字段列出 7 个路径：`cmd/opencodereview/output.go`、`review_cmd.go`、`sarif.go`、`scan_cmd.go`、`session_cmd.go`、`shared.go`、`shared_flags.go`。上一会话复核把其中 4 个判定为未完成：`output.go`、`sarif.go`、`scan_cmd.go`、`shared_flags.go`；其余 3 个已有 finding，但可能未完全覆盖。本文沿用“4 个文件未完成”的口径，同时保留原始 7 路径记录，避免把 warning 误读成只涉及 4 个文件。

两次结果存在重复，不能相加为独立问题数：

- 按 `(path, start_line, end_line)` 统计，两次结果有 10 个键重合；并集为 56 个键。
- 56 也不是“已确认独立问题数”：同一位置可能有多条不同主张，且原始 finding 未逐条复现。
- 因此本文只把进入 Step 范围的条目当作待修问题，其他条目按 §5 和附录标注去向。

### 1.3 原始产物绝对路径

- `/Users/issuser/.dsh/delegate-tasks/20261104-160516-ocr-self-review/ocr-review.json`
- `/Users/issuser/.dsh/delegate-tasks/20261104-160516-ocr-self-review/ocr-review-first-completed.json`
- `/Users/issuser/.dsh/delegate-tasks/20261104-160516-ocr-self-review/session.json`

### 1.4 证据使用边界

- `session.json` 中的 thinking 和模型自述不作为代码事实证据；代码事实以 HEAD 上的源码、`git diff` 和 OCR JSON 中的结构化字段为准。
- OCR 的 `severity` 是模型给出的初判，不是安全审计结论；`medium`/`low` 不代表问题一定轻微，`critical`/`high` 为 0 也不代表不存在高危。
- 第二次运行是 `partial`，且 4 个文件未完成，未覆盖文件不能据此判定“没有发现问题”。

## 2. 复核口径

主模型已组织独立只读复核：只读源码、`git` 和上述 JSON，不修改源代码，不把 OCR thinking 当作证据。本文中的 Step 范围是复核后保留的修复项；原始 34 条 finding 的去向见附录。

需要明确的边界：

1. **原始无 critical/high 不等于证明不存在高危。** 本次 OCR 为 `partial`，4 个文件未完成，`code_search` 还发生过 1 次失败；覆盖缺口本身就可能隐藏高危问题。本文不把“无 critical/high”当作安全结论。
2. **`ci_state_passed=false` 的 pending/running 语义未证实。** GitCode 的 `mergeable_state.ci_state_passed=false` 究竟表示失败、排队、运行中还是未触发，没有可靠文档或实测证据。该 API 字段不进入本轮修复；Step 6 只修标签文本解析，不碰这个字段。
3. **nil catalog 只是未来调用风险。** 现有 `*Catalog` 方法对 nil receiver 已有保护，当前没有已知调用路径会传 nil；本轮不为此改代码。零值 `Catalog{}` 的 nil `Namespaces` map 是另一回事，它在 `Merge` 中会实际 panic，作为 Step 4 的验收子项处理。
4. **未逐条复现 34 条 finding。** 本文不宣称 34 条都已人工复现；只对进入 Step 的条目给出可执行验收，其余条目明确标为不进入本轮。

## 3. 优先处理的问题待办清单

- [x] **Step 1：PR 元数据不可信输入与提示词注入** — 2026-11-05 完成。正文引用、结构标签中和、占位符二次展开均已修复；最终审查发现的非 LF 换行绕过也已返修，8 种分隔符测试 RED→GREEN，复审 APPROVE。相关两包测试 exit 0。证据：`~/.dsh/delegate-tasks/20261105-112127-ocr-fix-step1/`。
- [x] **Step 2：resume 身份** — 2026-11-05 完成。PR metadata 与 CatalogDomains/Sources 已纳入身份；4 个相关用例 RED 后转 GREEN。`GIT_CONFIG_GLOBAL=/dev/null` 隔离本机 trace2 干扰后，指定三包测试 exit 0，独立 verifier 复跑 exit 0，critic APPROVE。证据：`~/.dsh/delegate-tasks/20261105-113041-ocr-fix-step2/`。
- [x] **Step 3：领域路由与按需加载** — 2026-11-05 完成。空 Content 路径关键字、review/scan 开关、delegate 跳过可选配置均有真实命令测试；5 个新增行为用例 RED 后转 GREEN。指定两包测试及独立 verifier 均 exit 0，critic APPROVE。证据：`~/.dsh/delegate-tasks/20261105-120623-ocr-fix-step3/`。
- [x] **Step 4：catalog 合并** — 2026-11-05 完成。部分覆盖改为字段级合并，零值 namespaces 不再 panic，新规则按 ID 排序；4 个用例 RED 后转 GREEN。包测试与独立 verifier 均 exit 0，生成 JSON 未改，critic APPROVE。证据：`~/.dsh/delegate-tasks/20261105-121510-ocr-fix-step4/`。
- [x] **Step 5：catalog 生成与校验** — 2026-11-05 完成。支持迁移后的映射表，区分缺 root 与坏 checkout，partial 生成有显式保护。仓库 `-verify` exit 0（checked=5、skipped=291，5+291=212+84），缺文件负例 exit 1；cmd 包测试和独立 verifier 通过，critic APPROVE。证据：`~/.dsh/delegate-tasks/20261105-121954-ocr-fix-step5/`。
- [x] **Step 6：PR 状态解析与 reviews 汇总** — 2026-11-05 完成。DCO 否定/绕过、CI 修饰词否定、GitHub 最新评审状态聚合均已修复；5 组新用例 RED 后转 GREEN。prmeta 包测试与独立 verifier 均 exit 0，未改 `ci_state_passed`，critic APPROVE。证据：`~/.dsh/delegate-tasks/20261105-123114-ocr-fix-step6/`。
- [x] **Step 7：viewer 与输出契约** — 2026-11-05 完成。新增 process chip；ACP 明示 needs_confirmation；SARIF properties/message 保留 disposition 且不降级 level。主仓两包与 ACP 模块均 RED 后 GREEN，独立 verifier 复跑全过，critic APPROVE。证据：`~/.dsh/delegate-tasks/20261105-123836-ocr-fix-step7/`。

勾选规则：只有对应 Step 的代码、测试和验收命令都已完成并留下真实输出摘要时，才把该行从 `[ ]` 改为 `[x]`。只改计划、只写测试、或命令失败时都不得勾选。

## 4. 七步任务

### Step 1：PR 元数据不可信输入与提示词注入

**Files**

- `internal/prmeta/prmeta.go`
- `internal/prmeta/prmeta_test.go`
- `internal/agent/agent.go`
- `internal/agent/prmeta_test.go`（现有 agent 测试；必要时在同一文件扩展）
- 如确实需要新增测试文件，只能放在上述包内，并在本 Step 记录原因。

**问题**

- `internal/prmeta/prmeta.go:563` 的 `Body` 是唯一没有做字段级清洗的多行字段。其他单行字段走 `sanitizeSingleLine`，正文只调用 `neutralizeBlockMarkers`，且后者只处理 `pr_metadata` 标签。
- PR 作者可以在正文中写独立行 `DCO: passed`、`Checks/CI: completed/passed`、`Reviews: ...`，渲染后与真实字段同形，污染 `main_task_system.md` 的 Process Checks 判断。
- `internal/agent/agent.go:1327` 先替换 `{{pr_metadata}}`，再替换 `{{plan_guidance}}` 和 `{{confirmed_comments}}`。PR 字段中出现这两个字面占位符时会被二次展开，等于让不可信输入改写后续提示词结构。
- PR 字段还可以携带 `</user_task>`、`<other_changed_files>`、`<review_files>`、行首 `### ` 等结构标记，`neutralizeBlockMarkers` 目前不处理它们。

**必须达到的行为**

1. **正文仍可读，但不可能伪造字段行。** `Body:` 下的每一行（包括空行和截断提示）统一加固定引用前缀 `> `；不删除正文文本。这样 `DCO: passed` 只能显示为 `> DCO: passed`，不会成为块内独立字段。
2. **所有 provider 控制的文本都要中和提示词结构标签。** 除 `pr_metadata` 外，至少覆盖 `user_task`、`other_changed_files`、`review_files`、`domain_knowledge`；匹配要大小写不敏感，并容忍标签内的空白/控制字符。中和方式必须保留文本（例如把 `<` 替换为 `<\`），不能整段删除。
3. **正文行首结构标记不可生效。** 正文加 `> ` 后，`### Review Checklist` 只能成为 `> ### Review Checklist`；同时结构标签本身也要按第 2 条中和。
4. **禁止二次模板展开。** 把 `{{pr_metadata}}` 的替换放到 `{{plan_guidance}}`、`{{confirmed_comments}}` 之后，或改成不会重扫替换值的单次替换。PR 元数据中的 `{{plan_guidance}}`/`{{confirmed_comments}}` 必须原样保留，不能被展开。
5. **保持空元数据行为。** `PRMetadata == ""` 时仍走 `stripEmptyPRMetadataBlock`，不得留下空标题或空块占位。
6. **不改变提示词模板的既有段落顺序，除非实现证明必须改。** 本 Step 的固定 Files 不含 `main_task_user.md`；优先在 `agent.go` 内调整替换顺序完成。

**TDD 要求（先失败，后实现）**

- `internal/prmeta/prmeta_test.go` 新增 `TestRenderBodyCannotForgeMetadataFields`：正文包含 `DCO: passed`、`Checks/CI: completed/passed`、`Reviews: 1 total (approved: 1, changes requested: 0, commented: 0)`；断言渲染块中真实字段各只出现一次，正文行带 `> ` 前缀。旧实现会多出伪造字段行，测试必须失败。
- 新增 `TestRenderBodyCannotClosePromptStructure`：正文包含 `</user_task>`、`<other_changed_files>`、`<review_files>`、`### Review Checklist`；断言这些结构标记没有以未中和形式出现在 `<pr_metadata>` 块内，且正文文字仍存在。旧实现失败。
- 新增 `TestRenderNeutralizesPromptStructureTagsInSingleLineFields`：标题、标签、warning 等单行字段包含上述结构标签；断言全部被中和。旧实现只处理 `pr_metadata`，失败。
- 新增 `TestRenderBodyStaysReadable`：多行正文、空行、截断提示；断言去掉固定前缀后正文内容仍在，截断提示未被丢弃。
- `internal/agent/prmeta_test.go` 新增 `TestBuildMainTaskMessagesDoesNotExpandPlaceholdersFromPRMetadata`：模板同时包含 `{{pr_metadata}}`、`{{plan_guidance}}`、`{{confirmed_comments}}`；PRMetadata 内含两个字面占位符；调用 `buildMainTaskMessages(..., "PLAN", "CONFIRMED")` 后，元数据中的占位符保持字面量，`PLAN`/`CONFIRMED` 只出现在它们应在的位置。旧实现会二次展开，失败。

**验收命令**

```bash
make test PACKAGES="./internal/prmeta ./internal/agent"
```

**通过判据**

- 上述命令 exit 0，现有 `prmeta` 和 `agent` 测试全部通过。
- 新增用例在旧实现上因对应攻击向量失败，在新实现上通过；提交说明或任务记录中保留失败输出摘要。
- 渲染结果中 `</pr_metadata>` 只出现一次；正文伪造的 `DCO:`/`Checks/CI:`/`Reviews:` 不会再形成独立字段行。

### Step 2：resume 身份

**Files**

- `cmd/opencodereview/review_cmd.go`
- `internal/agent/agent.go`
- `internal/agent/identity.go`
- `internal/config/domainroute/domainroute.go`
- 相应现有测试：`cmd/opencodereview/review_cmd_test.go`、`internal/agent/identity_test.go`、`internal/config/domainroute/domainroute_test.go`

**问题**

- `cmd/opencodereview/review_cmd.go:168` 已经抓取 `prMetadata`，但 `validateResumeIdentity`（同文件 410 行起）构造 `agent.Args` 时没有传入它；实际运行在 251 行才把 `PRMetadata` 交给 agent。结果是同一 commit 在“带 `--pr-url`”和“不带 `--pr-url`”两次运行中 identity 相同，`--resume` 会错误复用旧检查点。
- `internal/agent/agent.go:1082` 把 `DomainRoute.CanonicalConfig()` 并入 `rule_config_sha256`，但 `internal/config/domainroute/domainroute.go:200` 的 `CanonicalConfig` 只序列化 `id/priority/paths/keywords/checks`，没有包含 `renderMatches` 实际渲染的 `CatalogDomains` 和 `Sources`。项目只改这两个字段时，提示词变化而哈希不变。
- `loadPRMetadata` 在 `loadReviewResumeState` 之前执行，这本身是对的：resume 校验需要当前 metadata。不要把它移到校验之后，否则无法把当前快照纳入身份。

**必须达到的行为**

1. **PRMetadata 进入身份。** 在 `ruleConfigSHA256` 中追加一个带标签的 PR 快照字段（建议 `"pr-metadata", a.args.PRMetadata`）。仅当 `PRMetadata != ""` 时追加，避免给无 PR 的运行增加空字段；空↔非空、快照 A↔快照 B 都必须改变哈希。注意本 Step 同时补齐 `CatalogDomains`/`Sources`，默认路由的哈希会按预期变化，不应把这种变化误判为回归。
2. **当前 metadata 传入 resume 校验。** `validateResumeIdentity` 增加 `prMetadata string` 参数，并在构造 `agent.Args` 时设置 `PRMetadata: prMetadata`；`executeReviewContext` 调用它时传入同一个 `prMetadata` 变量。实际运行的 `agent.Args` 必须使用同一份字符串，禁止重复抓取或二次渲染。
3. **CatalogDomains/Sources 进入 CanonicalConfig。** 在每个 domain 的现有字段之后，按配置顺序追加 `catalog_domains` 和 `sources` 的稳定标签和值。不要排序这两个切片，不要改变已有字段的顺序；仅当切片非空时追加，避免给没有这些字段的旧配置制造无意义差异。
4. **不改变无关身份字段。** 不动 source artifact、repository、provider、model、runtime config 的计算；不重复把同一字段加入哈希；不改变既有字段的语义。
5. **失败语义保持一致。** PR 抓取失败仍按现有 warning 降级为空 metadata；此时带 metadata 的父会话因哈希不同而拒绝 resume，这是预期行为，不能静默接受。

**TDD 要求（先失败，后实现）**

- `internal/agent/identity_test.go` 新增 `TestRuleConfigSHA256TracksPRMetadata`：同一 Args 下 `PRMetadata=""`、`"A"`、`"B"` 三种取值的哈希关系为“空≠A、A≠B、A=A”；旧实现三者相同，失败。
- 新增 `TestRuleConfigSHA256TracksCatalogDomainsAndSources`：构造两个只有 `CatalogDomains` 或 `Sources` 不同的 Router，断言哈希不同；旧实现相同，失败。
- `cmd/opencodereview/review_cmd_test.go` 新增 `TestValidateResumeIdentityRejectsChangedPRMetadata`：构造父 `ResumeState`，其 manifest 的 `RuleConfigSHA256` 用 metadata A 计算；调用 `validateResumeIdentity` 时传 metadata B，必须返回身份不匹配错误。旧实现忽略 metadata，会错误通过，失败。
- 新增 `TestValidateResumeIdentityAcceptsSamePRMetadata`：同一 metadata A 时不报身份错误。
- `internal/config/domainroute/domainroute_test.go` 扩展 `TestCanonicalConfig`：断言输出包含 `catalog_domains`/`sources` 的值；旧实现缺失，失败。

**验收命令**

```bash
make test PACKAGES="./cmd/opencodereview ./internal/agent ./internal/config/domainroute"
```

**通过判据**

- 命令 exit 0，现有 resume/identity 测试全部通过。
- 新增用例中至少 3 个在旧实现上失败：PR metadata 哈希不变、catalog 字段哈希不变、resume 错误接受不同 metadata。
- 同一输入重复计算的哈希稳定；除新增的 `pr-metadata`（仅非空）和 `catalog_domains`/`sources` 外，没有其他字段进入 `rule_config_sha256`。

### Step 3：领域路由与按需加载

**Files**

- `internal/config/domainroute/domainroute.go`
- `cmd/opencodereview/shared.go`
- `cmd/opencodereview/review_cmd.go`
- `cmd/opencodereview/delegate_cmd.go`
- `cmd/opencodereview/scan_cmd.go`（必要调用点：任务 Files 未列出，但 scan 的 `--no-domain-routing` 与坏 routes 行为必须同步；不改它就无法完成本 Step 的 scan 验收）
- 必要测试：`internal/config/domainroute/domainroute_test.go`、`cmd/opencodereview/review_helpers_test.go` 或新测试、`cmd/opencodereview/scan_helpers_test.go`、`cmd/opencodereview/delegate_cmd_test.go` 或新测试。

**问题**

- `internal/config/domainroute/domainroute.go:239` 在 `f.Content == ""` 时直接 `continue`，把整段关键字循环跳过；但 `File.Content` 在文档中是可选字段，`KeywordHit` 明确包括“关键字命中文件路径”。内容为空/不可读时，路径关键字也不会命中。
- `cmd/opencodereview/shared.go:149` 无条件执行 `domainroute.Load(repoDir)`，而 `--no-domain-routing` 要等 `loadCommonContext` 返回后才在 `review_cmd.go:149` 和 `scan_cmd.go:132` 置空。坏 routes 会在开关生效前阻断命令。
- `loadCommonContext` 还被 `delegate_cmd.go:98` 复用；delegate preview/rule 不使用路由和 catalog，却会因为这两份可选配置损坏而整体失败。
- `domainroute.Load` 同时会读取 `.opencodereview/rule.json` 的 `domain_routes`；坏配置可能来自两个文件。

**必须达到的行为**

1. **空 Content 不阻止路径关键字命中。** 仅跳过内容匹配：`lowerContent` 为空时，仍然对 `lowerPath` 执行 `strings.Contains(lowerPath, keyword)`。内容非空时保持现有“路径或内容命中即记录一次”的行为，不重复添加 `KeywordHit`。
2. **保留强关键字门槛。** 现有的 `isStrongKeyword`（长度≥6 或含 `.`/`_`）继续约束 keyword-only 匹配。测试用强关键字证明 Content 为空不影响路径命中；不要顺带放宽弱关键字策略。
3. **按需加载。** 给 `loadCommonContext` 增加显式开关（推荐 `commonContextLoadOptions{Catalog, DomainRoutes}` 或等价的两个布尔参数）。`review`、`scan` 传 `Catalog=true, DomainRoutes=!opts.noDomainRouting`；`delegate` 传两者都为 false。开关为 false 时不得读取/解析对应文件，相关字段保持 nil。
4. **坏配置的错误边界。**
   - review/scan 启用路由时，坏 routes 必须继续返回明确的 `load domain routes` 错误，不能吞掉。
   - review/scan 传 `--no-domain-routing` 时，坏 routes 必须被绕过；坏 catalog 仍按现有规则报错，因为 rule_id 校验仍需要 catalog。
   - delegate 不使用路由/catalog，坏 routes 和坏 catalog 都不得阻断 delegate。
5. **`ocr rules route` 诊断命令保持直连。** 它显式需要路由，继续调用 `domainroute.Load`，坏配置仍报错；不要因为本 Step 把它改成“静默无路由”。
6. **最小改动。** 不重构 `Router`、不改变 `Result` 结构、不改路由评分；只改加载时机、开关和空 Content 分支。

**TDD 要求（先失败，后实现）**

- `internal/config/domainroute/domainroute_test.go` 新增 `TestRouteKeywordMatchesPathWhenContentEmpty`：配置一个强关键字（例如 `konan.properties`），文件为 `{Path: ".../konan.properties", Content: ""}`；断言产生 `KeywordHit` 且 domain 被选中。旧实现因提前 `continue` 不命中，失败。
- `cmd/opencodereview` 新增 `TestLoadCommonContextSkipsCatalogAndRoutesForDelegate`：临时仓库放入非法的 `.opencodereview/domain-routes.json` 和 `.opencodereview/rules/catalog.json`，按 delegate 的加载选项调用；断言无错误且 `DomainRoute`/`RuleCatalog` 为 nil。旧实现报错，失败。
- 新增 `TestReviewNoDomainRoutingBypassesBadRoutes`：临时 git 仓库放入坏 routes，调用 `executeReviewContext` 的 preview 路径并设置 `noDomainRouting=true`；断言不返回 `load domain routes`。旧实现在开关生效前报错，失败。
- 新增 `TestScanNoDomainRoutingBypassesBadRoutes`：同样的坏 routes，调用 `executeScanContext`（可用已取消的 context 或让后续步骤尽早返回），断言返回的错误不是 `load domain routes`。旧实现失败。
- 新增 `TestReviewEnabledRoutingStillReportsBadRoutes`：不设 `noDomainRouting`，坏 routes 必须返回 `load domain routes` 错误。该用例在旧实现上也通过，用于锁定错误边界。
- 新增 `TestDelegateBadCatalogAndRoutesDoNotBlock`：坏 routes + 坏 catalog，delegate 路径不报这两个错误。

**验收命令**

```bash
make test PACKAGES="./internal/config/domainroute ./cmd/opencodereview"
```

**通过判据**

- 命令 exit 0，现有路由和命令测试全部通过。
- 三条真实命令路径均有测试：review（preview + `--no-domain-routing`）、scan（`--no-domain-routing`）、delegate（不使用路由/catalog）。
- review/scan 启用路由时坏配置仍报 `load domain routes`；关闭时绕过；delegate 不被坏配置阻断。

### Step 4：catalog 合并

**Files**

- `internal/config/rulecatalog/catalog.go`
- `internal/config/rulecatalog/catalog_test.go`

**问题**

- `internal/config/rulecatalog/catalog.go:160` 对已存在规则整条替换：`c.Rules[existing] = other.Rules[index]`。项目 catalog 用 `parse(data, false)` 解析，非严格校验只检查 `ID` 和 `Source.Path/Line`；项目只写 `{"id":"K1","severity":"high","source":{...}}` 会通过，却把内嵌 K1 的 `title/check/domain/namespace/prefix` 清空，`recount()` 还会把它从 `Counts.OCR` 剔除。
- `internal/config/rulecatalog/catalog.go:191` 直接写 `c.Namespaces[name]`；`Merge` 是导出方法，`Catalog{}` 的 `Namespaces` 为 nil map 时会 panic。`validate` 不强制 `Namespaces` 非空。
- `internal/config/rulecatalog/catalog.go:158` 追加新规则时遍历 `other.byID`（Go map 迭代顺序随机），合并后 `Rules` 尾部顺序不稳定。

**必须达到的行为**

选择“字段级安全合并”，而不是让项目覆盖静默清空内嵌元数据：

| 字段 | 合并规则 |
|---|---|
| `ID` | 必须相同，作为匹配键 |
| `Namespace` / `Prefix` / `Title` / `Domain` / `Severity` / `SourceSeverity` / `Check` / `Status` | 项目值非空才覆盖，空值保留内嵌值 |
| `Source` | 项目 `Source.Path` 和 `Source.Line` 是必填；`Repo` 非空才覆盖，空值保留内嵌值 |
| `AppliesTo` | 项目指针为 nil 时保留内嵌值；非 nil 且至少一个列表非空时覆盖；非 nil 但两个列表都空时保留内嵌值，避免“空对象”清空 |
| 新 ID | 按现有非严格校验接受并追加；不做内嵌元数据合并 |

另外：

1. **零值 Catalog 不 panic。** 写 `c.Namespaces` 前初始化 nil map；`other.Namespaces` 为 nil 时跳过。
2. **追加顺序确定。** 新规则按 ID 升序追加，与 `RuleIDs()` 的排序一致；不要依赖 map 迭代顺序。已存在规则保持原位置。
3. **recount 正确。** 部分覆盖后 K1 仍属于 `ocr` namespace，`Counts.OCR` 保持 5，`Counts.CPF` 保持 207，`ContentHash` 仍清空。
4. **不改生成 JSON。** 本 Step 只改 `catalog.go` 和测试；`catalog.json` 由 Step 5 的生成器处理。
5. **引用和 sources 的现有合并语义不变。** 仍按 ID/Name 去重；`ContentHash=""` 保持。

**TDD 要求（先失败，后实现）**

- `TestMergePartialOverrideKeepsEmbeddedMetadata`：项目 catalog 只提供 K1 的 `severity` 和 `source`；`Load` 后断言 K1 的 `title/check/domain/namespace/prefix` 仍是内嵌值，`Counts.OCR=5`、`Counts.CPF=207`。旧实现清空字段，失败。
- `TestMergeZeroValueNamespacesDoesNotPanic`：`c := &Catalog{Rules: ...}`（`Namespaces` 为 nil）合并一个带 `Namespaces` 的 catalog；断言不 panic 且 namespace 写入成功。旧实现 panic，失败。
- `TestMergeAppendsRulesInDeterministicOrder`：项目新增多个 ID，用不同输入顺序重复合并（或重复运行足够次数），断言新规则尾部始终按 ID 升序。旧实现依赖 map 顺序，失败。
- `TestMergePartialOverridePreservesAppliesTo`：项目只覆盖 severity，断言 `AppliesTo` 未丢。
- `TestMergeKeepsContentHashEmpty`：合并后 `ContentHash` 仍为空。

**验收命令**

```bash
make test PACKAGES=./internal/config/rulecatalog
```

**通过判据**

- 命令 exit 0，现有 `TestLoadProjectOverride` 等测试通过。
- 新增的部分覆盖、零值 map、确定性顺序三个用例在旧实现上失败，在新实现上通过。
- 生成文件未被手工改动：`git diff -- internal/config/rulecatalog/catalog.json` 在本 Step 中应为空。

### Step 5：catalog 生成与校验

**Files**

- `scripts/build-rule-catalog.go`
- 脚本既有测试/测试入口：当前脚本是 `//go:build ignore` 的 main 包单文件，没有 Go 测试入口；本 Step 不新增项目测试文件，改用命令级回归并记录真实输出。
- `Makefile`（仅当默认目标需要同步时）
- `internal/config/rulecatalog/catalog.json`（只能由生成器更新，禁止手改）
- `.github/workflows/ci.yml`（仅当 `-verify` 输出契约需要同步时）
- 只读输入：`ocr_vs_cpf/规则映射表.md`（用户已迁移，禁止移回根目录）

**问题**

- `scripts/build-rule-catalog.go:274` 的 `Counts.CPF != 207` 与 `-allow-missing-sources` 矛盾：缺少 kotlin 上游时 `refsDir==""`，只解析出 5 条 OCR 规则，`Counts.CPF=0`，必然报 `expected 207 CPF rules, got 0`；而 `parseAgentSkills` 的 allowMissing 分支又静默返回 0 references。结果是“一边硬失败、一边静默残缺”，标志实际不可用。
- `scripts/build-rule-catalog.go:749` 的 `verifySources` 把所有 `os.ErrNotExist` 都算作 skipped，包括“root 存在但具体源文件不存在”。这会把真正的漂移/缺文件伪装成跳过；`checked/skipped` 输出不真实。
- `scripts/build-rule-catalog.go:370` 的 `parseOCRRules` 仍读根目录 `规则映射表.md`；用户已把文件移到 `ocr_vs_cpf/规则映射表.md`。`verifySources` 对 `repo=="ocr"` 也按旧路径读。
- `scripts/build-rule-catalog.go:39` 的默认上游路径仍是作者本机的 `/tmp/cpf-kmp-ai-review`、`/tmp/agent-skills-ocr-read-20261029`；当前机器这两个目录都不存在。这个 low 不在本轮 7 步范围，但 Step 5 的“缺失源”策略必须能明确处理空/不存在的 root。

**必须达到的行为**

1. **适配迁移后的规则映射表。**
   - `parseOCRRules` 和 `verifySources` 必须能解析 `ocr_vs_cpf/规则映射表.md`；不得要求把文件移回根目录，也不得在根目录重新创建副本。
   - 优先使用新路径；为兼容旧 catalog 中仍记录 `规则映射表.md` 的 `Source.Path`，`verifySources` 可做“先按记录路径找，找不到再找 `ocr_vs_cpf/规则映射表.md`”的兼容解析。兼容解析只用于读取，不改变 catalog 内容。
   - 如果生成器可以完整运行（两个上游 root 都存在），更新 `reverseMapName` 为 `ocr_vs_cpf/规则映射表.md` 并由生成器刷新 `Source.Path` 和 `content_hash`；当前机器没有上游 root，因此不得手改 `catalog.json` 来伪造这次刷新。
2. **`-allow-missing-sources` 行为一致且明确。**
   - 缺少 kotlin 上游 root 和/或 agent-skills 上游 root 时，允许跳过对应 source；必须在输出中明确列出 skipped source 和原因，并说明产物是 partial。
   - 跳过 kotlin 时不得再执行 `Counts.CPF == 207` 断言；跳过 agent-skills 时 references/catalogs 为 0，并且输出不得声称有 84 references。OCR 5 条规则仍必须解析成功（映射表缺失是错误，不受该标志豁免）。
   - 未提供 `-allow-missing-sources` 时，缺任一上游 root 都硬失败，错误信息必须指出缺的是哪个 root，而不是只报数量不符。
   - root 存在但布局/具体文件缺失（例如 root 下没有 `comment-format.md`、没有 `catalog.generated.yaml`）一律报错，即使带 `-allow-missing-sources`；这属于坏 checkout，不是“源缺失”。
   - partial 构建只能写到显式 `-out` 临时路径；如果 `-out` 仍是默认 `internal/config/rulecatalog/catalog.json`，直接报错，避免残缺 catalog 覆盖完整文件。`-check` 在 partial 状态下直接报错，不比较、不判定“up to date”。
3. **`-verify` 的 root 策略。**
   - 只允许在“对应上游 root 不存在”时按明确策略跳过；`repoRoot` 是当前仓库，不存在直接报错。
   - kotlin/agent-skills root 存在时，逐条读取具体源文件；任一文件不存在必须报错，不能计入 skipped。
   - `repo=="ocr"` 的规则按第 1 条解析迁移后的映射表；root 存在但映射表和兼容路径都不存在时，报错。
   - `checked` 只统计真正读取并核对的规则行/reference；`skipped` 只统计因 root 不存在而整体跳过的条目。输出必须满足 `checked + skipped == rules + references`，并打印每个 root 的跳过原因。
4. **生成文件来源。** `catalog.json` 只能由生成器写；本 Step 若无法完整重生成，保持现有文件不动，验收以 `-verify` 的兼容解析为准。不得手工编辑 `content_hash`、`source.path` 或 counts。
5. **CI 只做必要同步。** 如果 `.github/workflows/ci.yml` 的 `-verify` 步骤需要看到真实 skipped 数量，可调整步骤名/输出说明；不要求在本轮引入上游 vendoring 或漂移检测。原始 finding #2 的完整上游漂移检测仍不进入本轮。

**TDD / 红绿要求（命令级）**

项目没有脚本单测入口，不能假装有 `go test` 覆盖。实施时先在任务目录或临时目录记录旧实现的失败输出，再改脚本：

- **红：root 存在但文件缺失被误跳过。** 在临时目录构造一个最小 catalog，包含一条 `repo:"ocr"`、`path:"ocr_vs_cpf/规则映射表.md"` 的规则；让 `-repo` 指向一个存在的空目录，运行 `-verify`。旧实现走 `os.ErrNotExist -> skipped`，exit 0；新实现必须报“root 存在但源文件缺失”，exit 1。
- **红：`-allow-missing-sources` 与固定数量断言矛盾。** 用不存在的 kotlin/agent root 运行 `-allow-missing-sources -out /tmp/partial-catalog.json`。旧实现先允许缺源，再因 `expected 207 CPF rules, got 0` 失败；新实现必须 exit 0、明确输出 partial、跳过 CPF 数量断言、不声称 84 references，并只写实际解析到的 OCR 规则。
- **红：迁移后 `-verify` 找不到 `ocr_vs_cpf/规则映射表.md`。** 在旧实现上直接 `-verify`，OCR 5 条会因根目录文件不存在而被跳过（或报错，取决于后续策略）；新实现必须核对这 5 条并以 checked 计数。
- **绿：** 修改后重复上述命令，输出与退出码符合预期；再运行仓库级 `-verify`。

**验收命令**

```bash
go run scripts/build-rule-catalog.go -verify
make test PACKAGES=./cmd/opencodereview
make test
```

说明：`make test PACKAGES=./cmd/opencodereview` 不覆盖 `scripts/build-rule-catalog.go`（`//go:build ignore` 的 main 包不在普通包列表中），因此 `go run ... -verify` 是本 Step 的真实脚本验收；最终的 `make test` 仍必须运行，不能省略。

**通过判据**

- `go run scripts/build-rule-catalog.go -verify` exit 0，输出 checked/skipped 真实且满足算术关系；当前机器上 kotlin/agent root 不存在时，skipped 只对应这两个 root，OCR 5 条必须 checked（或按兼容路径 checked）。
- `-allow-missing-sources` 在缺源时不再因 CPF 固定数量失败；输出明确 partial，不静默声称完整。
- root 存在但具体文件缺失的负例 exit 1，且错误信息指出缺失文件。
- `catalog.json` 没有被手工修改；如果上游可用并完整重生成，diff 只能来自生成器输出。

### Step 6：PR 状态解析与 reviews 汇总

**Files**

- `internal/prmeta/prmeta.go`
- `internal/prmeta/providers.go`
- `internal/prmeta/prmeta_test.go`

**问题**

- `internal/prmeta/prmeta.go:678` 的 `computeDCO` 用无边界子串：`strings.Contains(lower, "pass")`、`"success"`，并且先判成功。`dco-bypass`（`bypass` 含 `pass`）和 `dco-not-passed` 都会被判 `DCO: passed`。
- `internal/prmeta/prmeta.go:703` 的 `checkStateFromLabel` 在每个 token 后都执行 `negatedNext=false`；`ci-not-yet-passed` 的 `not` 被 `yet` 清掉，`passed` 按肯定处理，错误返回 `completed/passed`。
- `internal/prmeta/providers.go:208` 的 GitHub reviews 直接统计原始 review 事件，`Total=len(reviews)`；同一评审者先 `CHANGES_REQUESTED` 后 `APPROVED` 会同时增加两个计数；`PENDING`/`DISMISSED` 不进任何分类却进 Total，造成总数与分项矛盾。
- `internal/prmeta/providers.go:423` 的 GitCode `ci_state_passed=false` 语义未证实，明确不进入本 Step；不要顺手改这个字段。

**必须达到的行为**

1. **DCO 标签使用与 check 标签一致的分词/否定逻辑。**
   - `dco-passed`、`dco-success`、`dco-<成功标记>` 判 passed。
   - `dco-failed`、`dco-error` 判 failed。
   - `dco-not-passed`、`dco-bypass`、`dco-unsuccessful`、`dco-no-errors` 以及无法识别的 DCO 标签判 unknown，不能判 passed/failed。
   - 多个 DCO 标签时，未否定的 failure 优先于 passed。
   - 如果存在 DCO 标签但没有任何可结论的状态（包括 `dco-bypass`、`dco-not-passed`），返回 unknown 并以该标签为 basis，不得回退到 `Signed-off-by` 再判 passed；只有完全没有 DCO 标签时才回退到 commit 检查。保留现有 `DCOBasis` 文本风格。
2. **否定标记只在状态词上消费。** `negatedNext` 只能被 failure/running/passed 状态 token 清除；`yet`、`longer`、修饰词和普通 token 不得清除它。现有正向用例必须保持：`test-failed-not-blocking` 仍 failed、`ci-failed-no-retry` 仍 failed、`ci-not-failed-but-passed` 仍 passed、`ci-no-errors-and-passed` 仍 passed。
3. **GitHub reviews 按评审者最新有效状态汇总。**
   - 扩展 `githubReview` 至少包含 `user.login` 和 `state`；必要时带 `submitted_at`/`id` 仅用于排序，不输出。`user.login` 为空时用 review `id` 或数组索引作为独立 key，避免把所有匿名评审误合并成一个评审者。
   - 每个评审者维护一个有效状态：`APPROVED` 或 `CHANGES_REQUESTED`；后出现的 decision 覆盖先出现的 decision。
   - `COMMENTED` 只在评审者尚无 decision 时计数一次；已有 decision 的评审者后续只评论，不丢失其 approval/changes-requested。
   - `DISMISSED` 清除该评审者的有效状态；`PENDING` 忽略。
   - `Reviews.Total` 等于最终有效状态数量，且必须满足 `Total == Approved + ChangesRequested + Commented`；不再使用原始 review 事件数。
4. **`ci_state_passed=false` 不动。** 保持现有 `fetchGitCodeChecks` 映射；本 Step 的验收不覆盖该字段。若要改，必须另立任务并先证实 API 语义。
5. **不改变 `renderReviews` 的输出格式。** 只改计数来源；`DCO`/`Checks` 的渲染文本保持现有风格。

**TDD 要求（先失败，后实现）**

- `TestComputeDCORejectsBypassAndNegatedLabels`：表驱动覆盖 `dco-passed`、`dco-failed`、`dco-not-passed`、`dco-bypass`、`dco-unsuccessful`、`dco-no-errors`；旧实现对前两个 substring 误判，失败。
- `TestComputeDCOFailureOutranksPassed`：同时有 `dco-passed` 和 `dco-failed` 标签，断言 failed。
- `TestComputeDCOInconclusiveLabelDoesNotFallBackToCommits`：所有 commit 都带 `Signed-off-by`，但标签为 `dco-not-passed` 或 `dco-bypass`；断言 unknown，不能因 commit signoff 判 passed。旧实现会判 passed，失败。
- `TestCheckStateFromLabelNegationSurvivesModifiers`：`ci-not-yet-passed`、`ci-not-longer-running`、`ci-no-longer-failing` 均不得返回 passed/failed；旧实现对 `ci-not-yet-passed` 返回 passed，失败。
- `TestCheckStateFromLabelKeepsExistingPositiveCases`：锁定第 2 条列出的 4 个现有用例。
- `TestFetchGitHubReviewsAggregatesLatestPerReviewer`：构造 reviews 序列：alice `CHANGES_REQUESTED` 后 `APPROVED`；bob `PENDING`；carol `APPROVED` 后 `DISMISSED`；dave `COMMENTED` 后 `APPROVED`；erin `APPROVED` 后 `COMMENTED`。断言 approved/changes/commented/total 符合第 3 条，且 `Total == 分项之和`。旧实现按原始事件计数，失败。
- `TestRenderReviewsTotalsConsistent`：渲染后的 `Reviews:` 行分项之和等于 total。

**验收命令**

```bash
make test PACKAGES=./internal/prmeta
```

**通过判据**

- 命令 exit 0，现有 `TestComputeDCO`、`TestCheckStateFromLabel`、GitHub/GitCode fetch 测试全部通过。
- 新增 DCO、否定词、reviews 聚合用例在旧实现上失败，在新实现上通过。
- `dco-bypass`/`dco-not-passed`/`ci-not-yet-passed` 不再产生 passed；reviews 的 total 与分项自洽。
- `ci_state_passed=false` 相关代码和测试无改动。

### Step 7：viewer 与输出契约

**Files**

- `internal/viewer/templates/session.html`
- `internal/viewer/server.go`（仅必要时；现有 `categoryCounts` 已统计 `process`）
- viewer 现有测试：`internal/viewer/server_extra_test.go`、`internal/viewer/server_startserver_test.go`
- `acp/internal/adapter/presentation/report.go`
- ACP 现有测试：`acp/internal/adapter/presentation/report_test.go`
- `cmd/opencodereview/sarif.go`
- `cmd/opencodereview/sarif_test.go`

**问题**

- `internal/viewer/server.go:294` 已把 `process` 计入 `CategoryCount.Process`，`knownCommentCategories` 和 `categoryClass` 也支持它；但 `internal/viewer/templates/session.html:131-139` 的分类筛选栏没有 `process` chip。process 评论会计入 All，却无法用分类 chip 筛出，Other 也不含它们。
- `acp/internal/contract/result.go:96` 有 `Disposition`，但 `acp/internal/adapter/presentation/report.go:185` 的 `writeFinding` 只输出 severity/category/content/existing_code/suggestion_code；`needs_confirmation` 在 ACP 报告中与已确认 finding 完全一致。
- `cmd/opencodereview/sarif.go:65` 的 `sarifResult` 没有 properties；`sarifResultFromComment` 只映射 level/message/locations/fixes/fingerprints，disposition 被静默丢弃，消费端无法机器区分 `needs_confirmation`。

**必须达到的行为**

1. **viewer process chip。** 在 `session.html` 的分类筛选栏加入 `process` chip，字段用 `.Process`，属性为 `data-filter-kind="category"`、`data-filter-value="process"`，文案 `Process (N)`。计数和筛选必须与 All/Other 一致；`categoryCounts` 已有逻辑，不重复造计数。`server.go` 预期不改；只有测试暴露计数缺失时才补。
2. **ACP 显示 needs_confirmation。** `writeFinding` 在 `c.Disposition == "needs_confirmation"` 时输出固定、可搜索的标记，例如 `**Disposition:** needs_confirmation`；`confirmed`/空值保持隐式，不增加噪声。不要删除或改写现有 severity/category/content 渲染。
3. **SARIF 机器可读 disposition。**
   - 给 `sarifResult` 增加 `Properties map[string]string`（`json:"properties,omitempty"`）。
   - 每条 result 都写 `"severity": c.Severity`（原值，包括空串和 legacy 值）和 `"disposition": effective`；`effective` 为空/未知时归一化为 `confirmed`，`needs_confirmation` 保持原值。
   - `needs_confirmation` 的 `message.text` 追加稳定标记 `\n\nDisposition: needs_confirmation`；`confirmed` 的 message 保持原文，避免破坏现有消费端。
   - **不改变 SARIF level。** `sarifSeverityLevel` 继续按 severity 映射；`needs_confirmation` 不因为 disposition 被降级。`critical/high -> error`、`medium -> warning`、`suggestion/legacy -> note` 保持。
4. **不丢字段。** SARIF 的 `ruleId`、`locations`、`fixes`、`partialFingerprints`、invocation notifications 保持现有行为；空 category 仍回退 `other`。
5. **输出兼容。** 现有精确文本断言（如 `TestOutputSARIF_FullFieldMapping` 的 message 原文）不能被无差别追加破坏；只在 `needs_confirmation` 时改 message。ACP 也只在 `needs_confirmation` 时追加。

**TDD 要求（先失败，后实现）**

- viewer：新增 `TestRenderSessionTemplate_ProcessCategoryChip`：构造 `sessionPageData`，包含 1 条 `process` 和 1 条未知 category；渲染 `session.html`，断言 `Process (1)`、`data-filter-value="process"`、`Other (1)`、`All (2)` 同时存在。旧模板无 process chip，失败。
- ACP：新增 `TestFindingDisplaysNeedsConfirmation`：`contract.Comment{Disposition:"needs_confirmation", Content:"x"}` 经 `writeFinding`/`FormatResult` 渲染后包含 `needs_confirmation`；`confirmed`/空值不包含该标记。旧实现缺失，失败。
- SARIF：新增 `TestSarifResultFromComment_DispositionPropertiesAndMessage`：`needs_confirmation` + `severity:"high"`，断言 `properties.severity=="high"`、`properties.disposition=="needs_confirmation"`、message 含标记、level 仍为 `error`。旧实现 properties 为 nil 且 message 无标记，失败。
- 新增 `TestSarifResultFromComment_ConfirmedDispositionIsExplicit`：空 disposition 时 `properties.disposition=="confirmed"`，message 原文不变。
- 新增 `TestSarifResultFromComment_DispositionDoesNotDowngradeLevel`：分别用 critical/high/medium/suggestion 加 `needs_confirmation`，断言 level 与不加 disposition 时相同。

**验收命令**

```bash
make test PACKAGES="./internal/viewer ./cmd/opencodereview"
(cd acp && make test)
```

**通过判据**

- 两条命令均 exit 0，viewer、cmd/opencodereview、ACP 现有测试全部通过。
- 新增 process chip、ACP needs_confirmation、SARIF properties/message/level 用例在旧实现上失败，在新实现上通过。
- process 评论的 chip 计数与 Other/All 自洽；SARIF 消费端能从 properties 读取 disposition 和原 severity；level 未被无依据降低。

## 5. 其他已确认 low

任务契约第 5 条列出的 5 项如下。需要说明：**nil map panic 同时出现在任务第 5 条和 Step 4 的明确要求中**。本报告以 Step 4 的明确验收为准，把它作为 Step 4 的合并契约子项；如果不计入 Step 4，则它不在其余 6 步范围。其余 4 项明确不在本轮 7-step 范围内。

| 项目 | 位置 | 本轮状态 |
|---|---|---|
| nil map panic | `internal/config/rulecatalog/catalog.go:191` | 与 Step 4 同源，按 Step 4 验收；见上方说明 |
| 旧会话 MarkID 漂移 | `internal/viewer/store.go:718`、`internal/viewer/store.go:794-800` | 不在 7-step 范围。历史 severity 迁移后 `commentMarkID` 参与哈希的 severity 变化，旧 localStorage 标记会失配 |
| 旧 severity 过滤 | `cmd/opencodereview/session_cmd.go:454`（过滤实现）、`:210`（补全只列新枚举） | 不在 7-step 范围。旧 session 的 `low` 不会命中 `--severity suggestion` |
| 恰好 100 条误报截断 | `internal/prmeta/providers.go:105-106`、`:123-124`、`:216-217`、`:241-242`、`:325-326`、`:343-344`、`:416-417` | 不在 7-step 范围。`len(...) >= 100` 在恰好整页 100 条时误报“truncated to 100 of 100” |
| 4 个本地化文档 blank line at EOF | `pages/src/content/docs/ja/cli-reference.md:768`、`pages/src/content/docs/ko/cli-reference.md:814`、`pages/src/content/docs/ru/cli-reference.md:748`、`pages/src/content/docs/zh/cli-reference.md:779` | 不在 7-step 范围。当前 `git diff --check origin/main...HEAD` 实测 exit 2，输出正是这 4 条 `new blank line at EOF` |

除上述 5 项外，以下已确认项也不进入本轮，避免 7 个 implementer 任务扩大范围：

- GitCode `ci_state_passed=false` 的 pending/running 语义未证实，不修；见 §2 和 Step 6。
- `scripts/verify-english-only.go:80` 的根目录原始数据豁免涉及外部评审原文、账号和本机路径，属于隐私/合规问题，不在本轮。
- 路由用途的 low：`domainroute.go:394` 的 `(+N more)` 计数、`:404` 的排序契约、`:446` 的 priority 溢出、`:498` 的 `mergeConfig` 永不返回 error、`:524` 的 `rule.json` symlink 校验缺失。
- 命令/解析用途的 low：`route_cmd.go:86` 的 `(+0 more)`、`:102` 的读取错误吞掉；`prmeta.go:846` 的 `isCheckLabel` 子串匹配；`providers.go:204` 的 `statusesOK || checkRunsOK` 把未知当无检查。
- 性能/展示用途的 low：`internal/scan/agent.go:799` 的整文件关键字采样；`internal/viewer/handler.go:135` 的 compare 页丢弃 `Topic/Impact/RuleID`。
- `cmd/opencodereview/review_cmd.go:523` 的 `loadPRMetadata` 把 ctx 取消当 warning 吞掉；该问题不在 Step 2 的固定范围，独立记录。

这些项如果后续要修，应另立任务并单独写回归测试，不要塞进上述 7 个 Step。

## 6. 最终验收

### 6.1 命令

```bash
make check
make test
git diff --check origin/main...HEAD
test -s ocr_vs_cpf/OCR审查问题修复报告.md
test "$(grep -c '^- \[ \] \*\*Step [1-7]' ocr_vs_cpf/OCR审查问题修复报告.md)" = 7
git status --short ocr_vs_cpf/OCR审查问题修复报告.md
```

执行完所有 Step 后，把清单改为已勾选状态，再做：

```bash
test "$(grep -c '^- \[x\] \*\*Step [1-7]' ocr_vs_cpf/OCR审查问题修复报告.md)" = 7
test "$(grep -c '^- \[ \] \*\*Step [1-7]' ocr_vs_cpf/OCR审查问题修复报告.md)" = 0
```

报告本身是未跟踪文件，默认 `git diff` 不会显示；用 `git status --short ocr_vs_cpf/OCR审查问题修复报告.md` 确认它是 `??`。报告行尾空白可用 `grep -n '[[:blank:]]$' ocr_vs_cpf/OCR审查问题修复报告.md` 单独检查。

### 6.2 通过判据

- `make check` exit 0。它包含 `catalog-check`（`go run scripts/build-rule-catalog.go -verify`）、`license-check`、`english-check`、`go mod tidy`、`gofmt`、`go vet`；Step 5 必须让 `catalog-check` 能在迁移后的映射表上通过。
- `make test` exit 0，使用项目标准入口 `LC_ALL=C go test -v -race -count=1`。不得用 `go test` 绕过 `LC_ALL=C`，也不得只跑子包后声称全量通过。
- `git diff --check origin/main...HEAD`：当前 HEAD 实测 exit 2，只有 4 条 `new blank line at EOF`（ja/ko/ru/zh `cli-reference.md`）。这 4 条不在 7-step 范围，最终验收必须如实记录：
  - 若维护者在 7 步之外清掉这 4 个 EOF 空行，命令应 exit 0；
  - 若没有清理，命令仍 exit 2，只能记录为“已知的 4 条非本轮问题”，不能宣称 `git diff --check` 通过，也不能把其他新增空白错误混入这 4 条。
- 报告清单与实际证据一致：每个 `[x]` 都能对应到 Step 章节里的命令、退出码、新增测试名和旧实现失败证据；没有“只改文档就勾选”或“测试失败仍勾选”。
- `git status --short` 中只出现预期改动：本报告、7 个 Step 涉及的文件，以及用户已有的 6 个根目录 Markdown 删除和 `ocr_vs_cpf/` 新增。不得为了通过测试删除或还原用户迁移。

### 6.3 工作区干扰与失败处理

- 工作区已有用户改动：根目录 6 个 Markdown 显示为 `D`，`ocr_vs_cpf/` 显示为 `??`。这些是用户迁移，不是测试产物；最终验收必须保留。
- `make check` 会执行 `gofmt -s -w` 和 `go mod tidy`，可能产生额外改动。运行后必须重新看 `git status` 和 `git diff`，只保留与 Step 相关的格式化/依赖变化；无关变化要说明来源。
- 任何测试失败都必须记录失败的包、测试名、退出码和关键输出。不得用 `-run` 只跑通过的子集来冒充全量通过；不得把环境缺失、上游 checkout 缺失或已知 4 条 EOF 空白说成“全部通过”。
- 本机若出现测试环境干扰（缺少上游 checkout、网络/凭据不可用、旧 session fixture 不兼容等），必须记录为环境阻塞或失败；不得删除用户文件、不得跳过失败测试来制造通过。
- 报告创建阶段未运行构建或测试；各 Step 的后续真实结果已回填到待办清单，最终结果见 §6.4。

### 6.4 实际最终验证（2026-11-05）

- `make check`：exit 0。license、English、catalog verify、go mod tidy、gofmt、go vet 全部通过；日志为 `~/.dsh/delegate-tasks/ocr-final-validation.kioVLq/make-check.log`。
- 根模块完整 `make test`：exit 0。使用 `GIT_CONFIG_GLOBAL=/dev/null` 排除本机 Git trace2 守护进程对临时目录的干扰；日志为同目录 `make-test.log`。
- ACP 模块 `(cd acp && make test)`：exit 0，证据见 Step 7 任务目录。
- 当前工作区 `git diff --check` 和 staged `git diff --cached --check` 均 exit 0。`git diff --check origin/main...HEAD` 仍 exit 2，只包含报告已列出的 4 个既有本地化文档 EOF 空行；这些不属于本轮 7-step。
- 最终 whole-branch 审查曾发现 Step 1 的非 LF 换行绕过；已补测试并修复，原 reviewer 复审 APPROVE，无剩余 Critical/Important。
- 完整上游重生成仍是 UNVERIFIED：本机没有 kotlin-pr-review 和 agent-skills 上游 checkout；本轮只验证缺源策略、合成 fixture 和现有 catalog 的 `-verify`，未声称完整生成已验证。

## 7. 现有文档迁移说明

用户已把根目录 6 个 Markdown 移入 `ocr_vs_cpf/`。`git status --short` 当前显示：

```text
 D CPF平台AI评审对比报告.md
 D CPF评审知识库反推.md
 D OCR-vs-CPF实测对比.md
 D OCR增强待办.md
 D 同类AI评审项目调研.md
 D 规则映射表.md
?? ocr_vs_cpf/
```

目标目录实际文件为：

- `ocr_vs_cpf/202610-OCR增强待办.md`
- `ocr_vs_cpf/CPF平台AI评审对比报告.md`
- `ocr_vs_cpf/CPF评审知识库反推.md`
- `ocr_vs_cpf/OCR-vs-CPF实测对比.md`
- `ocr_vs_cpf/规则映射表.md`
- `ocr_vs_cpf/同类AI评审项目调研.md`

注意：根目录被删除的是 `OCR增强待办.md`，目标目录中的文件名是 `202610-OCR增强待办.md`；这是用户迁移时的重命名，保留现状，不改名、不还原、不在根目录重建副本。

约束：

- 这些删除和新增必须保留；不要 `git add`、`git commit` 或 `git push` 来“整理”它们。
- Step 5 读取规则映射表时，适配 `ocr_vs_cpf/规则映射表.md`；不得把文件移回根目录，也不得在根目录创建同名副本。
- 如果 Step 5 能完整运行生成器，`internal/config/rulecatalog/catalog.json` 中 OCR 规则的 `source.path` 由生成器更新；不能手工改生成文件。当前机器没有 `/tmp/cpf-kmp-ai-review` 和 `/tmp/agent-skills-ocr-read-20261029`，不能假装完整重生成成功。
- 本报告只新增 `ocr_vs_cpf/OCR审查问题修复报告.md`，不修改上述迁移文件，也不修改任何源代码。

## 附录：原始 34 条 finding 的去向

“Step”表示进入本轮对应章节；“不进入”表示已确认但不属于本轮 7 个固定范围，理由见 §5 或 §2。原始编号按上一会话 `findings-final.tsv` 的顺序。

| # | 原始位置/主题 | 去向 |
|---|---|---|
| 1 | `acp/internal/contract/result.go:96` Disposition 无消费者 | Step 7 |
| 2 | `.github/workflows/ci.yml:43` catalog 校验无法发现上游漂移 | 不进入；Step 5 只让 skipped 可见，不做上游 vendoring/漂移检测 |
| 3 | `scripts/verify-english-only.go:80` 根目录原始数据豁免 | 不进入 |
| 4 | `internal/config/domainroute/domainroute.go:239` 空 Content 跳过路径关键字 | Step 3 |
| 5 | `internal/config/domainroute/domainroute.go:200` CanonicalConfig 缺 Catalog/Sources | Step 2 |
| 6 | `internal/config/domainroute/domainroute.go:404` 排序与注释契约不一致 | 不进入 |
| 7 | `internal/config/domainroute/domainroute.go:498` mergeConfig 错误分支是死代码 | 不进入 |
| 8 | `internal/config/domainroute/domainroute.go:524` rule.json 覆盖未做 symlink 校验 | 不进入 |
| 9 | `cmd/opencodereview/route_cmd.go:86` 单个命中输出 `(+0 more)` | 不进入 |
| 10 | `cmd/opencodereview/route_cmd.go:102` 读取错误被吞掉 | 不进入 |
| 11 | `internal/prmeta/prmeta.go:563` Body 可伪造 DCO/Checks 字段 | Step 1 |
| 12 | `internal/prmeta/providers.go:423` `ci_state_passed=false` 被当 failed | 不进入；API 语义未证实 |
| 13 | `internal/prmeta/prmeta.go:678` DCO 子串误判 pass | Step 6 |
| 14 | `internal/prmeta/providers.go:106` 恰好 100 条误报截断 | 不进入 |
| 15 | `internal/prmeta/prmeta.go:846` isCheckLabel 子串匹配 | 不进入 |
| 16 | `internal/viewer/server.go:215` process 分类无筛选 chip | Step 7 |
| 17 | `cmd/opencodereview/shared.go:149` `--no-domain-routing` 生效太晚 | Step 3 |
| 18 | `cmd/opencodereview/review_cmd.go:168` PRMetadata 未纳入 resume 身份 | Step 2 |
| 19 | `cmd/opencodereview/session_cmd.go:210` 旧 severity 过滤失效 | 不进入 |
| 20 | `internal/agent/agent.go:1082` resume 哈希未覆盖 Catalog/Sources | Step 2 |
| 21 | `internal/scan/agent.go:799` scan 整文件关键字采样性能 | 不进入 |
| 22 | `internal/config/rulecatalog/catalog.go:160` Merge 整条替换 | Step 4 |
| 23 | `internal/config/rulecatalog/catalog.go:191` nil Namespaces 写入 panic | Step 4（与 §5 的 nil map 项同源） |
| 24 | `scripts/build-rule-catalog.go:274` allow-missing 与 CPF 数量断言矛盾 | Step 5 |
| 25 | `scripts/build-rule-catalog.go:39` 默认上游路径硬编码 `/tmp` | 不进入 |
| 26 | `internal/config/domainroute/domainroute.go:394` `(+N more)` 计数错误 | 不进入 |
| 27 | `internal/config/domainroute/domainroute.go:446` priority 乘法溢出 | 不进入 |
| 28 | `internal/viewer/store.go:718` 旧会话 MarkID 漂移 | 不进入 |
| 29 | `internal/viewer/handler.go:135` compare 页丢弃 Topic/Impact/RuleID | 不进入 |
| 30 | `internal/agent/agent.go:1327` PR 元数据提示注入/二次展开 | Step 1 |
| 31 | `internal/prmeta/prmeta.go:721` `ci-not-yet-passed` 误判 passed | Step 6 |
| 32 | `internal/prmeta/providers.go:204` 单端点成功即宣告检查已知 | 不进入 |
| 33 | `internal/config/rulecatalog/catalog.go:158` Merge 追加顺序不稳定 | Step 4 |
| 34 | `cmd/opencodereview/review_cmd.go:168` PR fetch 的 ctx 取消被吞 | 不进入 |

独立复核新增、原始 34 条中未列出的契约：

- Step 6 的 GitHub reviews 按评审者最新有效状态汇总。
- Step 7 的 SARIF properties 保留 disposition/原 severity、message 标注、level 不降级。
- Step 5 的迁移路径适配、`-verify` root 策略和 checked/skipped 真实性。
