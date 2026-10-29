# OCR vs CPF 平台 AI 评审实测对比报告

> 日期:2026-10-22(首轮 3 PR);2026-10-29(pr408 重跑);2026-10-29(Stage A 4 PR 实测)。样本:CPF-KMP-CMP/kotlin 的 3 个历史 PR(PR#408/427/426,合计 28 条 cpf-manager 评论)+ Stage A(PR#431/419/447/440,合计 16 条)。
> OCR:增强版本地构建,provider=volcano-glm(GLM 5.3),language=中文,timeout_sec=900,含 CPF 反推沉淀的 `.opencodereview/rule.json`。
> pr408 重跑基准为最终 merge diff c4244058501(含 konan/target 营救);CPF 评论针对中间状态 bd101f9,详见 §8。
> Stage A 基准:431/419/447 取 merge commit 的 `<merge>^1..<merge>` range;440 为 open PR,取 merge-base 4acec4fa..head cfdba2c。评审方式、耗时、tokens 与产物见 §9。
> 原始产物留存本地(每 PR 的 .diff、ocr.json、ocr.log、matching.json)。

## 0. 结论先行

**首轮 3 PR(408/427/426)**

- **首轮(2026-10-22)总 recall:严格口径(同根因)= 6/28 = 21.4%;宽口径(含同主题不同实例)= 11/28 = 39.3%**。
- **pr408 重跑(2026-10-29,对齐最终 merge diff c4244058501,含 konan/target 营救):严格 3/14 = 21.4%(CPF#4、#6、#14);宽口径 6/14 = 42.9%(加 CPF#1、#8、#9)**。营救后 9 个 `konan/target/` 文件全部纳入,新增 12 条评论;首轮「输入侧过滤误杀 7 条」不再成立:#1/#8 转弱重合、#14 转强重合,#2/#7/#10/#11 在文件可见后仍未命中。
- **修正后总 recall(3 个 PR,仅 pr408 重跑):严格 (6-2+3)/28 = 7/28 = 25.0%;宽口径 (11-3+6)/28 = 14/28 = 50.0%**。
- **排除已修复项后**:CPF#3/#5/#12/#13 在 head 已被 52fcab 修复,不计入 OCR miss;CPF#6 虽也已修复,但 OCR 有对应评论,仍保留在有效分母。pr408 有效分母 14-4=10,严格 3/10 = **30.0%**,宽口径 6/10 = **60.0%**;总计有效分母 28-4=24,严格 7/24 = **29.2%**,宽口径 14/24 = **58.3%**。

**Stage A 4 PR(431/419/447/440,16 条 CPF)**

- **原始口径:严格 5/16 = 31.3%;宽口径(强+弱)= 11/16 = 68.8%**。构成:强 5、弱 6、CPF-only 5、OCR-only 54。
- **有效口径:9 条 CPF 评论在 OCR 评审前已被作者修复/补充**(431#3、419#1/#3、447#1/#4/#5/#6、440#2/#4),有效分母 16-9=7;**严格 4/7 = 57.1%,宽口径 6/7 = 85.7%;有效 CPF-only 仅 447#3(GCApi `prctl` 前置条件)**。
- Stage A 的 5 条强重合全部是运行时/编译管线稳定性缺陷(运行时关闭守卫、备份换名、catch 死代码、StackMap 守卫、`cl::opt` 并发);6 条弱重合集中在测试覆盖、错误信号、相邻日志转换等「同文件同主题、不同机制」。
- **两阶段合并(7 PR,raw):严格 12/44 = 27.3%;宽口径 25/44 = 56.8%。有效口径 11/31 = 35.5% / 20/31 = 64.5%**(首轮有效 24 + Stage A 有效 7)。

**横向观察**

- OCR 独有发现首轮 66 条、Stage A 54 条,质量总体扎实(并行编译线程安全、资源泄漏、契约不一致等 high 级发现 CPF 未报);首轮 CPF 独有 17 条中 12 条可补(过滤误杀 7 + 规则已载入未触发 5),重跑后过滤误杀清零,pr408 真实 miss 收敛为 4 条(#2/#7/#10/#11)。Stage A 的 5 条 CPF-only 中 4 条已修复/结构性,真实 miss 仅 1 条。
- CPF 的跨文件穷尽性检查优势有量化证据(28 条中唯一的【致命】评论即此类,重跑后已命中);PR 流程检查(模板/测试报告)依赖 PR body,OCR 架构上暂不支持——属定性差距,本次样本未覆盖(见 §7)。Stage A 431#3 再次复现该结构性缺口,但该条在 merge 前已由作者补充。

## 1. 样本与方法

### 1.1 首轮 3 PR

| PR | cpf-manager 评论 | diff 规模 | OCR 评审方式 | OCR 评论 | 耗时/tokens |
|---|---|---|---|---|---|
| 408(bootstrap) | 14 | 20 文件/1181 行 | 首轮 range M^1..M;重跑 c4244058501(含 konan/target 营救) | 29(首轮) | 30min/2.2M(首轮) |
| 427(C-API 迁移) | 8 | 16 文件/801 行 | worktree(03ce569a40)+ apply + workspace 裸跑 | 18 | 30min/4.0M |
| 426(NAPI 代理串) | 6 | 28 文件/3980 行 | 同上 | 32 | 30min/9.0M |

- 样本选择:按 cpf-manager 评论数取 Top(408:14、403:13、409:12),但 **403/409 的服务端 diff 分别达 537/424 文件,按预案顺移**至 427(8)/426(6)。
- pr408 重跑:9 个营救文件全部纳入,新增 12 条评论;重跑总评论数与耗时未单独统计。

### 1.2 Stage A 4 PR

| PR | cpf-manager 评论 | diff 规模 | OCR 评审方式 | OCR 评论 | 耗时/tokens |
|---|---|---|---|---|---|
| 431(HWASan tag/运行时修复) | 3 | 11 文件/181 行 | range `12a189726f6^1..12a189726f6`(merge commit) | 6 | 30m10s/5.9M |
| 419(debug 增量编译) | 3 | 16 文件/567 行 | range `97a955b9ff5^1..97a955b9ff5`(merge commit) | 15 | 30m31s/10.4M |
| 447(finalizer/attr/log 修复) | 6 | 62 文件/1813 行;过滤 2、评审 60 | range `db249f00cd2^1..db249f00cd2`(merge commit) | 36 | 56m34s/28.2M |
| 440(RSP/KSG in-process) | 4 | 7 文件/114 行(merge-base 口径) | range `4acec4fa..cfdba2c10db9`(open PR) | 10 | 31m54s/3.6M |

- Stage A 合计:CPF 16 条、OCR 67 条、48.2M tokens、2h29m9s。
- 基准口径:431/419/447 均已合并,评审 `merge_commit^1..merge_commit`;440 仍 open,`base` 分支在评论后已前进(2def9483 -> 实际 merge-base 4acec4fa),若按 base tip 取 diff 会混入 51 个上游文件,故按 **merge-base 4acec4fa..head cfdba2c** 取 PR 净 diff,OCR manifest 与该范围一致(7 文件)。
- 447 的 OCR 运行 `terminal_state=partial`:60 个选中文件中 50 完成、10 失败(失败集中在 compiler Kotlin 侧,如 `CodeGenerator.kt`、`IntrinsicGenerator.kt`、`ContextUtils.kt` 等);CPF 评论锚定的 6 个文件均在已完成侧,故本 PR 的 CPF 匹配不受该缺口影响,但其 OCR-only 发现数存在低估。

### 1.3 匹配口径

- **强重合**=同文件+同根因(同一缺陷);**弱重合**=同文件+同主题(同一规则族、不同实例/不同机制);两者之外为 CPF-only / OCR-only。每条判定基于双方评论文本逐条人工核对,并回溯 merge commit/head 代码确认该缺陷在评审基准上是否仍存在。
- **文件级匹配**=CPF 有评论、OCR 也有评论的文件数(用于 §1/§2 的辅助列,不等于评论级重合数)。
- **有效口径**:若 CPF 指出的缺陷在 OCR 评审基准上已被作者修复/补充(含 PR body),则从分母剔除;部分修复仍计为有效 miss(例如 419#2 死代码已清理但工具失败清理仍缺失)。
- Stage A 每条 CPF 的配对、归因、修复状态与 OCR-only 清单见 `/tmp/cpf-compare/pr<PR>-matching.json`(schema `ocr.cpf-matching/v1`)。

## 2. 总表

### 2.1 首轮 3 PR

| | pr408 | pr427 | pr426 | 合计 |
|---|---|---|---|---|
| CPF 评论 | 14 | 8 | 6 | 28 |
| OCR 评论(首轮) | 29 | 18 | 32 | 79 |
| 强重合(pr408 重跑后) | 3 | 1 | 3 | **7** |
| 弱重合(pr408 重跑后) | 3 | 4 | 0 | **7** |
| CPF-only/未匹配(重跑后) | 8 | 3 | 3 | **14** |
| OCR-only(首轮口径) | 25 | 12 | 29 | **66** |

**pr408 重跑:严格 3/14 = 21.4%;宽口径 6/14 = 42.9%**。
**修正后总 recall:严格 (6-2+3)/28 = 7/28 = 25.0%;宽口径 (11-3+6)/28 = 14/28 = 50.0%**(口径:重合数/CPF 评论总数)。
排除已修复项(#3/#5/#12/#13,保留 #6):pr408 有效分母 10,严格 3/10 = 30.0%,宽口径 6/10 = 60.0%;总计有效分母 24,严格 7/24 = 29.2%,宽口径 14/24 = 58.3%。pr408 的 8 条未匹配中 4 条已修复,有效 miss 4 条。
注:pr408 重跑的 OCR 总评论数与 OCR-only 未重算,OCR-only 仍按首轮 66 条记录;重跑新增的 12 条营救文件评论未计入该列。

### 2.2 Stage A 4 PR

| | 431 | 419 | 447 | 440 | 合计 |
|---|---|---|---|---|---|
| CPF 评论 | 3 | 3 | 6 | 4 | **16** |
| OCR 评论 | 6 | 15 | 36 | 10 | **67** |
| 强重合 | 1 | 2 | 1 | 1 | **5** |
| 弱重合 | 1 | 1 | 2 | 2 | **6** |
| CPF-only/未匹配 | 1 | 0 | 3 | 1 | **5** |
| OCR-only | 4 | 12 | 32 | 6 | **54** |
| 有重合的文件数 | 2 | 1 | 3 | 4 | **10** |

**原始口径:严格 5/16 = 31.3%;宽口径 11/16 = 68.8%**。
**有效口径(剔除 9 条评审前已修复/补充):严格 4/7 = 57.1%;宽口径 6/7 = 85.7%;有效 CPF-only 1/7 = 14.3%(447#3)**。
分 PR 有效口径:431 2/2(1 强 1 弱);419 1/1(1 强);447 1/2(1 强,1 有效 miss);440 2/2(1 强 1 弱)。修复状态与证据见 §9.4。

**两阶段合并(7 PR)**:原始严格 12/44 = 27.3%,宽口径 25/44 = 56.8%;有效严格 11/31 = 35.5%,宽口径 20/31 = 64.5%。

## 3. 逐条对照

### pr408(bootstrap;14 条)

| CPF | 内容摘要 | OCR 对应 | 判定 | 归因(CPF-only 时) | 备注 |
|---|---|---|---|---|---|
| #1 严重 | OhosLinker 无条件链 Scrt1.o 进动态库 | 重跑命中(原 Linker.kt 被 `**/target/**` 误杀,营救后可见) | **弱重合**(重跑) | — | 首轮 CPF-only |
| #2 一般 | crtn.o 链接顺序错误 | 重跑可见仍未命中 | CPF-only | 营救后仍漏报(链接顺序未覆盖) | |
| #3 一般 | Usage 引用不存在的脚本名 | 无 | CPF-only | 规则已载入未触发(rule.json 有逐字规则) | PR 在 52fcab 已修复(head 脚本名正确),不计入 OCR miss |
| #4 一般 | -P/-D 明文传递部署凭据 | build-kotlin-ohos.sh:252/:304 [security/high]×2 | **强重合** | — | |
| #5 一般 | allWarningsAsErrors=false 削弱门禁 | 无 | CPF-only | 规则已载入未触发 | PR 在 52fcab 已修复(head 已是 true),不计入 OCR miss |
| #6 一般 | deploy-url 语义向后不兼容 | KotlinBuildPublishingPlugin.kt:164 [suggestion] | **强重合**(OCR 级别偏低) | — | PR 在 52fcab 已修复(head 恢复旧 deploy-url 语义);OCR 有对应评论,仍计入有效分母 |
| #7 严重 | useCompilerDriverAsLinker 与直接调 ld.lld 矛盾 | 重跑可见仍未命中 | CPF-only | 营救后仍漏报 | |
| #8 严重 | 响应文件路径含空格被 lld 拆分 | 重跑命中 | **弱重合**(重跑) | — | 首轮 CPF-only |
| #9 一般 | local.properties 清理残留 | 重跑命中 | **弱重合**(重跑) | — | 首轮 CPF-only |
| #10 严重 | 链接命令缺失 dynamicLibraries | 重跑可见仍未命中 | CPF-only | 营救后仍漏报 | |
| #11 严重 | OHOS_X64 不应列 HWADDRESS | 重跑可见仍未命中 | CPF-only | 营救后仍漏报 | |
| #12 一般 | LD_PRELOAD 硬编码 x86_64 路径 | 无 | CPF-only | 知识库未覆盖(脚本架构路径) | PR 在 52fcab 已修复(head 已删除该段),不计入 OCR miss |
| #13 一般 | rsync 未做可用性检查 | 首轮 build-kotlin-ohos.sh:100 [low](同主题) | 弱重合(首轮) | — | PR 在 52fcab 已修复(head 已删除 rsync 段),不计入 OCR miss |
| #14 致命 | HWADDRESS 新枚举未同步穷尽性 when(build-tools 4 处) | 重跑命中(跨文件穷尽性检查) | **强重合**(重跑) | — | 首轮锚文件 Sanitizer.kt 被误杀;营救后命中 |

### pr427(C-API 迁移;8 条)

| CPF | 内容摘要 | OCR 对应 | 判定 | 归因 |
|---|---|---|---|---|
| #1 严重 | Bitcode.kt:363 中文注释 | 无 | CPF-only | 规则已载入未触发(英文约定规则在 rule.json 且 delegate 验证加载) |
| #2 严重 | 并行路径缺 StackProtector 阶段 | 无 | CPF-only | 知识库未覆盖(并行 phase 完整性) |
| #3 严重 | stripSplitSoTypeInfoFromLlvmUsed 操作错误模块 | ContextUtils.kt:633(merged-module llvm.used 处理缺陷,相邻) | 弱重合 | — |
| #4 严重 | waitFor 后读流死锁 | Bitcode.kt:426(子进程无超时+无限期 waitFor) | 弱重合(同主题不同机制) | — |
| #5 严重 | restoreLlvmUsedComplete 静默失败 | Bitcode.kt:386 [high] 失败路径静默丢弃保活条目 | **强重合** | — |
| #6 严重 | processedModules 空时未写产物 | Bitcode.kt:534(下标错位)+ Config:131(空分区必崩) | 弱重合 | — |
| #7 一般 | 生产代码 println 替代日志 | 无 | CPF-only | 知识库未覆盖(日志规范;CPF 主题「日志输出问题」在反推库中有,rule.json 未沉淀) |
| #8 一般 | restore 末尾死代码 | Bitcode.kt:540 [suggestion] 死代码(同文件不同位置) | 弱重合 | — |

### pr426(NAPI 代理串;6 条)

| CPF | 内容摘要 | OCR 对应 | 判定 | 归因 |
|---|---|---|---|---|
| #1 严重 | NapiRefCleaner detach 线程无退出条件 | ArkTSStringRef.cpp:286 [high] detached 线程无退出条件 | **强重合**(近乎逐字) | — |
| #2 严重 | 析构未加锁 + teardown 后调 TSFN | ArkTSStringRef.cpp:257 [high] 同根因 | **强重合** | — |
| #3 严重 | SubmitTask 未判空 g_mainThreadSafeFunc | ArkTSStringRef.cpp:166 [high] 同函数同根因 | **强重合** | — |
| #4 严重 | NapiScopeGuard 成员声明顺序↔析构顺序相反 | 无 | CPF-only | 知识库未覆盖(C++ RAII 成员顺序规则) |
| #5 严重 | toCharArray destination 未 Pin(NAPI 跨 GC) | 无 | CPF-only | 规则已载入未触发(rule.json 有逐字 Pin 规则) |
| #6 一般 | indexOfString UTF-16 恒真判断+死代码 | KStringProxyOHOS.cpp:359(同函数族,边界语义不一致) | CPF-only(相邻) | 知识库未覆盖(恒真/死分支检测) |

### pr431(HWASan tag/运行时修复;3 条)

| CPF | 内容摘要 | OCR 对应 | 判定 | 归因(CPF-only 时) | 备注 |
|---|---|---|---|---|---|
| #1 严重 | 删除 3 处 `kGlobalRuntimeShutdown` 守卫;shutdown 后重入 `initRuntime` 仍执行 `InitMemory`,进入半关闭态 | Runtime.cpp:166-168 [needs_confirmation] OCR#4 | **强重合** | — | 同文件、同一被删守卫、同一触发链;OCR 级别低于 CPF 严重 |
| #2 严重 | `VisitGlobalPreforwardRoots` 新实现缺少全局/静态根 preforward 的集成回归测试 | KNRootVisitorTest.cpp:44-45 [needs_confirmation] OCR#6 | **弱重合** | — | 同测试覆盖主题;CPF 要 native.tests 集成用例,OCR 指出新增单测在非 OHOS_ARM64+CRT 配置下为空 TU;merge 前已补单测但未补集成用例 |
| #3 一般 | PR body 为模板,变更描述/测试情况/适配验证均为空 | 无 | CPF-only | 结构性缺口(OCR 不读 PR body) | merge 前作者已补充变更描述与测试情况,不计入 OCR miss |

### pr419(debug 增量编译;3 条)

| CPF | 内容摘要 | OCR 对应 | 判定 | 归因(CPF-only 时) | 备注 |
|---|---|---|---|---|---|
| #1 严重 | 第二次 `renameTo` 失败时删除备份,原始链接产物丢失 | DebugInfoGCCompress.kt:56-64 [medium] OCR#6 | **强重合** | — | merge 已把 delete 分支改为 restore;OCR 仍命中同一换名失败链的残余(恢复返回值未检查、预存 `.backup` 未清理) |
| #2 一般 | `reportCompilationError` 返回 `Nothing`,catch 内清理不可达;`KonanExternalToolFailure` catch 不可达 | DebugInfoGCCompress.kt:66-72 [medium] OCR#7 | **强重合** | — | merge 已删死 catch 并把 IOException 清理前移;工具失败路径仍以 `KonanCompilationException` 穿透、临时产物不清理,OCR#7 命中该残余 |
| #3 严重 | 用 `outputFile` 存在性推断本轮 GC 产出,无法区分上轮失败遗留的 stale 文件 | DebugInfoGCCompress.kt:46-50 [medium] OCR#8 | **弱重合** | — | merge 已在阶段头预删除 stale output;OCR 改报回退分支原地改写最终产物无备份,属同一产物完整性规则族的不同机制 |

### pr447(finalizer/attr/log 修复;6 条)

| CPF | 内容摘要 | OCR 对应 | 判定 | 归因(CPF-only 时) | 备注 |
|---|---|---|---|---|---|
| #1 严重 | `ThrowInvalidMutabilityException` 用 `RuntimeAssert(false)`,`kIgnore/kLog` 下不终止,违反 `RUNTIME_NORETURN` | Exceptions.h:51 [suggestion] OCR#24 + Arrays.cpp:45-50 [medium] OCR#36 | **弱重合** | — | 同函数/同 exception 语义;merge 已改 `RuntimeFail`;OCR 改报 `Throw` 命名/契约与 `mutabilityCheck` 测试缺口 |
| #2 严重 | `CollectStackMapBaseRoot` 删除 `IsPlausibleStackMapSlot` 守卫,误分类帧可触发 GC 线程 SIGSEGV | ConcurrentMark.cpp:194-195 [high] OCR#12 | **强重合** | — | 同文件、同一被删守卫、同一 `markInSTW` 触发链 |
| #3 一般 | `SafeAlloc` 的 `prctl(PR_SET_VMA_ANON_NAME)` 去掉 `useMmap` 短路,calloc 分支也会执行 | 无 | CPF-only | **知识库未覆盖**(系统调用前置条件/平台分支) | merge 代码仍缺 `useMmap` 守卫;Stage A 唯一有效 miss |
| #4 建议 | GC 调度日志先强转 `uint32_t` 再除,>4GB 截断;注释写成 KB;`%d` 应为 `%u` | HeapGrowthController.hpp:57-62 [medium] OCR#18 | **弱重合** | — | merge 已改为先除后截断、`%u` 并修正注释;OCR 改报同一语句的残余风险(NaN/Inf 转 `uint32_t` 为 UB、日志位于 `isfinite` 守卫之前) |
| #5 一般 | `DumpMemoryToFile` 创建权限从 0600 放宽为 0666 | 无(同文件 OCR#20 为平台门控) | CPF-only | merge 前已恢复 0600 | 不计入 OCR miss |
| #6 严重 | `FindCallSiteRow` 删除 `framePC < startPC` 校验,无符号回绕可 alias 到错误行 | 无 | CPF-only | merge 前已恢复守卫 | 不计入 OCR miss |

### pr440(RSP/KSG in-process;4 条)

| CPF | 内容摘要 | OCR 对应 | 判定 | 归因(CPF-only 时) | 备注 |
|---|---|---|---|---|---|
| #1 严重 | `StatepointStubPhase` 翻转进程级 `cl::opt`;fragment 级并行下标志复位/写-写竞争导致 KSG no-op 或 UB | Bitcode.kt:199-201 [high] OCR#5 + CAPIExtensions.cpp:27-34 [high] OCR#3 | **强重合** | — | 同根因;两处 OCR 评论分别从调用方与 C API 侧复现同一 fragment 并行触发链 |
| #2 一般 | `LLVMKotlinSetRegisteredFlag` 在选项未注册时静默返回,编译成功但 KSG 为 no-op | CAPIExtensions.h:37-42 [low] OCR#4 | **弱重合** | — | head 调用方已加 `require` 兜底使未注册选项编译失败;callee 仍静默返回 0,OCR#4 报的是返回 1 不能证明 set 生效的契约问题 |
| #3 一般 | 新增 `StatepointStubPhase` 管线未新增自动化测试,仅手工验收 | KonanPropertiesStatepointTest.kt:24-27 [suggestion] OCR#2 | **弱重合** | — | 同测试覆盖主题;CPF 列三条端到端用例,OCR#2 只指出新增测试钉住了契约的一半(未断言 KSG 标志已移除) |
| #4 一般 | 注释称空 `llvmStatepointPasses` 跳过 in-process 管线,但 `ifEmpty` 使其回退默认 pass,注释与行为矛盾 | 无(同文件 OCR#1 为跨目标一致性) | CPF-only | head 已删除 `llvmStatepoint*` 属性与注释 | 不计入 OCR miss |

## 4. 归因 Top 榜

### 4.1 首轮(CPF-only 17 条)

| 归因 | 条数 | 说明 |
|---|---|---|
| **输入侧过滤误杀**(`**/target/**` 误伤 `konan/target/` 源码包) | **7** | 全部在 pr408;一个默认排除模式砍掉该 PR 一半的 recall 上限(7/14=50%),其中含 CPF 唯一的【致命】跨文件发现 |
| **规则已载入但未触发** | 5 | rule.json/delegate 验证确认规则加载,模型未对具体实例开火(408:Usage 脚本名、allWarningsAsErrors、local.properties 清理;427:中文注释;426:NAPI Pin) |
| **知识库未覆盖** | 5 | 脚本架构硬编码路径、并行 phase 完整性、日志规范(println)、C++ RAII 析构顺序、恒真/死分支检测 |

### 4.2 pr408 重跑后(CPF-only/未匹配 14 条,有效 miss 10 条)

| 归因 | 条数 | 明细 |
|---|---|---|
| 已修复,不计入 OCR miss | 4 | pr408 #3/#5/#12/#13(52fcab) |
| 营救解锁后仍未命中(链接类) | 4 | pr408 #2/#7/#10/#11 |
| 规则已载入但未触发 | 2 | pr427 #1、pr426 #5 |
| 知识库未覆盖 | 4 | pr427 #2/#7、pr426 #4/#6 |
| **合计** | **14**(有效 10) | |

口径闭合:首轮 17 = 过滤误杀 7 + 规则未触发 5 + 知识缺口 5;重跑后过滤误杀 7 中 3 转重合、4 仍漏,规则类 5 中 2 修复、1 转重合(#9)、2 仍漏,知识缺口 5 中 1 修复(#12)、4 仍漏,另 #13 从弱重合转为已修复;未匹配合计 14,有效 miss 10。

### 4.3 Stage A(CPF-only 5 条,有效 miss 1 条)

| 归因 | 条数 | 明细 |
|---|---|---|
| 评审前已修复/补充,不计入 OCR miss | 4 | 431#3(PR body)、447#5(0600)、447#6(PC 守卫)、440#4(`llvmStatepoint*` 属性/注释删除) |
| **知识库未覆盖(唯一有效 miss)** | **1** | 447#3:GCApi `SafeAlloc` 的 `prctl(PR_SET_VMA_ANON_NAME)` 去掉 `useMmap` 短路;rule.json 的 runtime 规则未覆盖「系统调用前置条件/平台分支」 |
| **合计** | **5**(有效 1) | |

另有 5 条已修复项仍被 OCR 相邻评论命中(419#1、419#3、447#1、447#4、440#2),按 pr408 #6 口径保留在强/弱重合计数中,但不计入有效分母;9 条已修复项的证据见 §9.4。

### 4.4 Stage A 弱重合的机制差异(6 条)

| CPF | OCR | 差异 |
|---|---|---|
| 431#2 全局根 preforward 缺集成测试 | OCR#6 新增单测在非 CRT 配置下为空 TU | 集成用例 vs 单测可执行性 |
| 419#3 stale outputFile | OCR#8 回退分支原地改写无备份 | 上轮残留复用 vs 本轮写入中断 |
| 447#1 `RuntimeAssert` 不终止 | OCR#24 `Throw` 命名/契约 + OCR#36 测试缺口 | 终止语义已改 `RuntimeFail`,OCR 转向契约与覆盖 |
| 447#4 >4GB 强转截断 | OCR#18 NaN/Inf 转 `uint32_t` UB + 日志先于守卫 | 截断 vs 非有限值 UB |
| 440#2 未注册选项静默返回 | OCR#4 返回 1 不能证明 set 生效 | callee 静默 miss vs 返回值语义 |
| 440#3 整体缺测试 | OCR#2 新测试只钉住契约一半 | 广度 vs 具体断言缺口 |

## 5. OCR-only 发现概览

### 5.1 首轮(66 条,逐条性质见附录 A)

- **实质 bug 类(高价值)**:并行路径共享编译器状态线程安全(Bitcode.kt:494 [high])、未检查强转必崩路径(:583 [high])、tmpContext 泄漏(:544)、TSFN 生命周期与 GC 卸载链路(ExtraObjectData.cpp:96)、@Escapes 注解契约与零拷贝逃逸矛盾(NapiInterface.kt:125 [high])等——**CPF 均未报**。
- **流程/配置类(CPF 未涉足的矿脉)**:pr408 上 OCR 对 OAT.xml 连发 9 条(process 类别生效:许可证文件缺失、版权主体不一致、中文全角逗号违反英文约定、转义对照表写反),CPF 对 OAT.xml 零评论。
- **新 schema 运转良好**:needs_confirmation 用于「llvm-split 输出契约未经验证」「多 napi_env 假设」等证据不足场景(共 9 条),suggestion 用于非阻塞建议。级别对齐:pr426 三条强重合 CPF 严重 ↔ OCR high 全部一致;唯一例外是 pr408 #6,OCR 以 suggestion 报了 CPF 判「一般」的行为兼容性问题(低评一级,已由 §6 校准完成)。
- **代价**:3 个 PR 合计约 15.3M tokens;OCR-only 中打磨+流程类合计 19/66≈29%,存在噪声矿脉(OAT.xml 的 9 条里 5 条为打磨类)。
- 注:本节为 2026-10-22 首轮统计;pr408 重跑在 9 个营救文件上新增 12 条评论,OCR-only 未重算。

### 5.2 Stage A(54 条)

- **高价值类别**:419 的 `NATIVE_DEBUGINFO_MINIFY` 默认值语义反转(OCR#1)、自证式测试(OCR#4)、`introducedVersion` 元数据错误(OCR#14);447 的 CRT `StopFinalizerThreadIfRunning` 必 abort(OCR#16)、`ExtraObjectData` 放行 STACK tag(OCR#8)、HiDebug shutdown 后回调复活 runtime(OCR#23)、OOM 转储逻辑扩散到非 OHOS 平台(OCR#20)、`destroyExtraObjectData` CRT 分支分叉(OCR#21)、`-Werror` 门禁移除(OCR#29);440 的 ohos_x64 未同步 `-disable-llvm-passes`(OCR#1)、`emitRuntimeOpt=RUNTIME` 路径绕过 KSG(OCR#6)、`addOccurrence` 重复复位风险(OCR#7/#9);431 的 `RUNTIME_SWITCH` 掩码分叉(OCR#1)、HWASan 有效位在转发后被清零(OCR#5)、llvmInterop 硬编码库列表(OCR#3)。
- **噪声/低价值**:447 的注解一致性清扫(OCR#32-#35)、kdumputil 文档/格式(OCR#5/#7/#25)、`OhosExecutor` workaround 注释(OCR#26/#30/#31);419 的风格类(OCR#5/#10/#11/#13)。Stage A 的 54 条中,实缺陷与测试缺口仍占多数,但相比首轮,OAT.xml 式流程矿脉消失(Stage A 样本无 OAT.xml 变更)。
- **计数口径**:54 = 67 条 OCR 评论 - 13 条被 §3 配对引用(431 2 条、419 3 条、447 4 条、440 4 条)。逐条清单见各 `pr<PR>-matching.json` 的 `ocr_only`。

## 6. 对 OCR 知识库与 rule.json 的补强建议(按优先级)

1. ~~修 `**/target/**` 误杀(输入层,收益最大)~~ **已完成(2026-10-29,目标仓 rule.json)**:通过 `"include": ["**/konan/target/**"]` 营救,pr408 重跑 9 个文件全部纳入,新增 12 条评论;原 7 条过滤误杀中 #1/#8 转弱重合、#14 转强重合,#2/#7/#10/#11 仍漏报。默认排除模式 `**/target/**` 本身未改,建议后续收敛为精确构建产物模式,避免其他 Kotlin/Native 仓库复现。
2. ~~规则触发率问题(prompt 层)~~ **已完成(Project Rule Sweep)**:`main_task_system.md` 增加逐条 user rule 巡检指令(a269ee5),`scan_template.json` 同步到 scan 模式(66c45d9);规则类未命中由首轮 5 条降为 2 条(pr427 #1、pr426 #5;首轮 5 条中 #3/#5 已修复、#9 已命中)。
3. **补 4 条知识库缺口到 rule.json(待办,全部来自本次 CPF-only 实测)**:
   - 脚本:禁止按架构硬编码多库目录路径(/usr/lib/x86_64-linux-gnu),应按 uname -m 推导;
   - 编译管线:并行化 phase 改造必须对照顺序路径 phase 清单(穷尽性);
   - 通用:生产代码禁用 println/stdout 充当日志,走统一日志设施(CPF「日志输出问题」主题);
   - C++:RAII 成员声明顺序与析构顺序语义(注释意图 vs 声明逆序)。
4. **恒真/死分支检测**交给后续规则沉淀(tautology 模式:`it = begin() + (ptr - data())` 后比较 `ptr == &*it`),低优先。
5. ~~级别校准~~ **已完成(66c45d9)**:`main_task_system.md` 已将「CLI/属性语义向后不兼容」从 suggestion 提到 medium。
6. **重跑残余漏报(新增)**:pr408 #2/#7/#10/#11 在原过滤误杀解除后仍未见,分别对应 crtn.o 链接顺序、compiler driver 与 ld.lld 矛盾、响应文件空格、dynamicLibraries 完整性;建议并入 `R-xplat-5` 的逐项 checklist。

## 7. 与CPF 评审的结构性差异(实测确认)

- CPF 独有的 PR 流程检查(模板四要素、测试报告完整性)依赖 PR body,OCR 输入层暂不支持(见对比报告 §6.4,本次未做)。
- CPF 的「触发路径」写作规范在 OCR 增强 prompt 中已生效(本次 OCR 多条评论含调用链),但 OCR 的证据呈现更偏代码摘录,CPF 更偏运行时后果推演——风格差异,非能力差距。
- 双方在「资源生命周期/并发/NAPI 边界」上高度重合(pr426 三条逐字级),说明该领域规则一旦显式沉淀,两家模型都能稳定命中——**知识库沉淀比模型差异更决定 recall**。

## 8. pr408 基准 revision 与重跑验证

### 8.1 CPF 评论的基准 revision

| 项 | revision/说明 |
|---|---|
| CPF 评论基准 | `bd101f9`(pr408 PR 中间状态) |
| pr408 最终 merge | `c4244058501` |
| 最终修复提交 | `52fcab`(修复 CPF#3/#5/#6/#12/#13) |
| OCR 重跑基准 | `c4244058501` + 目标仓 rule.json `include: ["**/konan/target/**"]` |

CPF 评论针对的是 PR 中间状态,而非最终 merge diff。`52fcab` 在最终合并前修复了 5 条:Usage 脚本名(#3)、allWarningsAsErrors(#5)、deploy-url 旧语义(#6)、LD_PRELOAD 硬编码段(#12)、rsync 可用性段(#13)。直接拿 CPF 评论总数作分母会低估 OCR:重跑看到的是已修复代码,不可能再对已删除或已修正的缺陷发评论。本报告同时保留原始口径(总计 28、pr408 14)与排除已修复项后的有效口径(总计 24、pr408 10);#6 因 OCR 仍有对应评论而保留在有效分母。

| pr408 指标 | 首轮 | 重跑 | 重跑(排除已修复) |
|---|---|---|---|
| 严格 | 2/14 = 14.3% | 3/14 = 21.4% | 3/10 = 30.0% |
| 宽口径 | 3/14 = 21.4% | 6/14 = 42.9% | 6/10 = 60.0% |

### 8.2 重跑验证记录

- **konan/target 营救**:目标仓 rule.json 加入 `**/konan/target/**` 后,pr408 的 9 个被误杀文件全部纳入评审范围,OCR 在这些文件上新增 12 条评论;原 7 条过滤误杀中 #1/#8 转弱重合、#14 转强重合,#2/#7/#10/#11 仍漏报。
- **synthetic diff 验证**:构造含 3 条注入问题的 diff 重跑,OCR 输出 6 条评论,全部命中 3 条注入问题(3/3)。

## 9. Stage A 过程记录(431/419/447/440)

### 9.1 评审方式与基准

| PR | 状态 | 基准/merge commit | OCR range | 变更/评审/完成/失败(文件) | CPF/OCR 评论 | 耗时 | tokens |
|---|---|---|---|---|---|---|---|
| 431 | merged | `12a189726f6` | `12a189726f6^1..12a189726f6` | 11/11/11/0 | 3/6 | 30m10s | 5,914,301 |
| 419 | merged | `97a955b9ff5` | `97a955b9ff5^1..97a955b9ff5` | 16/16/16/0 | 3/15 | 30m31s | 10,446,636 |
| 447 | merged | `db249f00cd2` | `db249f00cd2^1..db249f00cd2` | 62/60/50/10 | 6/36 | 56m34s | 28,233,008 |
| 440 | open | base `2def9483b455` / head `cfdba2c10db9` / merge-base `4acec4fa` | `4acec4fa..cfdba2c10db9` | 7/7/7/0 | 4/10 | 31m54s | 3,616,611 |
| **合计** | | | | | **16/67** | **2h29m9s** | **48,210,556** |

- provider/model 均为 `volcano-glm` / `glm-5.3`,OCR `v1.13.1`,并发 8,仓库路径 `/Users/zhangxinyu/oh/hello/kotlin`。
- 变更文件数:431 `111+/70-`;419 `567+`;447 `1379+/434-`(过滤 2 个 testData 文件);440 `104+/10-`(merge-base 口径)。
- 447 的 10 个失败文件:`NativeSecondStageCompilationConfig.kt`、`CodeGenerator.kt`、`ContextUtils.kt`、`IntrinsicGenerator.kt`、`IrToBitcode.kt`、`LlvmAttributes.kt`、`LlvmUtils.kt`、`RTTIGenerator.kt`、`Runtime.kt`、`CompilerCInterface.cpp`。CPF 评论锚定的 `Exceptions.cpp`、`ConcurrentMark.cpp`、`GCApi.cpp`、`HeapGrowthController.hpp`、`AllocatedSizeTracker.cpp`、`StackMapTable.cpp` 均已完成。

### 9.2 运行状态与异常

- **terminal_state**:431/419/440 为 `complete`;447 为 `partial`(60 选中、50 完成、10 失败)。447 的 OCR-only 数量因此存在低估,CPF 匹配不受影响。
- **LLM 重试**:431 总请求 105、失败 2;419 总请求 216、失败 2;447 总请求 541、失败 6、取消 2;440 总请求 97、失败 1。失败均发生在 Round-2(deadline exceeded),首轮评论完整;`warnings` 中有多条 `comment_refiled`(评论被重新挂到更准确的 diff 文件)。
- **CPF 快照差异**:447 的 CPF API 另有 1 条 2026-10-19 的 AI 评论(`DBG_OOM` 日志级别,AllocatedSizeTracker.cpp:34),不在 Stage A 冻结的 6 条快照内;若计入,原始严格 5/17 = 29.4%、宽 11/17 = 64.7%,且该条无 OCR 对应,不影响本报告主口径。

### 9.3 产物

| 产物 | 路径 |
|---|---|
| OCR 完整输出 | `/tmp/cpf-compare/pr<PR>-v2.json` |
| OCR 运行日志 | `/tmp/cpf-compare/pr<PR>-v2.log` |
| 匹配结果 | `/tmp/cpf-compare/pr<PR>-matching.json` |
| CPF 评论快照 | `/tmp/cpf-compare/pr<PR>-cpf.json` |
| PR 元数据 | `/tmp/cpf-compare/pr<PR>-meta.json` |

- `matching.json` 的 schema 为 `ocr.cpf-matching/v1`,包含 `review`(基准/耗时/tokens/terminal_state)、`counts`、`effective`、`pairs`(CPF↔OCR 配对与判定理由)、`cpf_only`、`ocr_only`。每条的 `status` 为 `open` / `fixed_before_review` / `partially_fixed`。

### 9.4 评审前已修复/补充的 9 条 CPF 评论

| PR# | CPF 摘要 | 评审基准上的状态 | 证据 |
|---|---|---|---|
| 431#3 | PR body/测试情况为空 | 已补充 | `12a189726f6` PR body 含变更描述与测试情况 |
| 419#1 | rename 失败删除备份 | 已改 restore 分支 | `97a955b9ff5` `DebugInfoGCCompress.kt:60-64` |
| 419#3 | stale outputFile 复用 | 阶段头预删除 stale output | `97a955b9ff5` `DebugInfoGCCompress.kt:36` |
| 447#1 | `RuntimeAssert(false)` 可能不终止 | 已改 `RuntimeFail` | `db249f00cd2` `Exceptions.cpp` |
| 447#4 | 先强转后除/%d/注释 KB | 已改先除后截断、`%u`、MiB 注释 | `db249f00cd2` `HeapGrowthController.hpp:18,57-62` |
| 447#5 | OOM dump 权限 0666 | 已恢复 0600 | `db249f00cd2` `AllocatedSizeTracker.cpp` `open(..., 0600)` |
| 447#6 | `FindCallSiteRow` 缺 `framePC < startPC` 守卫 | 已恢复守卫 | `db249f00cd2` `StackMapTable.cpp` |
| 440#2 | 未注册选项静默返回 | 调用方加 `require` 兜底(callee 仍静默) | `130e928` / head `cfdba2c` `Bitcode.kt` |
| 440#4 | `llvmStatepointPasses` 注释与 `ifEmpty` 矛盾 | 属性与注释已删除 | head `cfdba2c` `konan.properties` |

有效分母推导:16(全部)- 9(上表)= 7;其中 431#2(已补单测未补集成)与 419#2(死代码已清、工具失败清理仍缺)按部分修复保留。有效匹配 6 条(强 4、弱 2),有效 CPF-only 1 条(447#3)。

## 附:方法偏差说明

- pr408 首轮用已合并 commit 的 M^1..M range 评审(与 PR diff 逐文件等价,已核对 20 文件清单一致);2026-10-29 在最终 merge diff c4244058501 上重跑(含 konan/target 营救)。
- pr403/409 因服务端 diff 过大(537/424 文件)按预案顺移为 427/426。
- 逐条判定由主模型直接核对双方原文完成,未再拆子代理(spec 允许子代理但要求复核原文,直接核对等效且更稳)。
- kernel 噪声:首轮三次评审各有一次 Round-2 重试/失败(网关偶发),均未影响最终产出(complete,comments 齐全)。Stage A 同样存在 Round-2 deadline,447 另有 10 个文件失败(terminal_state=partial),已在 §9.1/§9.2 披露。
- 2026-10-23 修正轮:经增强版 OCR 对本报告自审(15 条发现)后修正:OCR-only 计数闭合(66=25+12+29,剔除 3 条误入的重合对应评论)、pr408 误杀占比口径(该 PR 的 50%)、pr408 token 数(实际 2.2M)、pr426 主题标注、needs_confirmation 计数(9 条)、§0/§4/§5 多处表述;匹配判定与结论不变。
- 2026-10-29 重跑轮:pr408 对齐 c4244058501,标记 52fcab 已修复的 5 条(有效分母剔除 #3/#5/#12/#13、保留 #6),刷新总 recall;konan/target 营救与 synthetic diff 验证结果见 §8。
- 2026-10-29 Stage A 轮:431/419/447 按 merge commit `^1..merge` 评审,440 按 merge-base..head 评审;匹配时额外回溯 merge/head 代码判定 CPF 缺陷是否仍存在,9 条评审前已修复/补充的评论单列(§9.4),同时保留原始口径与有效口径。447 的 CPF 快照为 6 条冻结集,API 中第 7 条(10-19)未计入主口径。

## 附录 A:OCR-only 逐条性质表(66 条)

性质标签:【真】实质缺陷/契约问题(建议修)【流程】合规/流程发现(依赖仓库治理上下文)【打磨】低价值风格/文档噪音【待确认】needs_confirmation(证据不足,非缺陷断言)

### pr408(首轮 25 条)

| 位置 | 级别 | 主题 | 性质 |
|---|---|---|---|
| LlvmAttributes.kt:43 | needs_confirmation | OHOS 强制 frame-pointer 的平台差异 | 【待确认】 |
| DependencyDownloader.kt:134 | suggestion | ReplacingMode 枚举成死代码 | 【真】 |
| DependencyDownloader.kt:235 | medium | 5xx 重试后原样抛出,回退逻辑缺口 | 【真】 |
| DependencyDownloader.kt:160 | low | 抛异常前未 disconnect(资源清理不一致) | 【真】 |
| DependencyDownloader.kt:144 | suggestion | instanceFollowRedirects 冗余赋值 | 【打磨】 |
| DependencyDownloader.kt:229 | low | KDoc 与 exists() 短路行为不符 | 【真】 |
| DependencyDownloader.kt:235 | low | 无条件删除遗留 .part 文件,断点续传回归 | 【真】 |
| HostLibcxxRuntimeLibraries.kt:24 | medium | 新对象全仓无引用 + KDoc 不准 | 【真】 |
| DependencyProcessor.kt:31 | low | 构造期 fail-fast 行为变更未文档化 | 【打磨】 |
| DependencyProcessor.kt:31 | suggestion | 新解析/回退契约无测试覆盖 | 【真】(验证缺口) |
| OAT.xml:60 | medium | licensefile 留空违反本文件自述规则 | 【流程】 |
| OAT.xml:71 | low | 豁免理由引用不存在的 .kt.out 文件 | 【打磨】 |
| OAT.xml:51 | low | 转义对照表 4 行写反 | 【打磨】 |
| OAT.xml:65 | low | 两条 policyitem 冗余 | 【打磨】 |
| OAT.xml:2 | low | 版权主体不一致(Huawei vs Eazytec) | 【流程】 |
| OAT.xml:17 | low | 联系邮箱缺域名 | 【打磨】 |
| OAT.xml:70 | low | desc 中文全角逗号违反英文约定 | 【流程】 |
| OAT.xml:62 | needs_confirmation | path=".*" 覆盖全仓但白名单仅一家 | 【待确认】 |
| OAT.xml:2 | suggestion | 版权年份区间 "2026-2026" 写法不成立 | 【打磨】 |
| gradle.properties:20 | medium | JDK installations 两文件清单失同步(注释明示要求同步) | 【真】 |
| build-kotlin-ohos.sh:306 | high | altDeploymentRepository 三段式与锁定的 deploy 插件版本不兼容 | 【真】 |
| build-kotlin-ohos.sh:326 | needs_confirmation | -Pkotlin.native.hook 全仓无消费方 | 【待确认】 |
| build-kotlin-ohos.sh:37 | low | --help 依赖硬编码行号 sed 提取,脆弱 | 【真】 |
| cn-mirror.init.gradle.kts:11 | low | startsWith 前缀匹配改写仓库 URL 过宽 | 【真】 |
| KotlinBuildPublishingPlugin.kt:164 | low | 双属性链优先级方向不一致(旧优先 vs 新优先) | 【真】 |

(注:上表为首轮 25 条;pr408 重跑在 9 个营救文件上新增 12 条评论,未并入本表,OCR-only 总数仍按首轮 66 条计。)

### pr427(12 条)

| 位置 | 级别 | 主题 | 性质 |
|---|---|---|---|
| DebugInfoGCCompressTest.kt:114 | medium | 测试未验证其名字声称的 target-gating 逻辑 | 【真】 |
| TestProcessSettings.kt:140 | suggestion | KDoc 引用无法解析的跨模块符号 | 【打磨】 |
| ConfigurationProperties.kt:83 | low | KDoc 声称 OPT 专属但无 gating | 【真】 |
| nativeTest.kt:61 | low | 同上文档失真 | 【打磨】 |
| TestProcessSettings.kt:145 | suggestion | 两处"未启用 split"判定条件不一致 | 【真】 |
| CheckExternalCalls.kt:239 | high | 并行路径 checker 全局符号从未创建,必然失败 | 【真】 |
| Bitcode.kt:494 | high | async N 任务并发使用共享非线程安全编译器状态 | 【真】 |
| Bitcode.kt:583 | high | 未检查强转在并行路径必崩 | 【真】 |
| Bitcode.kt:544 | medium | tmpContext 无 dispose,资源泄漏 | 【真】 |
| Bitcode.kt:491 | needs_confirmation | llvm-split 输出命名契约未经验证 | 【待确认】 |
| TopLevelPhases.kt:314 | low | 日志与实际行为不符 | 【打磨】 |
| build.gradle.kts:19 | needs_confirmation | 编译器后端首引 kotlinx-coroutines,打包路径未确认 | 【待确认】 |

(注:pr427 OCR 18 条,6 条计入 §3 强/弱重合对(#3/#4/#5/#6/#8),其余 12 条如上;ContextUtils.kt:633、Bitcode.kt:426、Config:131(NativeSecondStageCompilationConfig.kt)属重合对的 OCR 侧评论,已移出本表。pr408 表中 KotlinBuildPublishingPlugin.kt:164 为同位置的两条独立评论:一条计入 §3 #6 强重合(deploy-url 语义),表内这条为另一主题(双属性链优先级),非重复计数。)

### pr426(29 条)

| 位置 | 级别 | 主题 | 性质 |
|---|---|---|---|
| ArkTSConfig.h:23 | low | ALWAYS_INLINE 宏缺 inline,跨 TU 无效 | 【真】 |
| ArkTSConfig.cpp:134 | low | extern "C" setter 无入参校验,可绕过 Kotlin 守卫 | 【真】 |
| ArkTSInit.cpp:18 | suggestion | 尖括号包含同目录头文件,依赖偶然的搜索路径 | 【打磨】 |
| Memory.h:70 | medium | HWASan 注释引用的依据不存在(与 TypeInfo.h 矛盾) | 【真】 |
| Console.cpp:20 | suggestion | 宏定义跨文件使用未 include-what-you-use | 【打磨】 |
| Console.cpp:47 | medium | NAPI 跨线程新入口未判空 TSFN | 【真】 |
| ArkTSConfig.cpp:41 | suggestion | 6 个配置原子全部默认 seq_cst,可放宽 | 【打磨】 |
| NapiInterface.kt:125 | high | @Escapes.Nothing 契约与零拷贝逃逸路径矛盾 | 【真】 |
| NapiInterface.kt:136 | medium | 非空返回类型在 napi 失败时 RETURN_OBJ(nullptr) | 【真】 |
| ArkTSMMAPI.cpp:22 | medium | GC sweep/终结路径接入 NAPI 清理,无 teardown 防护 | 【真】 |
| NapiInterface.cpp:317 | low | tryCreate 强引用创建后失败路径泄漏 | 【真】 |
| NapiInterface.cpp:336 | low | 二次 utf16 读取未校验长度一致,尾部未初始化 | 【真】 |
| NapiInterface.kt:21 | low | OHOS stub 不对称:ArkTSConfig 有桩,NapiInterface 无 | 【真】 |
| Memory.cpp:154 | medium | 非法入参返回 nullptr,与兄弟函数抛异常不一致 | 【真】 |
| Memory.cpp:170 | needs_confirmation | 移除 ReadHeapRef HAS_SAFEPOINT 注解依据不足 | 【待确认】 |
| ExtraObjectData.cpp:96 | medium | GC/析构 → NAPI 清理链路无 TSFN 判空与关闭防护 | 【真】 |
| HeapObject.hpp:74 | low | +1 尺寸溢出防护依赖 release 下被裁剪的 RuntimeAssert | 【真】 |
| Memory.cpp:83 | suggestion | (void)success 应改 [[maybe_unused]] | 【打磨】 |
| ArkTSStringRef.cpp:216 | needs_confirmation | call_once 单一 napi_env 假设(多 Ability/worker 场景) | 【待确认】 |
| KString.cpp:733 | medium | 负 fromIndex 未归一化,size_t 巨值回绕 | 【真】 |
| KStringProxyOHOS.cpp:359 | medium | fromIndex==length 空模式串返回值与兄弟路径不一致 | 【真】 |
| ArkTSStringRef.h:160 | medium | getStringAddressOf 无越界检查 | 【真】 |
| KString.cpp:249 | medium | NO_SAFEPOINT 函数内引入可阻塞的 NAPI 调用 | 【真】 |
| ArkTSStringRef.cpp:387 | needs_confirmation | 裸 this 异步投递主线程,生命周期由 GC 控制存疑 | 【待确认】 |
| KStringProxyOHOS.cpp:69 | low | unsafeUtf16ToUtf8Impl/toCharArray 全仓无调用点 | 【真】 |
| ArkTSStringRef.cpp:86 | low | 日志 tag "[StringZeroCopy]" vs "[String0Copy]" 拼写不一致 | 【打磨】 |
| ArkTSStringRef.cpp:75 | suggestion | getpid()/gettid() 隐含主线程初始化约束未文档化 | 【真】 |
| ContextUtils.kt:462 | needs_confirmation | fail-fast 检查被降级为静默跳过 | 【待确认】 |
| ContextUtils.kt:461 | low | 字面量与 K2RStubFunctions.ohosOnlyNames 首项重复 | 【打磨】 |

(注:pr426 OCR 32 条,3 条计入强重合,:286/:257/:166,其余 29 条如上。)

**OCR-only 性质分布**:【真】38(含 1 条验证缺口)/【流程】3 /【打磨】16 /【待确认】9(合计 66,与总表一致)。真发现率 58%,与 CPF 评论的误报率无从直接对比(首轮 CPF-only 17 条中经核对未见明显误报,但存在 2 条主题级弱相邻已单列)。
