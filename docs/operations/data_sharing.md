# 数据共享

本文描述分组显式开启的数据共享采集、会话聚合、质量评估、用户告知、导出和存储生命周期。它不定义普通 Usage/Ops 日志，也不表示所有成功请求都会被完整保存。

## 章节导航

- [启用与告知](#启用与告知)：修改 Group 开关或用户须知时读取。
- [采集流水线](#采集流水线)：修改协议捕获、worker、buffer 或跳过规则时读取。
- [会话与质量](#会话与质量)：修改 trajectory 合并、压缩或可导出状态时读取。
- [导出与存储](#导出与存储)：修改票据、产物、本地/S3 或备份时读取。
- [隐私与失败边界](#隐私与失败边界)：修改脱敏、容量或 fail-open 行为时读取。

## 启用与告知

只有当前 API Key 的 Group 设置 `data_sharing_enabled=true` 才进入采集。市场和分组选择面同步展示该标志；用户切换到数据共享分组前读取带版本的须知。关闭开关只停止新采集，不删除既有 session、导出产物或备份内容。

数据共享与内容审核、Usage log 和 Ops error capture 相互独立。它可能保存请求/响应正文，因此权限、留存和导出边界更严格；不能因为普通请求日志开启就隐式启用数据共享。

<a id="data_sharing_capture_pipeline"></a>
## 采集流水线

当前捕获入口覆盖 Claude/Gemini 兼容成功请求和 OpenAI 协议成功请求。处理器提交轻量 job 到有界 worker，按 trajectory/session 在进程 buffer 合并增量，空闲、容量或关闭时最终化并 UPSERT 数据库。OpenAI Responses 和 Messages 可以先保存 raw 增量，避免热路径重复压缩和质量扫描。

采集前应用可配置跳过规则。规则可按客户端族、request path、模型、字段作用域和 contains/equals 模式排除标题生成、预热、辅助模型等非训练轨迹；无效配置回退默认规则并告警。规则缓存有短 TTL，更新后主动清理本地缓存。

worker 缺失、队列满、buffer 丢弃或持久化暂时失败不会改变主请求和计费结果。持久化对连接重置等瞬时错误做有界退避和幂等重试；最终失败累计 dropped/error 统计。

## 会话与质量

会话按 trajectory 聚合，保存 provider、model、request path、User-Agent、状态、压缩 payload、存储字节和必要归属。最终化会压缩重放片段，并验证 system prompt、有效轮次、工具定义、结构化 tool call/result 配对和最终 assistant 等结构。

质量状态为 `complete`、`partial` 或 `invalid`。可通过尾部裁剪恢复完整前缀时标为 partial；兼容历史 payload 的规范化恢复也保留原错误列表。Usage 缺失当前不是质量硬门槛。导出前再次复核并清理 payload，不能只信任入库时状态。

## 导出与存储

用户与管理员使用不同 scope 的短期签名下载票据。即时导出支持 JSON/JSONL/zstd；大数据集可创建 pending/running/completed/failed/deleted 的预生成 artifact，记录进度、hash、行数和文件大小。服务重启会把中断的生成/上传任务标为失败，避免永久 running。

产物默认写受控本地目录，也可显式上传独立 S3/R2 端点。远端状态与本地产物状态分开，secret 加密保存，下载使用短期 presigned URL。删除需协调本地文件、远端对象和元数据；存储上限同时约束新采集。

数据库备份默认不包含 data-share sessions，只有显式选择对应内容组才进入 dump；本地/远端导出对象仍需独立备份和恢复。

## 隐私与失败边界

导出明确排除 `user_email`、IP、API Key/账号/用户 ID 和用户名等身份来源字段，并在写出前复核明显坏数据。新增字段时必须同时检查采集 payload、压缩/合并、质量验证、列表 DTO、导出 redaction 和备份范围。

采集是 fail-open 的附属链路：容量不足可以丢数据，但不能阻断已成功的推理或改变扣费。管理端统计暴露 worker、buffer、capture/export duration、质量和存储分布，以便发现静默降级。含用户内容的错误只记录摘要和归属，不写普通系统日志。

相关文档：[可观测性与数据生命周期](observability_and_data_lifecycle.md)、[网关策略控制](../domains/gateway_policy_controls.md)、[模型目录与市场](../interfaces/model_catalog_and_marketplace.md)。
