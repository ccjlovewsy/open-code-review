# 同类 AI 评审项目调研(为 OCR 增强寻找可借鉴能力)

> 调研方式:GitHub API 实测(2026-10),锚定 MRXVAM/AIReviewer(GitCode PR 的 AI 检视:Chrome 扩展 + 本地 Express + OpenCode SDK + skill)与 alibaba/open-code-review 的能力面。
> 子代理产出,人工核校。

## 平台型 bot / 流水线

- [pr-agent (Qodo)](https://github.com/The-PR-Agent/pr-agent) — 13.5k★,MIT,活跃。/review /improve /describe 单调用工具 + PR 压缩 + JSON 分层提示(configurable.toml);/improve 二轮自我反思过滤;repo 级 `best_practices.md` 组织知识库。
- [kodus-ai](https://github.com/kodustech/kodus-ai) — 1.5k★,自托管 CodeRabbit 平替,BYOLLM;org 级评审标准 + ast-grep 结构规则;姊妹库 [kodus-graph](https://github.com/kodustech/kodus-graph)(MIT)用 14 语言 AST 代码图算 blast radius/风险评分/测试缺口。
- [claude-code-security-review](https://github.com/anthropics/claude-code-security-review) — 6.5k★,MIT。GitHub Action 多 pass:先评审,再按置信度二次过滤误报后才发行内评论。
- [cr-gpt (anc95/ChatGPT-CodeReview)](https://github.com/anc95/ChatGPT-CodeReview) — 4.5k★,ISC。Probot 托管 App,diff 分块单 prompt——工程化弱,属"套壳"参考。
- [shippie](https://github.com/mattzcarey/shippie)(原 code-review-gpt)— 2.5k★,MIT,活跃。embedding 召回相关文件构建上下文,agent 化,支持 MCP。
- [sweep](https://github.com/sweepai/sweep) — 7.7k★,停滞(2025-09)。issue→PR,tree-sitter 分块;借鉴价值有限。

## 规则 / 知识库 / 定位(非 LLM 确定性机制)

- [reviewdog](https://github.com/reviewdog/reviewdog) — 9.7k★,MIT,活跃。统一解析任意 linter 输出,按 diff filter 模式(added/diff_context/file)做**行级锚定、只评新增行**。
- [danger](https://github.com/danger/danger) — 5.7k★,MIT,活跃。Dangerfile DSL 把团队 checklist 沉淀为可版本化代码,按 PR 元数据(路径/标签/作者)触发。

## 基准 / 规则组织

- [secpriv-skill (Meta)](https://github.com/facebookresearch/secpriv-skill) — 9★,MIT。评审 skill 按 **CWE/GDPR 条款**组织 detector-validator,附 128 例基准含"真阴性"子集专测误报。
- [code-review-graph](https://github.com/tirth8205/code-review-graph) — 32k★,MIT,活跃。本地 tree-sitter 代码图谱,给评审供最小上下文(非 PR bot)。
- CodeRabbit:核心闭源 SaaS,开源仅 skills/插件;产品化亮点:`.coderabbit.yaml`、逐评论学习、path 过滤。
- aacr-bench(OCR 自家,246★,略)。

排除:gptcomet 未查到且实为 commit 工具;GitLab 仿品多 <20★ 停更。

## Top 3 最值得 OCR 借鉴

1. **pr-agent**:与 OCR 管线同构且最成熟——best_practices 知识库对应 rule.json、自我反思过滤对应反思阶段、JSON 提示分层与 PR 压缩直接可抄;多平台 webhook 架构最完整。
2. **kodus-ai + kodus-graph**:同为"自托管 bot + 组织规则沉淀"定位;AST 代码图给"文件分桶→风险排序"提供确定性证据源,能把行级评论幻觉压到最低。
3. **reviewdog(+danger)**:行级锚定与"只评新增行"的过滤模式、规则即代码的 checklist 沉淀,是 OCR 定位/幻觉抑制层最值得移植的零幻觉确定性机制。
