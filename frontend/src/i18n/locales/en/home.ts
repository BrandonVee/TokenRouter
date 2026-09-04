export default {
home: {
    viewOnGithub: 'View on GitHub',
    viewDocs: 'View Documentation',
    docs: 'Docs',
    switchToLight: 'Switch to Light Mode',
    switchToDark: 'Switch to Dark Mode',
    dashboard: 'Dashboard',
    login: 'Login',
    getStarted: 'Get Started',
    goToDashboard: 'Go to Dashboard',
    exploreMarketplace: 'Browse Models',
    viewAll: 'View all',
    nav: {
      searchModels: 'Search models, providers, and capabilities',
      models: 'Models'
    },
    // 面向用户的价值主张
    heroBadge: 'Unified access, smart routing, usage-based billing',
    heroTitle: 'The unified interface for AI models',
    heroSubtitle: 'One Key, All AI Models',
    heroDescription: 'No need to manage multiple subscriptions. Access Claude, GPT, Gemini and more with a single API key',
    tags: {
      subscriptionToApi: 'Subscription to API',
      stickySession: 'Session Persistence',
      realtimeBilling: 'Pay As You Go'
    },
    // 用户痛点区块
    painPoints: {
      title: 'Sound Familiar?',
      items: {
        expensive: {
          title: 'High Subscription Costs',
          desc: 'Paying for multiple AI subscriptions that add up every month'
        },
        complex: {
          title: 'Account Chaos',
          desc: 'Managing scattered accounts and API keys across different platforms'
        },
        unstable: {
          title: 'Service Interruptions',
          desc: 'Single accounts hitting rate limits and disrupting your workflow'
        },
        noControl: {
          title: 'No Usage Control',
          desc: "Can't track where your money goes or limit team member usage"
        }
      }
    },
    // 解决方案区块
    solutions: {
      title: 'We Solve These Problems',
      subtitle: 'Three simple steps to stress-free AI access'
    },
    features: {
      unifiedGateway: 'One-Click Access',
      unifiedGatewayDesc: 'Get a single API key to call all connected AI models. No separate applications needed.',
      multiAccount: 'Always Reliable',
      multiAccountDesc: 'Smart routing across multiple upstream accounts with automatic failover. Say goodbye to errors.',
      balanceQuota: 'Pay What You Use',
      balanceQuotaDesc: 'Usage-based billing with quota limits. Full visibility into team consumption.',
      dataPolicies: 'Custom Data Policies',
      dataPoliciesDesc: 'Manage models, quotas, and access scope in one place so team requests only use trusted services.',
      browseAll: 'Browse all',
      learnMore: 'Learn more',
      viewUsage: 'View usage',
      usageChart: 'Usage trend'
    },
    // 优势对比
    comparison: {
      title: 'Why Choose Us?',
      headers: {
        feature: 'Comparison',
        official: 'Official Subscriptions',
        us: 'Our Platform'
      },
      items: {
        pricing: {
          feature: 'Pricing',
          official: 'Fixed monthly fee, pay even if unused',
          us: 'Pay only for what you use'
        },
        models: {
          feature: 'Model Selection',
          official: 'Single provider only',
          us: 'Switch between models freely'
        },
        management: {
          feature: 'Account Management',
          official: 'Manage each service separately',
          us: 'Unified key, one dashboard'
        },
        stability: {
          feature: 'Stability',
          official: 'Single account rate limits',
          us: 'Multi-account pool, auto-failover'
        },
        control: {
          feature: 'Usage Control',
          official: 'Not available',
          us: 'Quotas & detailed analytics'
        }
      }
    },
    providers: {
      title: 'Supported AI Models',
      description: 'One API, Multiple Choices',
      supported: 'Supported',
      soon: 'Soon',
      claude: 'Claude',
      gpt: 'GPT',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      more: 'More',
      empty: 'No models to display yet',
      unavailable: 'Model data is temporarily unavailable',
      groups: 'service groups',
      modelCount: 'Models'
    },
    stats: {
      todayTokens: 'Today Total Tokens',
      totalTokens: 'Historical Total Tokens',
      totalUsers: 'Registered Users',
      supportedModels: 'Supported Models',
      providerTypes: 'model types',
      unavailable: 'Stats are temporarily unavailable'
    },
    steps: {
      signup: {
        title: 'Sign up',
        description: 'Create an account and add API keys. Team space and quotas can be configured later.'
      },
      browse: {
        title: 'Choose models',
        description: 'Use the model marketplace to compare supported models, provider types, and relative pricing.'
      },
      apiKey: {
        title: 'Get your API key',
        description: 'Send OpenAI-compatible, OpenAI Responses, or Anthropic-format requests and settle by actual token usage.'
      }
    },
    // CTA 区块
    cta: {
      title: 'Ready to Get Started?',
      description: 'Sign up now and get free trial credits to experience seamless AI access',
      button: 'Sign Up Free'
    },
    footer: {
      allRightsReserved: 'All rights reserved.',
      quickLinks: 'Quick links'
    },
    // 静态首页设计稿使用的全部展示文案，避免组件内维护双语副本。
    meteor: {
      nav: {
        platform: 'Platform',
        network: 'Network',
        pipeline: 'Pipeline',
        controlPlane: 'Control Plane',
        getApiKey: 'Get API key'
      },
      hero: {
        badge: 'ENTERPRISE GRADE',
        title: 'Large Model Service Platform',
        titlePrimary: 'Large Model',
        titleSecondary: 'Service Platform',
        subtitle: '99.9% SLA assurance · Global node coverage',
        supporting: 'Every model, every path, and every decision is unified by one control plane.',
        footnote: 'Built for AI systems moving from experiment to production.',
        startRouting: 'Get API key',
        seeHow: 'See how it works'
      },
      observability: {
        title: 'REAL-TIME OBSERVABILITY',
        regions: 'region: us-west-2 · tokyo-1'
      },
      metrics: {
        averageLatency: 'AVG LATENCY',
        uptime: 'UPTIME SLA',
        providers: 'PROVIDERS',
        values: { averageLatency: '<50ms', uptime: '99.9%', providers: '20+' }
      },
      providers: {
        heading: 'WORKS WITH LEADING MODEL PROVIDERS'
      },
      stats: {
        tokensToday: 'tokens today',
        tokensRouted: 'tokens routed',
        activeUsers: 'active users',
        supportedModels: 'supported models'
      },
      pillars: {
        eyebrow: 'THE PLATFORM',
        titlePrimary: 'One plane,',
        titleSecondary: 'three disciplines',
        items: {
          routing: {
            number: '01 · MODEL ROUTING',
            title: 'Route requests across frontier and private models',
            description: 'Weighted selection across providers by latency, cost and health, with automatic failover the moment an upstream degrades.'
          },
          governance: {
            number: '02 · GOVERNANCE LAYER',
            title: 'Control access, policy, cost, and compliance',
            description: 'Per-key budgets, organization-level policy, audit trails and spend caps enforced before a single token leaves the building.'
          },
          observability: {
            number: '03 · REAL-TIME OBSERVABILITY',
            title: 'Trace latency, quality, spend, and failures live',
            description: 'Every request traced end to end, with stage-level timings, token flow, and cost attribution streamed in real time.'
          }
        }
      },
      network: {
        eyebrow: 'GLOBAL NETWORK',
        titlePrimary: 'Land nearby,',
        titleSecondary: 'cross the Pacific faster',
        description: 'The primary node runs in California. Asia-Pacific requests land on the Tokyo acceleration node first and relay back from there, cutting out a round trip across the Pacific.',
        primary: 'Primary · California',
        acceleration: 'Acceleration · Tokyo',
        liveRequests: 'Live requests',
        canvasHint: 'DRAG TO ROTATE · WEBGL',
        nodes: { california: 'CA', tokyo: 'TK', london: 'LDN' },
        labels: { california: 'California · Primary', tokyo: 'Tokyo · Acceleration', frankfurt: 'Frankfurt', singapore: 'Singapore', sydney: 'Sydney', virginia: 'Virginia' },
        metrics: {
          routedMonthly: 'ROUTED MONTHLY',
          routedMonthlyValue: '1T tokens'
        }
      },
      pipeline: {
        eyebrow: 'REQUEST LIFECYCLE',
        titlePrimary: 'One request, six stages',
        titleSecondary: 'inside the platform',
        description: 'From the SDK call to the streamed response, every stage is observable, throttleable and reroutable.',
        inFlight: 'IN FLIGHT · END TO END',
        requestPath: '/v1/chat/completions',
        stages: {
          ingress: { name: 'Ingress', title: 'Request ingress', description: 'Accept an OpenAI-compatible request, normalize headers, and attach the trace context before it enters the platform.', chip: 'POST /v1/chat/completions' },
          auth: { name: 'Auth', title: 'Authentication', description: 'Resolve the API key, team scope, model permissions, and current quota in one cache-friendly lookup.', chip: 'key · tk_live-••••' },
          policy: { name: 'Policy', title: 'Policy check', description: 'Apply budget, rate, privacy, and model constraints before any upstream credentials are touched.', chip: 'allow · budget $2k' },
          routing: { name: 'Routing', title: 'Intelligent routing', description: 'Choose among healthy providers by group, weight, latency, cost, and circuit state, skipping attempts already tried.', chip: 'GPT-5.6 Sol · lowest latency' },
          upstream: { name: 'Upstream', title: 'Upstream call', description: 'Call the provider with its own credentials. Failures and throttles automatically retry against another eligible route.', chip: 'OpenAI · 38ms' },
          stream: { name: 'Stream out', title: 'Stream response', description: 'Tokens stream over SSE while usage, cost, and diagnostics are batched off the response path.', chip: '' }
        }
      },
      control: {
        eyebrow: 'CONTROL PLANE',
        title: 'The Control Plane for AI at Scale',
        description: 'Unify model access, routing, governance, observability, and cost control through one infrastructure layer.',
        tabs: {
          route: { tag: '01 / ROUTE', title: 'Intelligent routing', description: 'Model selection across providers, regions, and private deployments, scored on latency, cost, and reliability in real time.', features: ['Multi-provider weighted routing', 'Latency & cost aware selection', 'Automatic failover & retry', 'Private deployment support'] },
          govern: { tag: '02 / GOVERN', title: 'Centralized governance', description: 'Policy, permissions, audit trails, and budget enforcement in one place, applied before any request leaves the edge.', features: ['Per-key budget & spend caps', 'Organization-level policy', 'Full audit trails', 'Compliant by default'] },
          observe: { tag: '03 / OBSERVE', title: 'Live observability', description: 'Live traces for latency, quality, reliability, token flow, and spend, streamed as every request happens.', features: ['Stage-level trace timelines', 'Token & spend attribution', 'Quality scoring', 'Failure replay'] }
        },
        visual: {
          request: 'request', router: 'router', policyEngine: 'policy engine', budget: 'budget · $2k', rate: 'rpm · 1,200', allow: 'allow · gpt-*', deny: 'deny · pii', pass: 'pass ✓', ingress: 'ingress 2ms', auth: 'auth 3ms', route: 'route 4ms', done: 'done · 47ms',
          providers: { openai: 'OpenAI', gemini: 'Gemini', grok: 'Grok' }
        }
      },
      flow: {
        chaoticTraffic: 'CHAOTIC TRAFFIC',
        platform: 'Model Service Platform',
        stages: 'auth · route · govern · observe',
        orderedOutput: 'ORDERED OUTPUT',
        protocol: 'unified protocol · e2e observable',
        route: '01 · ROUTE',
        govern: '02 · GOVERN',
        observe: '03 · OBSERVE',
        description: 'Every request passes through the platform, authenticated, routed to the best model, and returned over one unified protocol.'
      },
      footer: {
        description: 'The large model service platform for AI moving from experiment to production.',
        product: 'PRODUCT',
        routing: 'Routing',
        governance: 'Governance',
        observability: 'Observability',
        costControl: 'Cost control',
        developers: 'DEVELOPERS',
        documentation: 'Documentation',
        apiReference: 'API reference',
        sdks: 'SDKs',
        status: 'Status',
        company: 'COMPANY',
        about: 'About',
        blog: 'Blog',
        careers: 'Careers',
        contact: 'Contact',
        regions: 'us-west-2 · tokyo-1 · eu-west-1'
      }
    }
  }
}
