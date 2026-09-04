export default {
home: {
    viewOnGithub: '在 GitHub 上查看',
    viewDocs: '查看文档',
    docs: '文档',
    switchToLight: '切换到浅色模式',
    switchToDark: '切换到深色模式',
    dashboard: '控制台',
    login: '登录',
    getStarted: '立即开始',
    goToDashboard: '进入控制台',
    exploreMarketplace: '浏览模型广场',
    viewAll: '查看全部',
    nav: {
      searchModels: '搜索模型、服务商和能力',
      models: '模型'
    },
    // 新增：面向用户的价值主张
    heroBadge: '统一接入、智能路由、按量计费',
    heroTitle: '统一的大模型接口',
    heroSubtitle: '一个密钥，畅用多个 AI 模型',
    heroDescription: '无需管理多个订阅账号，一站式接入 Claude、GPT、Gemini 等主流 AI 服务',
    tags: {
      subscriptionToApi: '订阅转 API',
      stickySession: '会话保持',
      realtimeBilling: '按量计费'
    },
    // 用户痛点区块
    painPoints: {
      title: '你是否也遇到这些问题？',
      items: {
        expensive: {
          title: '订阅费用高',
          desc: '每个 AI 服务都要单独订阅，每月支出越来越多'
        },
        complex: {
          title: '多账号难管理',
          desc: '不同平台的账号、密钥分散各处，管理起来很麻烦'
        },
        unstable: {
          title: '服务不稳定',
          desc: '单一账号容易触发限制，影响正常使用'
        },
        noControl: {
          title: '用量无法控制',
          desc: '不知道钱花在哪了，也无法限制团队成员的使用'
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: '我们帮你解决',
      subtitle: '简单三步，开始省心使用 AI'
    },
    features: {
      unifiedGateway: '一键接入',
      unifiedGatewayDesc: '获取一个 API 密钥，即可调用所有已接入的 AI 模型，无需分别申请。',
      multiAccount: '稳定可靠',
      multiAccountDesc: '智能调度多个上游账号，自动切换和负载均衡，告别频繁报错。',
      balanceQuota: '用多少付多少',
      balanceQuotaDesc: '按实际使用量计费，支持设置配额上限，团队用量一目了然。',
      dataPolicies: '数据策略可控',
      dataPoliciesDesc: '在统一入口管理模型、额度和访问范围，让团队请求走向可信的服务。',
      browseAll: '浏览全部',
      learnMore: '了解更多',
      viewUsage: '查看用量',
      usageChart: '用量趋势'
    },
    // 优势对比
    comparison: {
      title: '为什么选择我们？',
      headers: {
        feature: '对比项',
        official: '官方订阅',
        us: '本平台'
      },
      items: {
        pricing: {
          feature: '付费方式',
          official: '固定月费，用不完也付',
          us: '按量付费，用多少付多少'
        },
        models: {
          feature: '模型选择',
          official: '单一服务商',
          us: '多模型随意切换'
        },
        management: {
          feature: '账号管理',
          official: '每个服务单独管理',
          us: '统一密钥，一站管理'
        },
        stability: {
          feature: '服务稳定性',
          official: '单账号易触发限制',
          us: '多账号池，自动切换'
        },
        control: {
          feature: '用量控制',
          official: '无法限制',
          us: '可设配额、查明细'
        }
      }
    },
    providers: {
      title: '已支持的 AI 模型',
      description: '一个 API，多种选择',
      supported: '已支持',
      soon: '即将推出',
      claude: 'Claude',
      gpt: 'GPT',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: '更多',
      empty: '暂无可展示的模型',
      unavailable: '模型数据暂不可用',
      groups: '个服务组',
      modelCount: '模型'
    },
    stats: {
      todayTokens: '今日总 Token 量',
      totalTokens: '历史总 Token 量',
      totalUsers: '总注册用户量',
      supportedModels: '已接入模型',
      providerTypes: '类模型',
      unavailable: '统计数据暂不可用'
    },
    steps: {
      signup: {
        title: '注册账号',
        description: '登录后即可创建密钥，团队空间和额度配置可以后续再完善。'
      },
      browse: {
        title: '选择模型',
        description: '在模型广场查看已支持模型、服务类型和相对官方价格。'
      },
      apiKey: {
        title: '获取 API Key',
        description: '支持 OpenAI 兼容、OpenAI Responses 和 Anthropic 格式请求，按实际 Token 用量结算。'
      }
    },
    // CTA 区块
    cta: {
      title: '准备好开始了吗？',
      description: '注册即可获得免费试用额度，体验一站式 AI 服务',
      button: '免费注册'
    },
    footer: {
      allRightsReserved: '保留所有权利。',
      quickLinks: '快速链接'
    },
    // 静态首页设计稿使用的全部展示文案，避免组件内维护双语副本。
    meteor: {
      nav: {
        platform: '平台能力',
        network: '网络',
        pipeline: '请求链路',
        controlPlane: '控制面',
        getApiKey: '获取 API Key'
      },
      hero: {
        badge: '企业级',
        title: '大模型服务平台',
        titlePrimary: '大模型',
        titleSecondary: '服务平台',
        subtitle: '99.9% SLA 保障 · 全球节点覆盖',
        supporting: '每个模型、每条路径、每次决策，都由一个控制平面统一。',
        footnote: '为从实验走向生产的 AI 系统而建。',
        startRouting: '获取 API Key',
        seeHow: '查看工作原理'
      },
      observability: {
        title: '实时可观测性',
        regions: '区域：us-west-2 · tokyo-1'
      },
      metrics: {
        averageLatency: '平均延迟',
        uptime: '可用性 SLA',
        providers: '服务商',
        values: { averageLatency: '<50ms', uptime: '99.9%', providers: '20+' }
      },
      providers: {
        heading: '兼容主流大模型服务商'
      },
      stats: {
        tokensToday: '今日 tokens',
        tokensRouted: '累计路由 tokens',
        activeUsers: '活跃用户',
        supportedModels: '支持模型'
      },
      pillars: {
        eyebrow: '平台能力',
        titlePrimary: '一个控制面，',
        titleSecondary: '三项核心能力',
        items: {
          routing: {
            number: '01 · 模型路由',
            title: '在前沿与私有模型之间路由请求',
            description: '按延迟、成本与健康度加权选择服务商，上游降级时自动故障转移。'
          },
          governance: {
            number: '02 · 治理层',
            title: '统一控制访问、策略、成本与合规',
            description: '在 token 离开系统前执行密钥预算、组织策略、审计追踪与消费上限。'
          },
          observability: {
            number: '03 · 实时可观测性',
            title: '实时追踪延迟、质量、成本与故障',
            description: '端到端追踪每个请求，实时呈现阶段耗时、token 流与成本归属。'
          }
        }
      },
      network: {
        eyebrow: '全球网络',
        titlePrimary: '就近接入，',
        titleSecondary: '更快跨越太平洋',
        description: '主节点位于加州。亚太请求先落地东京加速节点再中继返回，减少一次跨太平洋往返。',
        primary: '主节点 · 加州',
        acceleration: '加速节点 · 东京',
        liveRequests: '实时请求',
        canvasHint: '拖动旋转 · WEBGL',
        nodes: { california: '加州', tokyo: '东京', london: '伦敦' },
        labels: { california: '加州 · 主节点', tokyo: '东京 · 加速节点', frankfurt: '法兰克福', singapore: '新加坡', sydney: '悉尼', virginia: '弗吉尼亚' },
        metrics: {
          routedMonthly: '月度路由量',
          routedMonthlyValue: '100T tokens'
        }
      },
      pipeline: {
        eyebrow: '请求生命周期',
        titlePrimary: '一个请求，平台内六个阶段',
        titleSecondary: '全程可观测',
        description: '从 SDK 调用到流式响应，每个阶段都可观测、可限流、可重新路由。',
        inFlight: '请求处理中 · 端到端',
        requestPath: '/v1/chat/completions',
        stages: {
          ingress: { name: '入口', title: '请求进入', description: '接收 OpenAI 兼容请求，规范化请求头并注入追踪上下文。', chip: 'POST /v1/chat/completions' },
          auth: { name: '认证', title: '身份认证', description: '一次缓存友好的查询中解析 API 密钥、团队范围、模型权限与当前配额。', chip: '密钥 · tk_live-••••' },
          policy: { name: '策略', title: '策略检查', description: '在接触上游凭据前应用预算、速率、隐私与模型约束。', chip: '允许 · 预算 $2k' },
          routing: { name: '路由', title: '智能路由', description: '按分组、权重、延迟、成本和熔断状态选择健康服务商，跳过已尝试的路径。', chip: 'GPT-5.6 Sol · 最低延迟' },
          upstream: { name: '上游', title: '调用上游', description: '使用服务商凭据发起调用，失败或限流时自动切换到其他可用路由。', chip: 'OpenAI · 38ms' },
          stream: { name: '输出', title: '流式响应', description: '通过 SSE 流式返回 token，同时将用量、成本和诊断信息异步写入存储。', chip: '' }
        }
      },
      control: {
        eyebrow: '控制面',
        title: '面向规模化 AI 的控制面',
        description: '通过一个基础设施层统一模型访问、路由、治理、可观测性与成本控制。',
        tabs: {
          route: { tag: '01 / 路由', title: '智能路由', description: '跨服务商、区域和私有部署选择模型，实时按照延迟、成本与可靠性评分。', features: ['多服务商加权路由', '感知延迟与成本的选择', '自动故障转移与重试', '支持私有化部署'] },
          govern: { tag: '02 / 治理', title: '集中式治理', description: '在统一控制面管理策略、权限、审计与预算，并在请求离开边缘前执行。', features: ['按密钥预算与消费上限', '组织级策略', '完整审计追踪', '默认合规'] },
          observe: { tag: '03 / 观测', title: '实时可观测性', description: '实时追踪延迟、质量、可靠性、token 流和成本，让每个请求都可观测。', features: ['阶段级追踪时间线', 'Token 与成本归属', '质量评分', '故障回放'] }
        },
        visual: {
          request: '请求', router: '路由器', policyEngine: '策略引擎', budget: '预算 · $2k', rate: '每分钟请求 · 1,200', allow: '允许 · gpt-*', deny: '拒绝 · pii', pass: '通过 ✓', ingress: '入口 2ms', auth: '认证 3ms', route: '路由 4ms', done: '完成 · 47ms',
          providers: { openai: 'OpenAI', gemini: 'Gemini', grok: 'Grok' }
        }
      },
      flow: {
        chaoticTraffic: '混乱流量',
        platform: '大模型服务平台',
        stages: '认证 · 路由 · 治理 · 观测',
        orderedOutput: '有序输出',
        protocol: '统一协议 · 端到端可观测',
        route: '01 · 路由',
        govern: '02 · 治理',
        observe: '03 · 观测',
        description: '每个请求都经过平台认证，路由到最佳模型，并通过统一协议返回。'
      },
      footer: {
        description: '帮助 AI 从实验走向生产的大模型服务平台。',
        product: '产品',
        routing: '路由',
        governance: '治理',
        observability: '可观测性',
        costControl: '成本控制',
        developers: '开发者',
        documentation: '文档',
        apiReference: 'API 参考',
        sdks: 'SDK',
        status: '状态',
        company: '公司',
        about: '关于',
        blog: '博客',
        careers: '招聘',
        contact: '联系',
        regions: 'us-west-2 · tokyo-1 · eu-west-1'
      }
    }
  }
}
