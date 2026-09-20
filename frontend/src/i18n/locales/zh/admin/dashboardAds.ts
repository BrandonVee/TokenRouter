export default {
// 仪表盘广告管理页文案
  dashboardAds: {
    title: '仪表盘广告',
    description: '管理用户仪表盘展示的广告内容与有效期。',
    workspace: '内容展示管理',
    save: '保存广告',
    loadFailed: '广告加载失败，请重试。',
    retry: '重试',
    adIndex: '广告 {index}',
    moveUp: '上移',
    moveDown: '下移',
    emptyTitle: '创建第一条仪表盘广告',
    empty: '添加图片、跳转链接和展示周期，即可在用户仪表盘发布广告。',
    add: '添加广告',
    addAnother: '继续添加广告',
    summary: {
      total: '广告总数',
      enabled: '已启用',
      scheduled: '待生效'
    },
    status: {
      active: '展示中',
      scheduled: '待生效',
      expired: '已过期',
      disabled: '已停用'
    },
    preview: '仪表盘预览',
    previewHint: '预览会实时反映图片适应方式。',
    previewEmpty: '等待广告素材',
    previewEmptyHint: '上传图片或粘贴图片地址后可在此预览。',
    uploadSection: '本地素材',
    imageUploadLabel: '选择图片',
    imageRemoveLabel: '移除',
    imageHint: '最大 5 MB，上传后自动压缩',
    orUseImageUrl: '或使用图片地址',
    imageUrlPlaceholder: 'https://cdn.example.com/banner.jpg',
    imageUrlHint: '支持 HTTPS、站内相对地址或上传后生成的数据地址。',
    delivery: '展示设置',
    deliveryHint: '配置广告的状态、跳转行为和展示周期。',
    visibility: '广告状态',
    visibilityEnabled: '已启用，将按照展示周期投放。',
    visibilityDisabled: '已停用，用户仪表盘不会展示。',
    linkUrl: '跳转链接',
    linkUrlPlaceholder: 'https://example.com',
    fitMode: '图片适应方式',
    fitModes: {
      adaptive: '自适应（保持比例）',
      cover: '填充（裁剪超出部分）',
      fill: '拉伸（铺满区域）'
    },
    schedule: '展示周期',
    scheduleHint: '不设置过期时间时，广告会持续展示至手动停用。',
    startsAt: '开始时间',
    endsAt: '过期时间'
  }
}
