# 多协议与 System One 上游

本文描述 Kimi、智谱、DeepSeek、MiniMax、OpenCode、Command Code、Cline 的多协议 API Key 适配，以及 TypeSafe 的独立 System One 协议。用户余额、团队与订阅结算仍遵循路由与结算领域规则。

<a id="provider_profiles"></a>
## 平台与协议目录

`internal/domain/platforms.go` 是具体平台的权威清单，保留 Qoder 的独立网关。分组、复合路由目标、用户平台额度、错误规则与渠道的平台集合从清单派生。`provider_profile.go` 进一步声明多协议供应商的默认模式、端点和模型分流规则；前端内置目录由后端生成并校验。

| 平台 | 接入模式 | 原生协议与边界 |
| --- | --- | --- |
| Kimi | payg / coding | Chat、Messages、Responses |
| 智谱 GLM | payg / coding | Chat、Messages；Responses 入站转换为 Chat |
| DeepSeek | payg | Chat、Messages、Responses；Responses 官方路径为 `/responses` |
| MiniMax | payg / coding | Chat、Messages、Responses；套餐由上游 Key 决定 |
| OpenCode | go / zen | 按模型和端点能力分流；保留存储平台 ID `opencode_go` |
| Command Code / Cline | profile 默认模式 | 按模型目录的支持端点和显式规则分流 |
| TypeSafe / Jev | API Key | 仅 `POST /v1/systemone`，不进入 Chat、Messages 或 Responses 转换链 |

多协议账号可配置 `api_protocol`、`account_mode`、`api_base_urls` 和 `protocol_rules`。显式固定协议优先；按模型分流时，管理员规则优先于上游模型目录，再回退 profile 默认规则。入站协议属于模型支持集合时同协议转发，否则使用首选协议。按入站自适应的供应商优先原生端点，缺少对应端点时回退 Chat。Chat 路径收到 Responses 形状正文也按实际正文选择协议。

每个实际端点仍经过已有 URL、Header 和代理安全边界。模型同步、协议能力与余额探测分别判定资格，不能把自定义兼容地址视为官方管理 API。未定价模型保持未知价格语义，需由管理员配置渠道价格。

<a id="native_bridges"></a>
## 原生协议转换与用量

原生 Messages 可承接 Chat 和 Responses 转换，SSE 解析同时限制行、事件和队列占用，并以读取空闲超时结束挂起上游。客户端断开后继续在有界上游生命周期内读取结算用量；写出成功终态前验证流终止与 DeepSeek 可计费用量。缓存读写桶与普通输入归一后再进入统一结算，避免重叠或漏计。

DeepSeek 的原生 Responses 请求强制无状态，不转发 `previous_response_id`，并适配图片输入形状。Ollama Cloud 的输出 token 上限默认 65535，可通过账号 `extra.ollama_max_tokens_cap` 覆盖；非正数关闭。原生 Responses、Chat 与 Messages 按实际官方端点和模型选择钩子。OpenAI 平台强制 Chat 的 Ollama 账号另补齐 `reasoning` / `thinking` 与 `reasoning_content` 的兼容字段，保留已有显式值。

<a id="provider_usage"></a>
## 上游额度与钱包

国产供应商余额和套餐窗口探测只访问已识别的官方端点。`CNProviderBalanceCheckService` 按进程配置周期刷新：按量余额过低时临时停调，恢复只清本服务写入的原因；套餐窗口快照交给调度阈值评估。

Command Code 的窗口与积分一起采样。Cline 的 ClinePass、积分和免费模型限额分别冷却：一个钱包耗尽不能封禁整账号或另一个钱包。探测失败保留已有状态，不伪造余额恢复。

OpenCode Go 用量刷新有独立全局开关，默认关闭。管理员启用全局和账号自动刷新后，实际发送模型请求才触发活动合并；静默期到达后刷新，持续请求则由最长等待限制强制刷新，成功刷新之间至少间隔五分钟。它按凭据和代理身份合并请求，通过分布式锁及身份 CAS 防止多实例重复刷新、旧结果回写。普通账号编辑不能覆盖受管快照和开关；身份变化清空相关状态，代理变化仅清快照。列表 DTO 返回窄用量状态，不暴露受管原始 extra。

管理端路径均位于 `/api/v1/admin/accounts`：`/opencode-go-usage/settings` 读写全局设置，`/:id/opencode-go-usage` 查询，`/:id/opencode-go-usage/auto-refresh` 修改账号开关，`/:id/opencode-go-usage/refresh` 手动刷新。手动刷新有独立速率限制。

<a id="system_one"></a>
## TypeSafe System One

`POST /v1/systemone` 接受 TypeSafe 原生非流式结构，验证模型与正文，提取 `state` 和各 question 的 instructions 参与内容审核。仅 TypeSafe 分组或明确路由到 TypeSafe 的 Composite 分组可以调用；其余生成入口拒绝 TypeSafe 平台。

请求沿用 fork 的 API Key 认证、用户与账号并发、余额及平台额度准入、故障转移和强制异步用量结算。排队的结算任务只复制计费元数据，不持有完整响应正文。Team、指定订阅、混合余额和发票边界仍由现有结算链管理。

相关文档：[上游账号能力矩阵](upstream_account_matrix.md)、[账号维护](../operations/account_maintenance.md)、[路由与结算](../domains/routing_and_billing.md)。
