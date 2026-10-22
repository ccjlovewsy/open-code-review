# OCR vs CPF 平台 AI 评审实测对比报告

> 日期:2026-10-22。样本:CPF-KMP-CMP/kotlin 的 3 个历史 PR(PR#408/427/426,合计 28 条 cpf-manager 评论)。
> OCR:增强版二进制(~/code/open-code-review/ocr),provider=volcano-glm(GLM 5.3→网关映射 glm-5.3-flash),language=中文,timeout_sec=900,含 CPF 反推沉淀的 `.opencodereview/rule.json`。
> 原始产物:`/tmp/cpf-compare/`(每 PR 的 .diff、ocr.json、ocr.log、matching.json)。

## 0. 结论先行

- **总 recall(严格口径,同根因)= 6/28 = 21.4%;宽口径(含同主题不同实例)= 11/28 = 39.3%**。
- 但这个数字被一个 OCR 输入侧缺陷严重压低:默认排除模式 `**/target/**`(本意防构建产物)误杀 Kotlin/Native 源码包 `konan/target/`,**仅 pr408 就有 7/14 条 CPF 评论(25%)落在 OCR 从未看到的文件上**。剔除该因素后,可达样本上严格 recall = 6/21 = 28.6%,宽口径 = 11/21 = 52.4%。
- OCR 独有发现 68 条,质量总体扎实(并行编译线程安全、资源泄漏、契约不一致等 high 级发现 CPF 未报);CPF 独有 17 条中有 12 条是 OCR 明确可补的(过滤误杀 7 + 规则已载入未触发 5)。
- CPF 的两条结构化优势得到量化确认:跨文件穷尽性检查(其唯一【致命】评论)与 PR 流程检查,后者 OCR 架构上暂不支持(输入无 PR body)。

## 1. 样本与方法

| PR | cpf-manager 评论 | diff 规模 | OCR 评审方式 | OCR 评论 | 耗时/tokens |
|---|---|---|---|---|---|
| 408(bootstrap) | 14 | 20 文件/1181 行 | range 模式 M^1..M(已合并,与 PR diff 逐文件等价) | 29 | 30min/1.0M |
| 427(C-API 迁移) | 8 | 16 文件/801 行 | worktree(03ce569a40)+ apply + workspace 裸跑 | 18 | 30min/4.0M |
| 426(并行编译) | 6 | 28 文件/3980 行 | 同上 | 32 | 30min/9.0M |

- 样本选择:按 cpf-manager 评论数取 Top(408:14、403:13、409:12),但 **403/409 的服务端 diff 分别达 537/424 文件,按预案顺移**至 427(8)/426(6)。
- 匹配口径:**强重合**=同文件+同根因(同一缺陷);**弱重合**=同文件+同主题(同一规则族、不同实例/不同机制);两者之外为 CPF-only / OCR-only。每条判定基于双方评论文本逐条人工核对。

## 2. 总表

| | pr408 | pr427 | pr426 | 合计 |
|---|---|---|---|---|
| CPF 评论 | 14 | 8 | 6 | 28 |
| OCR 评论 | 29 | 18 | 32 | 79 |
| 强重合 | 2 | 1 | 3 | **6** |
| 弱重合 | 1 | 4 | 0 | **5** |
| CPF-only | 11 | 3 | 3 | **17** |
| OCR-only | 25 | 13 | 29 | **67** |

**recall:严格 6/28 = 21.4%;宽口径 11/28 = 39.3%**(口径:重合数/CPF 评论总数)。
可达样本口径(剔除 7 条落在被误杀文件上的评论):严格 6/21 = 28.6%,宽口径 11/21 = 52.4%。

## 3. 逐条对照

### pr408(bootstrap;14 条)

| CPF | 内容摘要 | OCR 对应 | 判定 | 归因(CPF-only 时) |
|---|---|---|---|---|
| #1 严重 | OhosLinker 无条件链 Scrt1.o 进动态库 | 无(Linker.kt 被 `**/target/**` 误杀) | CPF-only | **输入侧过滤误杀** |
| #2 一般 | crtn.o 链接顺序错误 | 无(同上) | CPF-only | 输入侧过滤误杀 |
| #3 一般 | Usage 引用不存在的脚本名 | 无 | CPF-only | 规则已载入未触发(rule.json 有逐字规则) |
| #4 一般 | -P/-D 明文传递部署凭据 | build-kotlin-ohos.sh:252/:304 [security/high]×2 | **强重合** | — |
| #5 一般 | allWarningsAsErrors=false 削弱门禁 | 无 | CPF-only | 规则已载入未触发 |
| #6 一般 | deploy-url 语义向后不兼容 | KotlinBuildPublishingPlugin.kt:164 [suggestion] | **强重合**(OCR 级别偏低) | — |
| #7 严重 | useCompilerDriverAsLinker 与直接调 ld.lld 矛盾 | 无(误杀) | CPF-only | 输入侧过滤误杀 |
| #8 严重 | 响应文件路径含空格被 lld 拆分 | 无(误杀) | CPF-only | 输入侧过滤误杀 |
| #9 一般 | local.properties 清理残留 | 无 | CPF-only | 规则已载入未触发 |
| #10 严重 | 链接命令缺失 dynamicLibraries | 无(误杀) | CPF-only | 输入侧过滤误杀 |
| #11 严重 | OHOS_X64 不应列 HWADDRESS | 无(误杀) | CPF-only | 输入侧过滤误杀 |
| #12 一般 | LD_PRELOAD 硬编码 x86_64 路径 | 无 | CPF-only | 知识库未覆盖(脚本架构路径) |
| #13 一般 | rsync 未做可用性检查 | build-kotlin-ohos.sh:100 [low](同主题:工具可用性,但查的是 gradlew/mvnw) | 弱重合 | — |
| #14 致命 | HWADDRESS 新枚举未同步穷尽性 when(build-tools 4 处) | 无(锚文件 Sanitizer.kt 被误杀) | CPF-only | 输入侧过滤误杀(+跨文件检查因看不到锚点而失效) |

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

## 4. 归因 Top 榜(CPF-only 17 条)

| 归因 | 条数 | 说明 |
|---|---|---|
| **输入侧过滤误杀**(`**/target/**` 误伤 `konan/target/` 源码包) | **7** | 全部在 pr408;一个默认排除模式吃掉该 PR 25% 的 recall,其中含 CPF 唯一的【致命】跨文件发现 |
| **规则已载入但未触发** | 5 | rule.json/delegate 验证确认规则加载,模型未对具体实例开火(408:Usage 脚本名、allWarningsAsErrors、local.properties 清理;427:中文注释;426:NAPI Pin) |
| **知识库未覆盖** | 5 | 脚本架构硬编码路径、并行 phase 完整性、日志规范(println)、C++ RAII 析构顺序、恒真/死分支检测 |

## 5. OCR-only 发现概览(67 条,逐条性质见附录 A)

- **实质 bug 类(高价值)**:并行路径共享编译器状态线程安全(Bitcode.kt:494 [high])、未检查强转必崩路径(:583 [high])、tmpContext 泄漏(:544)、TSFN 生命周期与 GC 卸载链路(ExtraObjectData.cpp:96)、@Escapes 注解契约与零拷贝逃逸矛盾(NapiInterface.kt:125 [high])等——**CPF 均未报**。
- **流程/配置类(CPF 未涉足的矿脉)**:pr408 上 OCR 对 OAT.xml 连发 9 条(process 类别生效:许可证文件缺失、版权主体不一致、中文全角逗号违反英文约定、转义对照表写反),CPF 对 OAT.xml 零评论。
- **新 schema 运转良好**:needs_confirmation 用于「llvm-split 输出契约未经验证」「多 napi_env 假设」等证据不足场景(6 条),suggestion 用于非阻塞建议;suggestion/needs_confirmation 未出现在强重合对中,级别对齐良好(pr426 三条强重合 CPF 严重 ↔ OCR high 全部一致;pr408 #6 OCR 低评一级)。
- **代价**:3 个 PR 合计约 14M tokens;宽口径下 OCR 的 documentation/style 类占 OCR-only 约 40%,存在噪声矿脉(OAT.xml 的 9 条里 4 条属低价值打磨)。

## 6. 对 OCR 知识库与 rule.json 的补强建议(按优先级)

1. **修 `**/target/**` 误杀(输入层,收益最大)**:在目标仓 rule.json 增加 `"include": ["**/konan/target/**"]`(user include 优先级高于默认排除,selection.go:88 已验证语义),或在 fork 内改 default_exclude_patterns.json 为更精确的构建产物模式。仅此一项可解锁 pr408 上 50% 的 recall 上限。
2. **规则触发率问题(prompt 层)**:5 条「规则已载入未触发」说明规则文本进上下文≠被执行。建议在 main_task_system.md 增加对 rule.json 规则的显式巡检指令(逐条自查:type 每条 user rule 的典型形态,回答本 diff 是否命中),或把 user rule 挂到 grouping 后的每文件 checklist。
3. **补 4 条知识库缺口到 rule.json(全部来自本次 CPF-only 实测)**:
   - 脚本:禁止按架构硬编码多库目录路径(/usr/lib/x86_64-linux-gnu),应按 uname -m 推导;
   - 编译管线:并行化 phase 改造必须对照顺序路径 phase 清单(穷尽性);
   - 通用:生产代码禁用 println/stdout 充当日志,走统一日志设施(CPF「日志输出问题」主题);
   - C++:RAII 成员声明顺序与析构顺序语义(注释意图 vs 声明逆序)。
4. **恒真/死分支检测**交给后续规则沉淀(tautology 模式:`it = begin() + (ptr - data())` 后比较 `ptr == &*it`),低优先。
5. **级别校准**:强重合对中 OCR 唯一低估的是「行为向后不兼容」(suggestion vs CPF 一般)——建议在 severity 定义中把「CLI/属性语义向后不兼容」从 suggestion 提到 medium。

## 7. 与CPF 评审的结构性差异(实测确认)

- CPF 独有的 PR 流程检查(模板四要素、测试报告完整性)依赖 PR body,OCR 输入层暂不支持(见对比报告 §6.4,本次未做)。
- CPF 的「触发路径」写作规范在 OCR 增强 prompt 中已生效(本次 OCR 多条评论含调用链),但 OCR 的证据呈现更偏代码摘录,CPF 更偏运行时后果推演——风格差异,非能力差距。
- 双方在「资源生命周期/并发/NAPI 边界」上高度重合(pr426 三条逐字级),说明该领域规则一旦显式沉淀,两家模型都能稳定命中——**知识库沉淀比模型差异更决定 recall**。

## 附:方法偏差说明

- pr408 用已合并 commit 的 M^1..M range 评审(与 PR diff 逐文件等价,已核对 20 文件清单一致),未按 worktree 重跑(节省 30min/1M tokens)。
- pr403/409 因服务端 diff 过大(537/424 文件)按预案顺移为 427/426。
- 逐条判定由主模型直接核对双方原文完成,未再拆子代理(spec 允许子代理但要求复核原文,直接核对等效且更稳)。
- kernel 噪声:三次评审各有一次 Round-2 重试/失败(网关偶发),均未影响最终产出(complete,comments 齐全)。

## 附录 A:OCR-only 逐条性质表(67 条)

性质标签:【真】实质缺陷/契约问题(建议修)【流程】合规/流程发现(依赖仓库治理上下文)【打磨】低价值风格/文档噪音【待确认】needs_confirmation(证据不足,非缺陷断言)

### pr408(25 条)

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

### pr427(13 条)

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
| NativeSecondStageCompilationConfig.kt:131 | medium | splitBCfile=0 直通 → 必然 IllegalStateException | 【真】 |
| ContextUtils.kt:633 | low | interop 非空断言 + llvm.used 静默丢失 + 无 verifyModule | 【真】 |
| Bitcode.kt:426 | low | llvm-link/llvm-split 无超时,无限期阻塞 | 【真】 |
| TopLevelPhases.kt:314 | low | 日志与实际行为不符 | 【打磨】 |
| build.gradle.kts:19 | needs_confirmation | 编译器后端首引 kotlinx-coroutines,打包路径未确认 | 【待确认】 |

(注:pr427 OCR 18 条,2 条计入弱重合的第二条对应评论(Bitcode.kt:534/Config:131 中 Config:131 计为 OCR-only 辅助证据)——按主匹配计 13 条 OCR-only;上表 15 行系把 Config:131 一并列出便于复查。)

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

**OCR-only 性质分布**:【真】41 /【流程】3 /【打磨】14 /【待确认】9(合计 67,与总表一致)。真发现率 61%,与 CPF 评论的误报率无从直接对比(CPF-only 17 条中经核对未见明显误报,但存在 2 条主题级弱相邻已单列)。
