# Gemini 上游

本文描述 Gemini OAuth、API Key 和 Service Account 账号，Gemini v1beta 原生入口，以及 Anthropic/OpenAI 兼容转换的当前边界。它不固化上游动态模型清单，也不把 Antigravity 混合账号等同于 Gemini 原生账号。

## 章节导航

- [账号与认证](#账号与认证)：修改 OAuth 变体、API Key 或 Vertex 凭据时读取。
- [协议分派](#协议分派)：修改 Gemini 原生、Messages、Responses 或 Chat 转换时读取。
- [模型与会话](#模型与会话)：修改模型解析、thinking、signature 或缓存连续性时读取。
- [配额与调度](#配额与调度)：修改 tier、模型限流、混合调度或粘性时读取。
- [错误与诊断](#错误与诊断)：修改 Google 错误、刷新或 failover 时读取。

## 账号与认证

Gemini 正式支持：

| 类型 | 当前契约 |
| --- | --- |
| `oauth` | 支持 `code_assist`、`google_one` 和 `ai_studio` 变体；前两者使用由 `GEMINI_CLI_OAUTH_CLIENT_ID`/`GEMINI_CLI_OAUTH_CLIENT_SECRET` 成对注入的 Gemini CLI 客户端，AI Studio 需要配置的 OAuth client |
| `apikey` | 使用 Base URL 和 API Key 直连；`credentials.provider_type=third_party` 表示 Gemini 兼容第三方提供商，缺失或 `official` 表示 Google AI Studio 官方接入 |
| `service_account` | 使用 Google Service Account 换取 Vertex token，并解析 project/location 上下文 |

Gemini 的 `service_account` 也可设置 `credentials.auth_mode=apikey` 使用 Vertex API Key。没有 `project_id` 时走全局 Express 端点，有 project 时走项目/区域端点；使用 `x-goog-api-key`，不换取 SA token。它不参与显式缓存与 Vertex 批量图片。Anthropic Vertex 仍只接受 Service Account JSON。账号测试保持 fork 的原生 `generateContent` JSON 和配置的 User-Agent。SA 换取令牌明确返回账号不存在时停用账号，记录凭据阶段失败后换号；凭据错误不能当成模型端点能力问题。

Code Assist/Google One 需要有效 project；AI Studio 的 project 可选并使用选择的 tier。第三方 API Key 保持 `type=apikey` 和 Gemini 兼容请求形状，但必须配置非 Google 官方域名的 Base URL；它没有 Google 官方账号等级，因此不写 `tier_id`，也不参与本地模拟 RPD/RPM 预检或用量窗口。第三方上游实际返回 `429` 时始终使用通用冷却，不解析 Google 日配额的重置语义。OAuth refresh 会重试并兼容历史 client 元数据；token provider 使用过期前偏移和并发锁，避免同账号重复刷新。其它导入类型没有 Gemini 正式转发契约，见[上游账号能力矩阵](upstream_account_matrix.md)。

仓库和发布产物不内置 Gemini CLI OAuth Client ID 或 Client Secret。任一环境变量缺失时，Code Assist/Google One 授权按配置错误拒绝，不能回退到硬编码凭证；已经公开过的 Client Secret 必须在 Google Cloud 控制台撤销并轮换。

<a id="gemini_protocol_dispatch"></a>
## 协议分派

Gemini SDK/CLI 使用 `/v1beta/models`、`/v1beta/models/{model}` 和 `{model}:{action}` 形状，保持 Google 请求、流和错误语义。Anthropic Messages、Count Tokens、OpenAI Responses 与 Chat Completions 入口则先归一化，再由 Gemini 兼容服务转换为上游请求，响应恢复为原客户端协议。

Gemini 分组支持 Messages、Responses、Chat 和 Gemini GenerateContent，新建时默认只启用 GenerateContent；四项都可关闭，迁移前已有分组启用四项。GenerateContent、StreamGenerateContent 和 CountTokens 的 POST 动作受 Gemini 协议开关控制，模型列表 GET 不受影响。

Responses 转换是正式分支：非流和 SSE 共用 Gemini 上游执行、重试与响应适配，保留模型、reasoning、工具调用、usage、结束原因和首次 Token 指标。上游失败只在客户端收到首个字节前允许切换账号；流开始后按 Responses SSE 语义结束，不能回写普通 JSON 或换账号。

API Key、OAuth 和 Vertex Service Account 在认证头、base URL、project/location 和错误结构上不同；协议转换层必须保留这些传输差异。每次 failover attempt 都重新选择账号并重建 payload，流式输出开始后不再切换。

Antigravity 专用 `/antigravity/v1beta/*` 强制选择 Antigravity 账号，其契约由[Antigravity 上游](antigravity_upstream.md)拥有，不属于 Gemini 原生账号支持范围。

## 模型与会话

可请求模型由分组、渠道和账号能力共同解析。客户端模型依次经过 Key 重定向、渠道映射和账号映射；Vertex 或 AI Studio 的最终模型标识与计费模型可以不同。模型列表不能仅回显默认常量，也不能展示没有可调度账号支持的目标。

兼容层维护 thinking/推理字段、tool/schema、图片输入、usage、finish reason 和 Gemini thought signature。工具 schema 会递归移除 Gemini 不支持的字段；INTEGER 的整数 `exclusiveMinimum` 转换为加一后的包含式 `minimum`，且不覆盖更严格的既有下界，无法等价转换的独占下界只清理不伪造。需要跨轮次的 signature、session 和 cache 连续性时，粘性会话优先复用账号；切换账号必须重新评估可继续性，不能把另一个账号的内部状态当作通用上下文。

Messages 和 OpenAI 兼容入口按最终映射模型选择工具媒体形状：Gemini 3 及以上将图片放进对应调用的 `functionResponse.parts`，保留并行调用归属；旧模型继续在相邻用户 part 中提供图片，纯工具图片附带明确的文字占位。Antigravity 复用该转换规则，但其内置工具混用限制仍由平台专题单独控制。

原生与 Claude 兼容生图响应按上游实际返回的内联或文件 URI 图片 part 数量计费，自定义模型别名也适用。流式响应按单个 payload 中观测到的最大图片数记录，避免累积式 SSE 重复计费；未观测到内联图片时才回退到请求模型名或映射后模型名的生图启发式。

### 当前模型与图片响应

原生与连接测试默认模型目录已更新至 Gemini 3.x 与 Gemini 4 Argon，默认测试为 `gemini-3.5-flash`；Google One 仍使用其独立保守目录。旧 2.x 模型不再作为默认推荐，但管理员显式模型映射仍可配置。AI Studio `AQ.` 与旧 `AIza` Key 均可输入，并在日志脱敏层隐藏。

图片输出计数同时识别 `inlineData`/`inline_data` 和 `fileData`/`file_data`，要求图片 MIME 和非空数据或 URI，同一 part 只算一张。累积式 SSE 继续使用单个 payload 的最大图片数，避免重复计费。被拦截 prompt 只返回缓存命中时，净输入 token 截断为非负数，不产生负费用。

`allow_image_generation` 同时限制 Gemini、Antigravity 与 Composite 的 Gemini 图片输出：请求模型、渠道映射、账号映射或 `responseModalities` 的 IMAGE（含数字枚举和蛇形字段）任一命中即可识别。入口先拒绝，转发层在账号映射后复核；`countTokens` 不受生图开关限制。迁移 271 为历史 Gemini/Antigravity 分组开启开关，保持此前可生图行为；管理员之后可显式关闭。

模型列表优先使用 fork 的自定义展示配置；账号全部声明映射时按声明目录与允许混合的 Antigravity 模型合并。否则选择可服务模型接口的账号：AI Studio 优先，Vertex SA 读取对应区域 Google 发布方模型并分页转换为 AI Studio 形状，Vertex API Key 因上游列表不接受 Key 而返回内置目录。单页响应受 `gateway.models_list_read_max_bytes` 限制，分页有上限；未完成分页不能当作完整目录覆盖配置。

<a id="explicit_context_cache"></a>
## 显式上下文缓存

`/v1beta/cachedContents` 提供 POST/GET，以及 `/:cacheID` 的 GET/PATCH/DELETE。只有普通 Gemini 分组支持；创建和续期遵守 Gemini 协议准入、内容审核及资金检查。官方 AI Studio API Key、Vertex SA JSON 可用；自定义 Gemini 上游必须在账号声明 `explicit_cache_upstream=true`，可额外配置 `explicit_cache_upstream_max_ttl_seconds`。OAuth、Antigravity 和 Vertex API Key 不支持这些管理操作。

客户端只见随机缓存 ID，本地绑定 API Key、付款/请求归属、分组、模型和持有账号，查询列表不会读取共享上游的其他租户缓存。生成请求可引用本 Key、当前分组仍有效且模型一致的缓存；网关替换上游资源名并只允许持有账号，禁用会话摘要复用和换号。不存在、过期或无权访问统一返回 Google 形状的 403，不处罚账号。绑定上游网关的 429/5xx 可按池配置同号重试，耗尽后保留其状态，不能切到无缓存的账号。

创建一次收取缓存内容的输入费用和完整 TTL 的存储费，延长只收新增长度的存储费；缩短或删除不退已收费用。单次最长 TTL 默认 86400 秒，上游声明更小上限时取小值。存储单价来自确定识别的价格目录 `cache_storage_cost_per_token_per_hour`，否则按内置 Gemini 价卡；无价拒绝创建。存储成本单独加入总费用和账号成本，纯续期不再收一次模型请求费。创建和每次延长使用独立 `gcache:` 计费 ID，不能被同一客户端 request ID 合并。

已知费用用[固定费用准入](../domains/routing_and_billing.md#fixed_charge_admission)检查。创建后才能取得 token 数，不足时清理上游缓存，仅结算已发生的输入；续期在上游调用前检查完整费用。修改 TTL 先在数据库登记带令牌的租约，提交前保持旧到期时间；上游成功而本地未提交时，后台恢复按旧值回滚。恢复循环随进程启动/关闭，过期租约可接管，账户暂不可用时保留意图，只有可确定上游缓存必已过期才清理失联记录。

## 配额与调度

Gemini tier、上游配额和按模型 reset 信息作为账号资格与容量信号。普通账号的 429 响应可以更新账号或模型的 `rate_limited_until`，后续调度在恢复时间前过滤该候选；公共池账号由请求级同账号重试或切号消化 429，不写默认本地账号限流，但管理员显式配置的自定义错误策略仍然优先。实时配额查询失败不应伪造剩余额度。

Vertex 的共享容量不足不写账号每日冷却，由请求同号退避与切号处理；项目配额等账号自身限速没有可解析 reset 时使用短冷却。`google.rpc.RetryInfo.retryDelay` 可提供精确恢复时间，异常长值上限为 15 分钟。

显式开启 mixed scheduling 的 Antigravity 账号可以加入 Gemini 分组候选，但仍需满足目标分组、模型、endpoint、额度、并发和凭据约束。普通 Gemini 请求保持 Gemini 协议和计费归属；专用 Antigravity 路由不混入 Gemini 账号。

## 错误与诊断

OAuth refresh、Service Account token、project/tier 发现和上游请求错误分别记录。401/403 需要区分凭据、project/region、API 未启用或策略拒绝；429 解析 reset 并更新限流；网络/5xx 只在响应未开始时允许换账号。

Gemini 原生入口返回 Google 形状，Anthropic/OpenAI 入口返回对应客户端形状。最终错误可应用[网关错误响应策略](gateway_error_policy.md)，但 project、service account JSON、token、API key 和内部上游响应不能无条件透传。排障应核对 OAuth variant、project/location/tier、最终模型、thought/session 状态、quota reset 和 attempt 链。

相关文档：[网关请求生命周期](../architecture/gateway_request_lifecycle.md)、[账号调度与缓存一致性](../architecture/account_scheduling_and_cache.md)、[账号维护](../operations/account_maintenance.md)。
