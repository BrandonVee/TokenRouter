# 账号维护

本文描述上游账号凭据刷新、健康测试、配额与能力探测、临时不可调度和自动恢复的后台流程。它不定义创建表单字段或请求内调度评分；这些由平台专题与调度架构拥有。

## 章节导航

- [凭据刷新](#凭据刷新)：修改 refresh 候选、并发、重试或状态同步时读取。
- [状态与临时不可调度](#状态与临时不可调度)：修改错误、限流或恢复时间时读取。
- [账号测试与自动恢复](#账号测试与自动恢复)：修改定时测试或恢复条件时读取。
- [额度与能力探测](#额度与能力探测)：修改上游 usage、quota 或 endpoint capability 时读取。
- [运维诊断](#运维诊断)：排查刷新堆积、误封禁或账号抖动时读取。

<a id="account_credential_refresh"></a>
## 凭据刷新

`TokenRefreshService` 分页读取需要维护的账号，按平台 refresher 判断资格，并对每个 provider 应用独立并发/QPS 门槛、单次 attempt 超时、周期总超时和有界退避。OAuth、Setup Token 和 Qoder COSY 的候选规则不同；API Key、Bedrock 和 Service Account 通常由各自请求路径或签名 provider 管理，不应统一假设有 refresh token。

刷新成功后要原子更新凭据/过期时间，清理可恢复错误，并同步账号缓存与调度快照。OpenAI/Antigravity 还可在刷新后确保 privacy 状态。刷新失败按失败阈值记录，不立即把一次瞬时网络错误等同于永久禁用；凭据明确撤销或账号归属失效时才进入需要重新授权的状态。

请求路径 token provider 仍会在使用前检查过期偏移，并用账号级锁避免并发刷新。后台刷新降低热路径延迟，但不是唯一正确性来源；两条路径必须使用相同的凭据版本/CAS 保护，避免旧请求覆盖新 token。

## 状态与临时不可调度

账号长期状态、`schedulable`、全账号限流、模型限流和临时不可调度规则是不同层次：

- 凭据/配置错误可以记录 recoverable error 并要求人工重新授权。
- 429、明确 reset time 或短期网络/供应商故障使用恢复时间，在到期前过滤账号或模型。
- 管理员策略和代理过期可能临时移出调度，但不删除账号。
- 账号到期可由维护任务自动暂停；重新启用前仍需验证凭据和关联资源。

状态写入要携带凭据快照或版本条件。较早请求不能在新凭据生效后再次设置旧错误；恢复同样不能清除另一个请求刚确认的永久错误。

<a id="temporary_rule_scope"></a>
### 临时不可调度规则作用范围

规则的可选 `scope` 接受 `account` 或 `model`：前者暂停整个账号，后者只暂停当前请求模型；模型未知时仍以整账号为安全回退。未设置时保持 fork 的历史行为：模型已知且不是 401 时按模型暂停，否则按账号暂停。创建/编辑界面会显式保存选择，新规则默认按模型；保存历史 401 规则时回填整账号，避免静默改变凭据错误的隔离范围。

Cloudflare 1010 是边缘拦截，不消耗账号 403 连续错误次数，也不因该响应永久停用账号。模型限流快照写入使用复制后的顶层和嵌套 map；Antigravity 发布限流缓存时不能原地修改与其它候选共享的快照。

## 账号测试与自动恢复

管理端即时测试和 `scheduled-test-plans` 使用平台测试服务调用真实凭据/模型，并保存测试结果。计划使用分钟级 cron 表达式；每个计划可配置自动恢复。成功测试可以清除符合条件的 error、rate limit、temporary unschedulable 和模型限流，但不能绕过管理员禁用、账号过期或类型不匹配。

测试本身应使用受控超时、代理/TLS 路由和脱敏日志。Gemini 平台账号始终使用原生 `contents`、`generationConfig` 和 `:generateContent` 测试，生图模型也不改走 `/v1/images/generations`；Code Assist 因上游约束继续使用内部流式动作。OpenAI API Key 兼容账号选择符合 Gemini 生图结构规则或 `doubao-seedream-*` 模型族的模型时，管理端提交生图提示词且不提交 Compact 模式，后端通过 `/v1/images/generations` 测试，不走 Responses、Compact 或 Chat 文本探测；公开 Seedream 名映射为 `ep-*` 接入点时，测试类型仍以映射前的公开模型名判定。生图测试从返回图片字节解析实际像素分辨率，并随预览结果展示。一个模型测试成功只证明该路径当时可用，不证明所有 endpoint capability 或媒体资格。失败结果需区分认证、模型、配额、代理、TLS 和上游容量，以免自动恢复形成启停抖动。

## 额度与能力探测

平台可维护独立的上游额度快照：OpenAI/Codex 窗口、Gemini tier/model quota、Antigravity credits、Grok 计费/媒体资格、Qoder Credits 等。快照用于调度、容量展示和诊断，不是 TokenRouter 用户余额或订阅账本。

Anthropic Setup Token 的单账号查询与批量被动查询共用响应头采样的 5h、7d 和 7d_oi 窗口，不主动调用需要 profile scope 的 usage API。只有周窗口的成功响应和官方窗口限流的 429 也要采样，不能依赖 5h status 存在；5h 重置只清除本窗口利用率，保留独立周窗口的快照与采样时间，缺失数据仍不伪造为实时值。

OpenAI API Key 的 Responses 能力探测只在 `openai_responses_mode=auto`（含未设置或无效值的默认归一化）时执行，强制 Responses/Chat 模式跳过无效上游调用。GPT Image 账号连接测试不发送上游拒绝的 `response_format`；其它兼容图片模型保留原有格式请求。

OpenAI 重置次数查询把带到期时间的完整结果保存为账号展示快照；上游只返回正数次数却缺少到期明细时，实时结果仍返回给调用方，但旧快照必须保留。直接调用重置 API 成功消费次数后，服务先在脱离客户端取消信号的有界上下文中恢复账号 error、限流和临时不可调度状态，再回读额度快照与最新账号投影；恢复不修改人工 `schedulable` 开关。后续步骤部分失败时响应使用 `cache_refreshed`、`account_state_recovered` 和 `warning_code` 明确区分，调用方不得把已消费的次数当作可重试失败。

<a id="openai_quota_auto_reset"></a>
### OpenAI 自动使用重置机会

自动用卡仅支持 OpenAI OAuth 母账号，升级后默认关闭。管理员在账号编辑页显式开启，并分别设置 5h/7d 用卡阈值（0.1–100%）。它与原有自动暂停阈值独立：已确认有卡时可继续到用卡阈值，未知或失败状态仍按规则退出调度，等待自然窗口重置后解除动态阻塞；人工停用和调度开关继续生效。

后台四个 worker 接收去重通知，分钟扫描补偿重启与漏事件；真实响应、429 和普通额度刷新只在状态落库后通知。受限只读管理员 Key 的查询不会生成模型请求、恢复状态或启动自动消费；展示层的暂停判断仍保持无副作用。

每个窗口周期固定选择最早到期的官方卡 ID，兑换 ID 由账号、卡指纹和周期确定，跨实例使用 PostgreSQL 幂等记录串行化。同一周期的重试不得切换另一张卡；原卡丢失、明细或到期时间不完整时拒绝消费。状态只保存不可逆指纹，不把卡 ID、兑换 ID 写入账号 extra、幂等响应或审计。编辑表单不能改写受管状态，仓储在行锁内采用最新数据库值，防止覆盖后台的重试身份。

消费前重新读取开关；成功后与手动 API 共用状态恢复、窗口/余额/重置次数快照刷新。失败单独保存错误状态，不将已消费结果伪装成可重试的整次失败。账号行继续保留只读次数查询，新增积分及邀请入口，自动消费由独立配置控制。

<a id="claude_native_quota_reset"></a>
### Claude 原生额度重置

Anthropic OAuth 账号持有 `user:profile` scope 时，可查询原生 `cedar_ember` 重置机会。Setup Token、API Key 和其它平台不适用。查询只读且不消费次数；对外结果不含 grant、组织或上游请求 ID，界面区分持有次数与当前可消费资格。

`GET /api/v1/admin/accounts/:id/claude/reset-credits` 查询；POST 同路径的 `/redeem` 以 `Idempotency-Key` 对应一次操作确认，不接收客户端选择的 grant。服务在提交前再次查询，只允许上游指定的下一机会、当前可用、未暂停/阻断、冷却结束且满足触限条件的机会。受限管理密钥分别使用 `claude.quota.read` 与 `claude.quota.reset`，写权限不额外返回完整次数。

消费必须同时具备持久幂等仓储和 Redis 账号/组织租约；任一不可用都拒绝。组织级阻断记录位于幂等表，重复本地账号共用，账号编辑不能抹掉记录。发出请求前先持久化未知结果标记；同一确认崩溃后只重放，未知结果阻断该组织再次领取 24 小时，明确的上游不可用仅阻断 15 分钟。已明确成功的请求不会因后续刷新失败而盲目重发。

请求使用 fork 的共享上游客户端、实际账号与代理、TLS 指纹和出站安全校验，携带 OAuth 的请求禁止跟随重定向。只读查询有完整 25 秒期限，消费使用独立于客户端断开的 60 秒幂等执行期限，收尾保存使用短时独立上下文。额度组件保持稳定实例，重试沿用同一次确认的 key。

OpenAI API Key 可探测 Responses、Chat、Embeddings 等 endpoint capability。Responses 探测只有在响应足以下结论时才写入能力标记：2xx 响应若仍因 `max_output_tokens` 未完成，或响应状态为 `failed`，应保持 unknown，不能持久化为“不支持 Responses”；完成但没有 `function_call` 的响应仍判定为不支持。Ollama Cloud 等兼容 API Key 上游可以保存其管理会话和用量快照，但只有明确匹配的账号才进入探测，不能把探测协议推广到所有 `apikey`。Grok 计费与媒体资格、各平台额度探测也继续使用各自独立协议。

多协议供应商的周期余额/套餐检测、Cline 钱包隔离和 OpenCode Go 请求驱动刷新见[多协议上游](../interfaces/multi_protocol_upstream.md#provider_usage)。这些 worker 与现有 Ollama、Qoder 探测独立启停，不恢复已下线的声明倍率探测。

通用的上游声明倍率探测已移除，不再有定时任务、手动操作、快照或公开账单自省接口。账号创建、编辑、批量更新、复制、CRS 同步和仓储写入都会丢弃历史 `upstream_billing_probe` 与 `upstream_billing_probe_enabled` 键；这项清理不得影响 Ollama Cloud 会话/用量、endpoint capability 或其它额度状态。

实时探测失败时保留最近成功快照并同时暴露当前错误，不把旧数据标为实时。任何配额耗尽或 capability 变化都要触发相关调度投影失效。

## 运维诊断

- 观察每 provider 的候选数、刷新成功/失败、节流、超时和最长积压，而不只看总成功率。
- 关联账号测试、刷新、quota probe、代理健康和调度过滤原因，区分凭据故障与出站网络故障。
- 检查账号数据库状态、当前进程投影和跨实例失效是否一致；手工改数据库后等待周期重建不等于即时生效。
- 自动恢复或批量导入后抽查实际协议，避免仅凭 token endpoint 成功误判推理可用。

相关文档：[上游账号能力矩阵](../interfaces/upstream_account_matrix.md)、[账号调度与缓存一致性](../architecture/account_scheduling_and_cache.md)、[上游传输安全](upstream_transport_security.md)。
