package adminaccess

// forkDefinitions 补齐 fork 独有的管理资源；这些权限均明确作用于全站。
var forkDefinitions = []permissionDefinition{
	{Permission{"teams.read", "读取全站团队、成员及用量", "团队", "global"}, []string{"GET /teams", "GET /teams/:id", "GET /teams/:id/members", "GET /teams/:id/usage"}},
	{Permission{"teams.write", "创建及编辑全站团队", "团队", "global"}, []string{"POST /teams", "PATCH /teams/:id"}},
	{Permission{"teams.transfer", "强制转移团队所有权（另需二次验证）", "团队", "global"}, []string{"POST /teams/:id/force-transfer"}},
	{Permission{"teams.delete", "解散团队（另需二次验证）", "团队", "global"}, []string{"DELETE /teams/:id"}},
	{Permission{"invoices.read", "读取全站发票与附件（含敏感开票资料）", "发票", "global"}, []string{"GET /payment/invoices", "GET /payment/invoices/:id", "GET /payment/invoices/attachments/:attachment_id/download", "GET /payment/orders/:id/invoice"}},
	{Permission{"invoices.review", "审批及驳回发票申请", "发票", "global"}, []string{"POST /payment/invoices/:id/approve", "POST /payment/invoices/:id/reject"}},
	{Permission{"invoices.attachments.write", "上传及删除发票附件", "发票", "global"}, []string{"POST /payment/invoices/:id/attachments", "DELETE /payment/invoices/attachments/:attachment_id"}},
	{Permission{"invoices.issue", "开具及寄送发票", "发票", "global"}, []string{"POST /payment/invoices/:id/issue", "POST /payment/invoices/:id/send"}},
	{Permission{"data_sharing.config.read", "读取采集及导出配置", "数据共享", "global"}, []string{"GET /data-sharing/notice", "GET /data-sharing/skip-rules", "GET /data-sharing/storage-limit", "GET /data-sharing/runtime-settings", "GET /data-sharing/export-remote-config"}},
	{Permission{"data_sharing.config.write", "修改采集及导出配置", "数据共享", "global"}, []string{"PUT /data-sharing/notice", "PUT /data-sharing/skip-rules", "PUT /data-sharing/storage-limit", "PUT /data-sharing/runtime-settings", "PUT /data-sharing/export-remote-config", "POST /data-sharing/export-remote-config/test"}},
	{Permission{"data_sharing.read", "读取全站采集会话与产物（含正文）", "数据共享", "global"}, []string{"GET /data-sharing/filter-options", "GET /data-sharing/sessions", "GET /data-sharing/sessions/:id", "GET /data-sharing/exports", "GET /data-sharing/exports/:id", "GET /data-sharing/stats"}},
	{Permission{"data_sharing.export", "导出、下载及上传全站采集产物", "数据共享", "global"}, []string{"POST /data-sharing/sessions/:id/export-artifacts", "POST /data-sharing/exports", "POST /data-sharing/exports/:id/download-ticket", "POST /data-sharing/exports/:id/upload", "POST /data-sharing/exports/:id/upload/cancel", "GET /data-sharing/exports/:id/download-url"}},
	{Permission{"data_sharing.delete", "删除采集会话与导出产物", "数据共享", "global"}, []string{"DELETE /data-sharing/sessions/:id", "POST /data-sharing/sessions/batch-delete", "DELETE /data-sharing/exports/:id"}},
	{Permission{"storage.read", "读取发票及生图历史存储配置", "存储", "global"}, []string{"GET /settings/file-storage/invoice-attachments", "GET /settings/image-history-storage"}},
	{Permission{"storage.write", "修改私有存储目标（另需二次验证）", "存储", "global"}, []string{"PUT /settings/file-storage/invoice-attachments", "PUT /settings/image-history-storage", "POST /settings/file-storage/invoice-attachments/test", "POST /settings/image-history-storage/test"}},
	{Permission{"backups.read", "读取备份记录与配置", "备份", "global"}, []string{"GET /backups", "GET /backups/:id", "GET /backups/schedule", "GET /backups/content-config", "GET /backups/storage-config", "GET /backups/s3-config"}},
	{Permission{"backups.write", "配置备份策略及存储目标（敏感操作另需二次验证）", "备份", "global"}, []string{"PUT /backups/schedule", "PUT /backups/content-config", "PUT /backups/storage-config", "PUT /backups/s3-config", "POST /backups/storage-config/test", "POST /backups/s3-config/test"}},
	{Permission{"backups.create", "创建全库备份（另需二次验证）", "备份", "global"}, []string{"POST /backups"}},
	{Permission{"backups.download", "下载全库备份（另需二次验证）", "备份", "global"}, []string{"GET /backups/:id/download", "GET /backups/:id/download-url"}},
	{Permission{"backups.delete", "删除备份记录及文件", "备份", "global"}, []string{"DELETE /backups/:id"}},
	{Permission{"data_management.read", "读取数据管理配置及任务", "数据管理", "global"}, []string{"GET /data-management/agent/health", "GET /data-management/config", "GET /data-management/sources/:source_type/profiles", "GET /data-management/s3/profiles", "GET /data-management/backups", "GET /data-management/backups/:job_id"}},
	{Permission{"data_management.write", "修改数据来源及备份目标（敏感操作另需二次验证）", "数据管理", "global"}, []string{"PUT /data-management/config", "POST /data-management/sources/:source_type/profiles", "PUT /data-management/sources/:source_type/profiles/:profile_id", "DELETE /data-management/sources/:source_type/profiles/:profile_id", "POST /data-management/sources/:source_type/profiles/:profile_id/activate", "POST /data-management/s3/test", "POST /data-management/s3/profiles", "PUT /data-management/s3/profiles/:profile_id", "DELETE /data-management/s3/profiles/:profile_id", "POST /data-management/s3/profiles/:profile_id/activate", "POST /data-management/backups"}},
	{Permission{"scheduler.read", "读取全站账号调度评分及无状态预演", "调度", "global"}, []string{"GET /accounts/:id/advanced-scheduler-score", "POST /accounts/:id/advanced-scheduler-score/preview"}},
	{Permission{"groups.display.read", "读取全部分组展示候选（不限制调用）", "分组", "global"}, []string{"GET /groups/:id/models-list-candidates"}},
}

// allDefinitions 在初始化后只读，避免每次请求重新分配权限目录。
var allDefinitions = append(definitions, forkDefinitions...)
