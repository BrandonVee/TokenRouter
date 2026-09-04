<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useAuthStore, useAppStore } from "@/stores";
import { useTheme } from "@/composables/useTheme";
import { sanitizeUrl } from '@/utils/url';
import { hasAcceptedLoginAgreement } from "@/utils/loginAgreement";
import { isGoogleOneTapEligible, isGoogleOneTapOriginSupported } from "@/utils/googleIdentity";
import LocaleSwitcher from "@/components/common/LocaleSwitcher.vue";
import Icon from "@/components/icons/Icon.vue";
import GoogleOneTap from "@/components/auth/GoogleOneTap.vue";
import ProviderIcon from "@/components/common/ProviderIcon.vue";
import { getMarketplaceModels, getMarketplaceStats } from "@/api/marketplace";
import type { MarketplaceGroup, MarketplaceStats } from "@/types";
import createGlobe, { type COBEOptions } from "cobe";

const { locale, t } = useI18n();
const authStore = useAuthStore();
const appStore = useAppStore();
const { isDark, toggleTheme } = useTheme();
const isAuthenticated = computed(() => authStore.isAuthenticated);
const dashboardPath = computed(() => (authStore.isAdmin ? "/admin/dashboard" : "/dashboard"));
const siteName = computed(() => appStore.siteName || "Meteor");
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || "", { allowRelative: true, allowDataUrl: true }));
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ""));
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || "");
const isHomeContentUrl = computed(() => /^https?:\/\//.test(homeContent.value.trim()));
const isZh = computed(() => String(locale.value).toLowerCase().startsWith("zh"));
const heroTitleParts = computed(() => {
  return [t("home.meteor.hero.titlePrimary"), t("home.meteor.hero.titleSecondary")];
});
const googleOneTapClientID = computed(() => appStore.cachedPublicSettings?.google_oauth_client_id || "");
const googleOneTapEligible = computed(() => {
  const settings = appStore.cachedPublicSettings;
  if (!settings) return false;
  const agreementEnabled = settings.login_agreement_enabled === true;
  return isGoogleOneTapEligible({
    publicSettingsLoaded: appStore.publicSettingsLoaded,
    isAuthenticated: isAuthenticated.value,
    oneTapEnabled: settings.google_one_tap_enabled === true,
    clientID: googleOneTapClientID.value,
    backendModeEnabled: settings.backend_mode_enabled,
    tencentCaptchaEnabled: settings.tencent_captcha_enabled === true,
    aliyunCaptchaEnabled: settings.aliyun_captcha_enabled === true,
    loginAgreementEnabled: agreementEnabled,
    loginAgreementAccepted: !agreementEnabled || hasAcceptedLoginAgreement(settings.login_agreement_revision || ""),
    originSupported: isGoogleOneTapOriginSupported(),
  });
});

// ---------- 市场数据：供应商行与统计带，接口失败时回退到静态设计稿数值 ----------
const marketplaceGroups = ref<MarketplaceGroup[]>([]);
const marketplaceStats = ref<MarketplaceStats | null>(null);
const marketplaceLoading = ref(true);
const marketplaceError = ref(false);
const statsError = ref(false);
const totalModelCount = computed(() => marketplaceGroups.value.reduce((total, group) => total + group.models.length, 0));
const providerRowsFromApi = computed(() => {
  // 上架分组与静态兜底列表按名称统一去重（含分组之间的重名），
  // 避免跑马灯里同名供应商连续出现。
  const seen = new Set<string>();
  const rows: Array<[string, string, string]> = [];
  const pushRow = (label: string, color: string, tag: string) => {
    const key = label.trim().toLowerCase();
    if (!label.trim() || seen.has(key)) return;
    seen.add(key);
    rows.push([label.trim(), color, tag]);
  };
  for (const group of marketplaceGroups.value.filter((g) => g.models.length > 0).slice(0, 20)) {
    const label = group.display_brand?.trim() || group.name.trim() || group.platform;
    pushRow(label, providerColor(label), label.slice(0, 2).toUpperCase());
  }
  for (const p of providers) pushRow(p[0], p[1], p[2]);
  return rows;
});
const displayProviderRows = computed(() => {
  const base = providerRowsFromApi.value;
  // 复制基础列表直至数量充足，宽屏下也能铺满整条跑马灯并无缝循环。
  const rows = [...base];
  while (rows.length > 0 && rows.length < 28) rows.push(...base);
  return rows;
});
const statValue = (value: number | undefined, fallback: string) => (value == null ? fallback : new Intl.NumberFormat(isZh.value ? "zh-CN" : "en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value));
const displayStats = computed(() => [
  [statValue(marketplaceStats.value?.today_tokens, "..."), t("home.meteor.stats.tokensToday")],
  [statValue(marketplaceStats.value?.total_tokens, "..."), t("home.meteor.stats.tokensRouted")],
  [statValue(marketplaceStats.value?.total_users, "..."), t("home.meteor.stats.activeUsers")],
  [marketplaceLoading.value ? "..." : new Intl.NumberFormat(isZh.value ? "zh-CN" : "en-US").format(totalModelCount.value), t("home.meteor.stats.supportedModels")],
]);
const requestPath = computed(() => t("home.meteor.pipeline.requestPath"));
const metricValues = computed(() => ({
  averageLatency: t("home.meteor.metrics.values.averageLatency"),
  uptime: t("home.meteor.metrics.values.uptime"),
  providers: t("home.meteor.metrics.values.providers"),
}));
function providerColor(label: string): string {
  const colors = ["#10a37f", "#d97757", "#4796e3", "#94a3b8", "#0668e1", "#fa520f", "#4d6bfe", "#a855f7"];
  return colors[Math.abs([...label].reduce((hash, char) => hash + char.charCodeAt(0), 0)) % colors.length];
}
async function loadMarketplace() {
  marketplaceLoading.value = true;
  try {
    marketplaceGroups.value = await getMarketplaceModels();
  } catch (error) {
    marketplaceError.value = true;
    console.error("Failed to load homepage marketplace:", error);
  } finally {
    marketplaceLoading.value = false;
  }
}
async function loadStats() {
  try {
    marketplaceStats.value = await getMarketplaceStats();
  } catch (error) {
    statsError.value = true;
    console.error("Failed to load homepage stats:", error);
  }
}

// 静态兜底供应商列表：覆盖国内外主流服务商与多种模态（对话/图像/音乐/智能体），
// 名称必须能被 resolveProviderBrand 解析，跑马灯才能展示厂商自有 icon。
const providers: Array<[string, string, string]> = [
  // 国际服务商
  ["OpenAI", "#10a37f", "OA"],
  ["Anthropic", "#d97757", "AN"],
  ["Google", "#4796e3", "GO"],
  ["xAI", "#94a3b8", "XA"],
  ["Meta", "#0668e1", "ME"],
  ["Mistral", "#fa520f", "MI"],
  ["Cohere", "#39594d", "CO"],
  ["Perplexity", "#20808d", "PX"],
  ["Midjourney", "#111827", "MJ"],
  ["Suno", "#f87171", "SU"],
  ["Ollama", "#0f6b5c", "OL"],
  ["OpenRouter", "#a855f7", "OR"],
  ["Cloudflare", "#ff9900", "CF"],
  ["Jina", "#ff6d00", "JI"],
  // 国内服务商
  ["DeepSeek", "#4d6bfe", "DS"],
  ["Qwen", "#615ced", "QW"],
  ["Moonshot", "#8b5cf6", "MO"],
  ["Zhipu", "#3859ff", "ZP"],
  ["Doubao", "#2b7fff", "DB"],
  ["Hunyuan", "#0052d9", "HY"],
  ["Baidu", "#2932e1", "BD"],
  ["iFlytek", "#157ef3", "IF"],
  ["MiniMax", "#f03e5a", "MM"],
  ["Xiaomi", "#ff6900", "MI"],
  ["360", "#00b96b", "36"],
  ["ZeroOne", "#f5a623", "ZO"],
  ["Coze", "#4d6bfe", "CZ"],
  ["Dify", "#7dd3fc", "DF"],
];

// ---------- 实时观测流：复刻设计稿的随机模型 + 随机延迟推送行为 ----------
// brand 字段必须是 resolveProviderBrand 可解析的厂商键，行内展示厂商自有 icon。
interface StreamRow {
  id: number;
  name: string;
  brand: string;
  latency: number;
  ratio: number;
}
const streamModels: Array<{ name: string; brand: string; min: number; max: number }> = [
  { name: "GPT-5.6 Sol", brand: "openai", min: 14, max: 34 },
  { name: "Claude Fable 5.1", brand: "anthropic", min: 16, max: 42 },
  { name: "Claude Opus 5", brand: "anthropic", min: 15, max: 38 },
  { name: "Gemini 3.1 Pro", brand: "google", min: 18, max: 36 },
  { name: "Gemini 3.8 Flash", brand: "google", min: 10, max: 22 },
  { name: "Grok 4.6", brand: "xai", min: 17, max: 40 },
  { name: "GPT-5.6 Terra", brand: "openai", min: 20, max: 44 },
  { name: "Llama 4 Scout", brand: "meta", min: 22, max: 50 },
];
const streamRows = ref<StreamRow[]>([]);
let streamId = 0;
function pushStreamRow() {
  const model = streamModels[Math.random() * streamModels.length | 0];
  const latency = model.min + Math.random() * (model.max - model.min) | 0;
  streamRows.value = [
    { id: ++streamId, name: model.name, brand: model.brand, latency, ratio: Math.round((latency / model.max) * 100) },
    ...streamRows.value,
  ].slice(0, 5);
}
for (let i = 0; i < 5; i += 1) pushStreamRow();

// ---------- 请求管线：六个阶段，底部进度条与累计耗时复刻设计稿节奏 ----------
const stages = [
  { key: "ingress", duration: "2ms" },
  { key: "auth", duration: "3ms" },
  { key: "policy", duration: "4ms" },
  { key: "routing", duration: "4ms" },
  { key: "upstream", duration: "38ms" },
  { key: "stream", duration: "—" },
];
// 每个阶段边界的累计端到端耗时（ms），与阶段耗时标签一致。
const cum = [0, 2, 5, 9, 13, 51, 51];
const controlTabs = [
  { key: "route", type: "route" },
  { key: "govern", type: "govern" },
  { key: "observe", type: "observe" },
];
const activeStage = ref(0);
const activeControl = ref(0);
const progress = ref(0);
const scrolled = ref(false);
const webglFailed = ref(false);
const globeCanvas = ref<HTMLCanvasElement | null>(null);
let timer: number | undefined;
let streamTimer: number | undefined;
let globeCleanup: (() => void) | undefined;
let revealObserver: IntersectionObserver | undefined;
const onScroll = () => {
  scrolled.value = window.scrollY > 10;
};
const selectStage = (index: number) => {
  activeStage.value = index;
  progress.value = 0;
};
// 管线头部展示当前阶段完成时刻的累计耗时。
const pipelineMs = computed(() => Math.round(cum[activeStage.value] + ((cum[activeStage.value + 1] - cum[activeStage.value]) * progress.value) / 100));
// 全轨道连续进度：六个阶段均分轨道宽度，段内再按进度推进，保证进度线与视觉分段对齐。
const trackPercent = computed(() => ((activeStage.value + progress.value / 100) / stages.length) * 100);

// ---------- 控制面右侧 SVG 可视化：路由 / 治理 / 观测，文案绑定 i18n ----------
const visualProviders = computed(() => ({
  openai: t("home.meteor.control.visual.providers.openai"),
  gemini: t("home.meteor.control.visual.providers.gemini"),
  grok: t("home.meteor.control.visual.providers.grok"),
}));
const vizRoute = computed(() => `
  <g font-family="monospace" font-size="10">
    <circle cx="60" cy="140" r="5" fill="#565e6e"/>
    <text x="60" y="166" fill="#565e6e" text-anchor="middle">${t("home.meteor.control.visual.request")}</text>
    <path d="M70 140 H 130" stroke="rgba(255,255,255,.2)" stroke-width="1.2" class="viz-line"/>
    <rect x="130" y="118" width="70" height="44" rx="8" fill="rgba(18,167,232,.06)" stroke="rgba(18,167,232,.4)"/>
    <text x="165" y="144" fill="#12a7e8" text-anchor="middle">${t("home.meteor.control.visual.router")}</text>
    <path d="M200 130 Q 240 100 280 84" stroke="rgba(16,163,127,.5)" stroke-width="1.2" class="viz-line"/>
    <path d="M200 140 H 280" stroke="rgba(71,150,227,.5)" stroke-width="1.2" class="viz-line"/>
    <path d="M200 150 Q 240 180 280 196" stroke="rgba(148,163,184,.5)" stroke-width="1.2" class="viz-line"/>
    <g>
      <rect x="280" y="66" width="76" height="34" rx="7" fill="rgba(16,163,127,.08)" stroke="rgba(16,163,127,.4)"/>
      <text x="318" y="87" fill="#10a37f" text-anchor="middle">${visualProviders.value.openai}</text>
      <rect x="280" y="123" width="76" height="34" rx="7" fill="rgba(71,150,227,.08)" stroke="rgba(71,150,227,.4)"/>
      <text x="318" y="144" fill="#4796e3" text-anchor="middle">${visualProviders.value.gemini}</text>
      <rect x="280" y="180" width="76" height="34" rx="7" fill="rgba(148,163,184,.08)" stroke="rgba(148,163,184,.4)"/>
      <text x="318" y="201" fill="#94a3b8" text-anchor="middle">${visualProviders.value.grok}</text>
    </g>
    <text x="318" y="36" fill="#8b93a3" text-anchor="middle">weighted pick · lowest latency</text>
    <text x="318" y="52" fill="#12a7e8" text-anchor="middle">→ ${visualProviders.value.openai} (14ms)</text>
  </g>`);
const vizGovern = computed(() => `
  <g font-family="monospace" font-size="10">
    <rect x="120" y="40" width="140" height="200" rx="12" fill="rgba(125,211,252,.05)" stroke="rgba(125,211,252,.35)"/>
    <text x="190" y="66" fill="#7dd3fc" text-anchor="middle">${t("home.meteor.control.visual.policyEngine")}</text>
    <g fill="none" stroke="rgba(125,211,252,.4)" stroke-width="1">
      <rect x="140" y="86" width="100" height="26" rx="6"/>
      <rect x="140" y="122" width="100" height="26" rx="6"/>
      <rect x="140" y="158" width="100" height="26" rx="6"/>
      <rect x="140" y="194" width="100" height="26" rx="6"/>
    </g>
    <g fill="#8b93a3" text-anchor="middle">
      <text x="190" y="103">${t("home.meteor.control.visual.budget")}</text>
      <text x="190" y="139">${t("home.meteor.control.visual.rate")}</text>
      <text x="190" y="175">${t("home.meteor.control.visual.allow")}</text>
      <text x="190" y="211">${t("home.meteor.control.visual.deny")}</text>
    </g>
    <circle cx="60" cy="140" r="5" fill="#565e6e"/>
    <path d="M65 140 H 115" stroke="rgba(255,255,255,.2)" stroke-width="1.2" class="viz-line"/>
    <path d="M260 140 H 320" stroke="rgba(18,167,232,.5)" stroke-width="1.2" class="viz-line"/>
    <text x="60" y="166" fill="#565e6e" text-anchor="middle">req</text>
    <text x="340" y="136" fill="#12a7e8">${t("home.meteor.control.visual.pass")}</text>
    <text x="340" y="152" fill="#565e6e" font-size="9">audit logged</text>
  </g>`);
const vizObserve = computed(() => `
  <g font-family="monospace" font-size="10">
    <path d="M60 220 L 130 220 L 130 150 L 210 150 L 210 180 L 290 180 L 290 100 L 350 100"
      stroke="#12a7e8" stroke-width="1.6" fill="none" opacity=".8"/>
    <path d="M60 220 L 130 220 L 130 150 L 210 150 L 210 180 L 290 180 L 290 100 L 350 100"
      stroke="#7dd3fc" stroke-width="3" fill="none" opacity=".25" stroke-dasharray="40 560">
      <animate attributeName="stroke-dasharray" values="0 600;40 560;0 600" dur="3s" repeatCount="indefinite"/>
    </path>
    <g fill="#565e6e">
      <circle cx="130" cy="220" r="3.5"/><circle cx="130" cy="150" r="3.5"/>
      <circle cx="210" cy="150" r="3.5"/><circle cx="210" cy="180" r="3.5"/>
      <circle cx="290" cy="180" r="3.5"/><circle cx="290" cy="100" r="3.5"/>
    </g>
    <g fill="#8b93a3" font-size="9.5">
      <text x="130" y="240" text-anchor="middle">${t("home.meteor.control.visual.ingress")}</text>
      <text x="210" y="132" text-anchor="middle">${t("home.meteor.control.visual.auth")}</text>
      <text x="210" y="204" text-anchor="middle">${t("home.meteor.control.visual.route")}</text>
      <text x="290" y="202" text-anchor="middle">upstream 38ms</text>
      <text x="352" y="96" fill="#12a7e8" text-anchor="end">${t("home.meteor.control.visual.done")}</text>
    </g>
    <text x="60" y="196" fill="#565e6e">trace</text>
  </g>`);
const cpViz = computed(() => {
  const type = controlTabs[activeControl.value].type;
  if (type === "route") return vizRoute.value;
  if (type === "govern") return vizGovern.value;
  return vizObserve.value;
});

// 主题自适应：canvas 背景色跟随 html.dark 的切换。
const isDarkMode = () => document.documentElement.classList.contains("dark");

// ---------- 地球：cobe 点阵地球（原生 WebGL，约 5KB，无 three.js）----------
// 斐波那契点阵大陆 + 城市标记 + 航线弧线，亮暗主题在渲染循环中实时切换。
function setupCobeGlobe(canvas: HTMLCanvasElement): (() => void) | undefined {
  let phi = 4.4;
  let theta = 0.25;
  let dragging = false;
  let lastX = 0;
  let lastY = 0;
  let raf = 0;
  let visible = true;
  const onPointerDown = (event: PointerEvent) => {
    dragging = true;
    lastX = event.clientX;
    lastY = event.clientY;
    canvas.setPointerCapture(event.pointerId);
  };
  const onPointerMove = (event: PointerEvent) => {
    if (!dragging) return;
    phi += (event.clientX - lastX) * 0.005;
    theta = Math.max(-1.1, Math.min(1.1, theta + (event.clientY - lastY) * 0.004));
    lastX = event.clientX;
    lastY = event.clientY;
  };
  const onPointerUp = () => {
    dragging = false;
  };
  canvas.addEventListener("pointerdown", onPointerDown);
  window.addEventListener("pointermove", onPointerMove);
  window.addEventListener("pointerup", onPointerUp);

  let globe: { update: (state: Partial<COBEOptions>) => void; destroy: () => void } | undefined;
  try {
    globe = createGlobe(canvas, {
      devicePixelRatio: Math.min(window.devicePixelRatio || 1, 2),
      width: canvas.clientWidth,
      height: canvas.clientHeight,
      phi,
      theta,
      dark: 1,
      diffuse: 1.4,
      mapSamples: 16000,
      mapBrightness: 6.5,
      baseColor: [0.32, 0.38, 0.5],
      markerColor: [1, 0.72, 0.3],
      glowColor: [0.12, 0.16, 0.28],
      markerElevation: 0.02,
      // 城市标记：加州主节点（琥珀金）+ 东京加速节点（天蓝）。
      markers: [
        { location: [37.77, -122.42], size: 0.09, color: [1, 0.72, 0.3] },
        { location: [35.68, 139.69], size: 0.07, color: [0.49, 0.83, 0.99] },
      ],
      // 航线弧线：亚太城市汇入东京，东京与西方城市直连加州。
      arcs: [
        { from: [35.68, 139.69], to: [37.77, -122.42], color: [0.49, 0.83, 0.99] },
        { from: [31.23, 121.47], to: [35.68, 139.69] },
        { from: [1.35, 103.82], to: [35.68, 139.69] },
        { from: [37.57, 126.98], to: [35.68, 139.69] },
        { from: [-33.87, 151.21], to: [35.68, 139.69] },
        { from: [51.51, -0.13], to: [37.77, -122.42] },
        { from: [40.71, -74.01], to: [37.77, -122.42] },
      ],
      arcColor: [0.18, 0.55, 0.85],
      arcWidth: 0.35,
      arcHeight: 0.35,
    });
  } catch {
    // WebGL 不可用时降级为纯 CSS 背景。
    webglFailed.value = true;
    return undefined;
  }

  // cobe v2 不带动画循环，由外部 rAF 驱动逐帧 update。
  const visibility = new IntersectionObserver((entries) => {
    visible = entries[0].isIntersecting;
  });
  visibility.observe(canvas);
  const render = () => {
    raf = window.requestAnimationFrame(render);
    if (!visible || !globe) return;
    // 画布尺寸随容器实时变化；拖拽时暂停自转。
    const dark = isDarkMode();
    globe.update({
      width: canvas.clientWidth,
      height: canvas.clientHeight,
      phi,
      theta,
      dark: dark ? 1 : 0,
      diffuse: dark ? 1.4 : 1.1,
      mapBrightness: dark ? 6.5 : 7.5,
      baseColor: dark ? [0.32, 0.38, 0.5] : [0.88, 0.91, 0.96],
      glowColor: dark ? [0.12, 0.16, 0.28] : [0.72, 0.78, 0.88],
      markerColor: [1, 0.72, 0.3],
      arcColor: dark ? [0.18, 0.55, 0.85] : [0.12, 0.45, 0.75],
    });
    if (!dragging) phi += 0.0035;
  };
  render();

  return () => {
    window.cancelAnimationFrame(raf);
    visibility.disconnect();
    globe?.destroy();
    canvas.removeEventListener("pointerdown", onPointerDown);
    window.removeEventListener("pointermove", onPointerMove);
    window.removeEventListener("pointerup", onPointerUp);
  };
}

// 首页动画只在组件挂载期间运行，离开页面时统一释放监听器、定时器与 GPU 资源。
onMounted(() => {
  window.addEventListener("scroll", onScroll, { passive: true });
  authStore.checkAuth();
  void Promise.all([loadMarketplace(), loadStats()]);
  // 滚动进入视口时上浮显现。
  revealObserver = new IntersectionObserver((entries) => entries.forEach((entry) => entry.isIntersecting && entry.target.classList.add("in")), { threshold: 0.12 });
  document.querySelectorAll(".reveal").forEach((el) => revealObserver?.observe(el));
  if (globeCanvas.value) globeCleanup = setupCobeGlobe(globeCanvas.value) ?? undefined;
  streamTimer = window.setInterval(pushStreamRow, 2200);
  timer = window.setInterval(() => {
    progress.value += 2;
    if (progress.value >= 100) selectStage((activeStage.value + 1) % stages.length);
  }, 80);
});
onBeforeUnmount(() => {
  window.removeEventListener("scroll", onScroll);
  if (timer) window.clearInterval(timer);
  if (streamTimer) window.clearInterval(streamTimer);
  globeCleanup?.();
  revealObserver?.disconnect();
});
</script>

<template>
  <GoogleOneTap :enabled="googleOneTapEligible" :client-id="googleOneTapClientID" />
  <div v-if="homeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0" allowfullscreen />
    <div v-else v-html="homeContent" />
  </div>
  <div v-else class="meteor-page" :class="{ 'no-webgl': webglFailed }">
    <div class="aurora" aria-hidden="true"><i class="b1" /><i class="b2" /><i class="b3" /></div>
    <div class="grid-fx" aria-hidden="true" />

    <nav :class="{ scrolled }">
      <div class="nav-in">
        <router-link to="/home" class="logo">
          <span v-if="siteLogo" class="logo-mark"><img :src="siteLogo" alt="" /></span>
          <span>{{ siteName }}</span>
        </router-link>
        <div class="nav-links">
          <a href="#pillars">{{ t("home.meteor.nav.platform") }}</a>
          <a href="#network">{{ t("home.meteor.nav.network") }}</a>
          <a href="#pipeline">{{ t("home.meteor.nav.pipeline") }}</a>
          <a href="#control">{{ t("home.meteor.nav.controlPlane") }}</a>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t("home.docs") }}</a>
        </div>
        <div class="nav-cta">
          <LocaleSwitcher />
          <button class="icon-btn" type="button" :title="isDark ? t('home.switchToLight') : t('home.switchToDark')" @click="toggleTheme">
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
          <router-link v-if="isAuthenticated" :to="dashboardPath" class="btn sm">{{ t("home.dashboard") }}</router-link>
          <router-link v-else to="/login" class="btn sm">{{ t("home.login") }}</router-link>
          <router-link v-if="!isAuthenticated" to="/register" class="btn sm primary">{{ t("home.meteor.nav.getApiKey") }}</router-link>
        </div>
      </div>
    </nav>

    <!-- ============ HERO ============ -->
    <header class="hero">
      <div class="sweep" aria-hidden="true" />
      <div class="sweep s2" aria-hidden="true" />
      <div class="wrap">
        <div class="badge"><span class="dot" />{{ t("home.meteor.hero.badge") }}</div>
        <h1>
          {{ heroTitleParts[0] }}<br /><span class="grad">{{ heroTitleParts[1] }}</span>
        </h1>
        <p class="sub">{{ t("home.meteor.hero.supporting") }} {{ t("home.meteor.hero.footnote") }}</p>
        <div class="hero-ctas">
          <router-link :to="isAuthenticated ? dashboardPath : '/register'" class="btn primary">{{ t("home.meteor.hero.startRouting") }} →</router-link>
          <a href="#pipeline" class="btn">{{ t("home.meteor.hero.seeHow") }}</a>
        </div>
      </div>
    </header>

    <!-- ============ 实时观测与供应商：独立区块，避免与 Hero 动画重叠 ============ -->
    <section class="block hero-extras">
      <div class="wrap">
        <!-- 实时观测面板 -->
        <div class="obs reveal">
          <div class="obs-head">
            <span class="rec" />{{ t("home.meteor.observability.title") }}
            <span class="regions">{{ t("home.meteor.observability.regions") }}</span>
          </div>
          <div class="obs-body">
            <div class="obs-stream">
              <div v-for="row in streamRows" :key="row.id" class="obs-row">
                <span class="prov"><ProviderIcon :brand="row.brand" size="15px" />{{ row.name }}</span>
                <span class="bar"><i :style="{ width: `${row.ratio}%` }" /></span>
                <span class="lat"><b>{{ row.latency }}ms</b></span>
              </div>
            </div>
            <div class="obs-side">
              <div class="stat-cell">
                <div class="k">{{ t("home.meteor.metrics.averageLatency") }}</div>
                <div class="v g">{{ metricValues.averageLatency }}</div>
              </div>
              <div class="stat-cell">
                <div class="k">{{ t("home.meteor.metrics.uptime") }}</div>
                <div class="v">{{ metricValues.uptime }}</div>
              </div>
              <div class="stat-cell">
                <div class="k">{{ t("home.meteor.metrics.providers") }}</div>
                <div class="v c">{{ metricValues.providers }}</div>
              </div>
            </div>
          </div>
        </div>

        <!-- 供应商：双向无限跑马灯 -->
        <div class="providers reveal">
          <p v-if="marketplaceError" class="provider-error">{{ t("home.meteor.providers.unavailable") }}</p>
          <div class="lbl">{{ t("home.meteor.providers.heading") }}</div>
          <div class="marquee">
            <div class="marquee-track">
              <span v-for="(p, i) in displayProviderRows" :key="`${p[0]}-${i}`" class="prov-chip">
                <ProviderIcon :brand="p[0]" size="16px" />{{ p[0] }}
              </span>
            </div>
          </div>
          <div class="marquee rev">
            <div class="marquee-track">
              <span v-for="(p, i) in [...displayProviderRows].reverse()" :key="`${p[0]}-r-${i}`" class="prov-chip">
                <ProviderIcon :brand="p[0]" size="16px" />{{ p[0] }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ============ 三大支柱 ============ -->
    <section class="block" id="pillars">
      <div class="wrap">
        <div class="reveal">
          <div class="sec-tag">{{ t("home.meteor.pillars.eyebrow") }}</div>
          <h2 class="sec-title">{{ t("home.meteor.pillars.titlePrimary") }}<br />{{ t("home.meteor.pillars.titleSecondary") }}</h2>
        </div>
        <div class="pillars">
          <div class="pillar reveal">
            <div class="glow" />
            <div class="icon">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#12a7e8" stroke-width="1.8">
                <path d="M4 7h13m0 0-3-3m3 3-3 3M20 17H7m0 0 3 3m-3-3 3-3" />
              </svg>
            </div>
            <div class="num">{{ t("home.meteor.pillars.items.routing.number") }}</div>
            <h3>{{ t("home.meteor.pillars.items.routing.title") }}</h3>
            <p>{{ t("home.meteor.pillars.items.routing.description") }}</p>
          </div>
          <div class="pillar reveal">
            <div class="glow" style="background: radial-gradient(circle, rgba(125, 211, 252, 0.13), transparent 70%)" />
            <div class="icon">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#7dd3fc" stroke-width="1.8">
                <path d="M12 3l8 4v5c0 5-3.5 8-8 9-4.5-1-8-4-8-9V7l8-4z" />
                <path d="M9 12l2 2 4-4" />
              </svg>
            </div>
            <div class="num">{{ t("home.meteor.pillars.items.governance.number") }}</div>
            <h3>{{ t("home.meteor.pillars.items.governance.title") }}</h3>
            <p>{{ t("home.meteor.pillars.items.governance.description") }}</p>
          </div>
          <div class="pillar reveal">
            <div class="glow" style="background: radial-gradient(circle, rgba(251, 191, 36, 0.12), transparent 70%)" />
            <div class="icon">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fbbf24" stroke-width="1.8">
                <path d="M3 12h4l2-6 4 12 2-6h6" />
              </svg>
            </div>
            <div class="num">{{ t("home.meteor.pillars.items.observability.number") }}</div>
            <h3>{{ t("home.meteor.pillars.items.observability.title") }}</h3>
            <p>{{ t("home.meteor.pillars.items.observability.description") }}</p>
          </div>
        </div>

        <div class="stats-band reveal">
          <div v-for="stat in displayStats" :key="stat[1]" class="cell">
            <div class="n">{{ stat[0] }}</div>
            <div class="l">{{ stat[1] }}</div>
          </div>
        </div>
        <p v-if="statsError" class="stats-error">{{ t("home.meteor.stats.unavailable") }}</p>
      </div>
    </section>

    <!-- ============ 全球网络 ============ -->
    <section class="block" id="network">
      <div class="wrap">
        <div class="net-grid">
          <div class="net-copy reveal">
            <div class="sec-tag">{{ t("home.meteor.network.eyebrow") }}</div>
            <h2 class="sec-title">{{ t("home.meteor.network.titlePrimary") }}<br />{{ t("home.meteor.network.titleSecondary") }}</h2>
            <p class="sec-sub">{{ t("home.meteor.network.description") }}</p>
            <div class="net-mini">
              <div class="m">
                <div class="k">{{ t("home.meteor.network.metrics.routedMonthly") }}</div>
                <div class="v">{{ t("home.meteor.network.metrics.routedMonthlyValue") }}</div>
              </div>
              <div class="m">
                <div class="k">{{ t("home.meteor.metrics.providers") }}</div>
                <div class="v">{{ metricValues.providers }}</div>
              </div>
              <div class="m">
                <div class="k">{{ t("home.meteor.metrics.uptime") }}</div>
                <div class="v">{{ metricValues.uptime }}</div>
              </div>
              <div class="m">
                <div class="k">{{ t("home.meteor.metrics.averageLatency") }}</div>
                <div class="v">{{ metricValues.averageLatency }}</div>
              </div>
            </div>
          </div>

          <div class="net-panel reveal">
            <canvas ref="globeCanvas" class="globe-canvas" aria-hidden="true" />
            <div class="globe-hint">{{ t("home.meteor.network.canvasHint") }}</div>
            <div class="net-foot">
              <span><span class="sw" style="background: #12a7e8" />{{ t("home.meteor.network.primary") }}</span>
              <span><span class="sw" style="background: #7dd3fc" />{{ t("home.meteor.network.acceleration") }}</span>
              <span><span class="sw" style="background: #565e6e" />{{ t("home.meteor.network.liveRequests") }}</span>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ============ 请求管线 ============ -->
    <section class="block" id="pipeline">
      <div class="wrap">
        <div class="reveal">
          <div class="sec-tag">{{ t("home.meteor.pipeline.eyebrow") }}</div>
          <h2 class="sec-title">{{ t("home.meteor.pipeline.titlePrimary") }}<br />{{ t("home.meteor.pipeline.titleSecondary") }}</h2>
          <p class="sec-sub">{{ t("home.meteor.pipeline.description") }}</p>
        </div>

        <div class="reveal pipe-margin">
          <div class="pipe-head">
            <span><span class="method">POST</span> <span class="path">{{ requestPath }}</span></span>
            <span class="status">{{ t("home.meteor.pipeline.inFlight") }} · <span class="ms">{{ pipelineMs }}ms</span></span>
          </div>
          <div class="pipe-body">
            <div class="pipe-stages">
              <div class="track-fill" :style="{ width: `${trackPercent}%` }" aria-hidden="true" />
              <div
                v-for="(stage, i) in stages"
                :key="stage.key"
                class="stage"
                :class="{ active: activeStage === i, done: activeStage > i }"
                @click="selectStage(i)"
              >
                <div class="idx">0{{ i + 1 }}</div>
                <div class="nm">{{ t(`home.meteor.pipeline.stages.${stage.key}.name`) }}</div>
                <div class="t">{{ stage.duration }}</div>
              </div>
            </div>
            <div class="stage-detail">
              <div class="big-idx">0{{ activeStage + 1 }}</div>
              <div>
                <h4>{{ t(`home.meteor.pipeline.stages.${stages[activeStage].key}.title`) }}</h4>
                <p>{{ t(`home.meteor.pipeline.stages.${stages[activeStage].key}.description`) }}</p>
                <div v-if="t(`home.meteor.pipeline.stages.${stages[activeStage].key}.chip`)" class="chip">
                  {{ t(`home.meteor.pipeline.stages.${stages[activeStage].key}.chip`) }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ============ 控制面 ============ -->
    <section class="block" id="control">
      <div class="wrap">
        <div class="reveal">
          <div class="sec-tag">{{ t("home.meteor.control.eyebrow") }}</div>
          <h2 class="sec-title">{{ t("home.meteor.control.title") }}</h2>
          <p class="sec-sub">{{ t("home.meteor.control.description") }}</p>
        </div>

        <div class="cp-tabs reveal">
          <button
            v-for="(tab, i) in controlTabs"
            :key="tab.key"
            type="button"
            class="cp-tab"
            :class="{ on: activeControl === i }"
            @click="activeControl = i"
          >
            {{ t(`home.meteor.control.tabs.${tab.key}.tag`) }}
          </button>
        </div>
        <div class="cp-body reveal">
          <div class="cp-left">
            <div class="cp-no">{{ t(`home.meteor.control.tabs.${controlTabs[activeControl].key}.tag`) }}</div>
            <h3>{{ t(`home.meteor.control.tabs.${controlTabs[activeControl].key}.title`) }}</h3>
            <p>{{ t(`home.meteor.control.tabs.${controlTabs[activeControl].key}.description`) }}</p>
            <div class="cp-features">
              <div v-for="feature in 4" :key="feature" class="f">{{ t(`home.meteor.control.tabs.${controlTabs[activeControl].key}.features.${feature - 1}`) }}</div>
            </div>
          </div>
          <div class="cp-right">
            <!-- 复用设计稿的 SVG 可视化，文案经 i18n 注入。 -->
            <svg class="cp-viz" viewBox="0 0 380 280" fill="none" v-html="cpViz" />
          </div>
        </div>

        <!-- 混沌 → 有序 流量图 -->
        <div class="flow-wrap reveal">
          <svg viewBox="0 0 1060 300" fill="none" aria-hidden="true">
            <defs>
              <linearGradient id="fg" x1="0" x2="1">
                <stop offset="0" stop-color="#f87171" stop-opacity="0" />
                <stop offset=".5" stop-color="#7dd3fc" />
                <stop offset="1" stop-color="#12a7e8" />
              </linearGradient>
            </defs>
            <g font-family="monospace" font-size="11">
              <text x="60" y="40" fill="#f87171">{{ t("home.meteor.flow.chaoticTraffic") }}</text>
              <g stroke="rgba(248,113,113,.35)" stroke-width="1.2">
                <path d="M60 70 C 180 60, 240 110, 430 140" />
                <path d="M60 100 C 190 130, 250 90, 430 145" />
                <path d="M60 130 C 170 90, 260 150, 430 150" />
                <path d="M60 160 C 200 200, 240 130, 430 155" />
                <path d="M60 190 C 170 160, 260 210, 430 160" />
                <path d="M60 220 C 190 250, 250 180, 430 165" />
              </g>
              <g fill="#f87171" opacity=".7">
                <circle cx="60" cy="66" r="3" />
                <circle cx="60" cy="96" r="3" />
                <circle cx="60" cy="126" r="3" />
                <circle cx="60" cy="156" r="3" />
                <circle cx="60" cy="186" r="3" />
                <circle cx="60" cy="216" r="3" />
              </g>
            </g>
            <g class="core-pulse">
              <rect x="430" y="105" width="200" height="90" rx="12" fill="rgba(18,167,232,.05)" stroke="rgba(18,167,232,.45)" stroke-width="1.2" />
              <text x="530" y="140" text-anchor="middle" fill="#eef1f6" font-size="13" font-weight="600" font-family="sans-serif">{{ t("home.meteor.flow.platform") }}</text>
              <text x="530" y="160" text-anchor="middle" fill="#8b93a3" font-size="10" font-family="monospace">{{ t("home.meteor.flow.stages") }}</text>
            </g>
            <circle cx="530" cy="150" r="46" fill="none" stroke="rgba(18,167,232,.25)" stroke-width="1" class="ripple" />
            <g font-family="monospace" font-size="11">
              <text x="915" y="40" fill="#12a7e8">{{ t("home.meteor.flow.orderedOutput") }}</text>
              <g stroke="rgba(18,167,232,.5)" stroke-width="1.4">
                <path d="M630 140 C 760 148, 830 150, 890 150" class="flow-arc" />
                <path d="M630 145 C 760 150, 830 152, 890 152" class="flow-arc" />
              </g>
              <circle cx="900" cy="150" r="4" fill="#12a7e8" />
              <g fill="#565e6e">
                <circle cx="930" cy="130" r="2.5" />
                <circle cx="945" cy="150" r="2.5" />
                <circle cx="930" cy="170" r="2.5" />
              </g>
              <text x="915" y="200" fill="#565e6e" font-size="10">{{ t("home.meteor.flow.protocol") }}</text>
            </g>
            <g font-family="monospace" font-size="10.5" fill="#565e6e">
              <text x="70" y="265">{{ t("home.meteor.flow.route") }}</text>
              <text x="460" y="265">{{ t("home.meteor.flow.govern") }}</text>
              <text x="860" y="265">{{ t("home.meteor.flow.observe") }}</text>
            </g>
          </svg>
        </div>
        <p class="sec-sub flow-note reveal">{{ t("home.meteor.flow.description") }}</p>
      </div>
    </section>

    <!-- ============ 页脚 ============ -->
    <footer>
      <div class="wrap">
        <div class="foot-grid">
          <div>
            <router-link to="/home" class="logo foot-logo">
              <span v-if="siteLogo" class="logo-mark"><img :src="siteLogo" alt="" /></span>
              <span>{{ siteName }}</span>
            </router-link>
            <p class="foot-desc">{{ t("home.meteor.footer.description") }}</p>
          </div>
          <div>
            <h5>{{ t("home.meteor.footer.product") }}</h5>
            <a href="#pillars">{{ t("home.meteor.footer.routing") }}</a>
            <a href="#control">{{ t("home.meteor.footer.governance") }}</a>
            <a href="#pipeline">{{ t("home.meteor.footer.observability") }}</a>
            <a href="#control">{{ t("home.meteor.footer.costControl") }}</a>
          </div>
          <div>
            <h5>{{ t("home.meteor.footer.developers") }}</h5>
            <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t("home.meteor.footer.documentation") }}</a>
            <a v-else href="#pipeline">{{ t("home.meteor.footer.documentation") }}</a>
            <a href="#pipeline">{{ t("home.meteor.footer.apiReference") }}</a>
            <a href="#network">{{ t("home.meteor.footer.sdks") }}</a>
            <a href="#network">{{ t("home.meteor.footer.status") }}</a>
          </div>
          <div>
            <h5>{{ t("home.meteor.footer.company") }}</h5>
            <a href="#">{{ t("home.meteor.footer.about") }}</a>
            <a href="#">{{ t("home.meteor.footer.blog") }}</a>
            <a href="#">{{ t("home.meteor.footer.careers") }}</a>
            <a href="#">{{ t("home.meteor.footer.contact") }}</a>
          </div>
        </div>
        <div class="foot-bottom">
          <span>© {{ new Date().getFullYear() }} {{ siteName }}</span>
          <span>{{ t("home.meteor.footer.regions") }}</span>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped>
:global(html) {
  scroll-behavior: smooth;
}
:global(*) {
  box-sizing: border-box;
}
/* 页面级设计令牌：与静态设计稿保持一致的颜色与字体栈。 */
.meteor-page {
  --bg: #121621;
  --bg2: #0d1119;
  --panel: #151a26;
  --panel2: #121623;
  --line: rgba(255, 255, 255, 0.08);
  --line2: rgba(255, 255, 255, 0.16);
  --txt: #eef1f6;
  --dim: #8b93a3;
  --dim2: #565e6e;
  --acc: #12a7e8;
  --acc2: #7dd3fc;
  --amber: #fbbf24;
  --red: #f87171;
  --mono: "SF Mono", "Cascadia Code", Consolas, "JetBrains Mono", monospace;
  --sans: -apple-system, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  min-height: 100vh;
  overflow-x: hidden;
  background: var(--bg);
  color: var(--txt);
  font-family: var(--sans);
  line-height: 1.6;
}
:global(::selection) {
  background: rgba(18, 167, 232, 0.25);
}

/* ---------- 极光漂移光斑 ---------- */
.aurora {
  position: fixed;
  inset: -15%;
  z-index: 0;
  pointer-events: none;
}
.aurora i {
  position: absolute;
  border-radius: 50%;
  filter: blur(70px);
}
.aurora .b1 {
  width: 46vw;
  height: 46vw;
  top: -8%;
  left: 6%;
  background: radial-gradient(circle, rgba(18, 167, 232, 0.15), transparent 65%);
  animation: auroraDrift 22s ease-in-out infinite;
}
.aurora .b2 {
  width: 42vw;
  height: 42vw;
  top: 0%;
  right: 2%;
  background: radial-gradient(circle, rgba(125, 211, 252, 0.11), transparent 65%);
  animation: auroraDrift 27s ease-in-out infinite reverse;
}
.aurora .b3 {
  width: 38vw;
  height: 38vw;
  top: 32%;
  left: 34%;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.08), transparent 65%);
  animation: auroraDrift 32s ease-in-out infinite;
}
@keyframes auroraDrift {
  0% {
    transform: translate(0) scale(1);
  }
  50% {
    transform: translate(4%, -5%) scale(1.08);
  }
  100% {
    transform: translate(-4%, 4%) scale(0.96);
  }
}

/* ---------- 流动网格 ---------- */
.grid-fx {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 0;
  opacity: 0.5;
  background-image: linear-gradient(rgba(255, 255, 255, 0.03) 1px, transparent 1px), linear-gradient(90deg, rgba(255, 255, 255, 0.03) 1px, transparent 1px);
  background-size: 72px 72px;
  -webkit-mask-image: radial-gradient(ellipse 90% 60% at 50% 0%, #000 25%, transparent 70%);
  mask-image: radial-gradient(ellipse 90% 60% at 50% 0%, #000 25%, transparent 70%);
  animation: gridFlow 9s linear infinite;
}
@keyframes gridFlow {
  to {
    background-position: 72px 72px;
  }
}

.wrap {
  position: relative;
  z-index: 1;
  max-width: 1280px;
  margin: 0 auto;
  padding: 0 24px;
}

/* ---------- 导航 ---------- */
nav {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 100;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  background: rgba(13, 17, 25, 0.6);
  border-bottom: 1px solid transparent;
  transition: border-color 0.3s;
}
nav.scrolled {
  border-bottom-color: var(--line);
}
.nav-in {
  max-width: 1120px;
  margin: 0 auto;
  padding: 0 24px;
  height: 64px;
  display: flex;
  align-items: center;
  gap: 36px;
}
.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  font-weight: 700;
  font-size: 17px;
  letter-spacing: 0.02em;
  cursor: pointer;
  position: relative;
  color: var(--txt);
  text-decoration: none;
}
/* 站点 Logo：无底色，直接展示图标本身；固定像素上限，任何尺寸的图片都不会被放大。 */
.logo-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  flex: none;
  overflow: hidden;
}
.logo-mark img {
  max-width: 24px;
  max-height: 24px;
  width: auto;
  height: auto;
  object-fit: contain;
  border-radius: 6px;
}
.nav-links {
  display: flex;
  gap: 28px;
  font-size: 13.5px;
  color: var(--dim);
}
.nav-links a {
  color: inherit;
  text-decoration: none;
  transition: color 0.2s;
}
.nav-links a:hover {
  color: var(--txt);
}
.nav-cta {
  margin-left: auto;
  display: flex;
  gap: 12px;
  align-items: center;
}
.icon-btn {
  width: 32px;
  height: 32px;
  padding: 0;
  display: grid;
  place-items: center;
  border: 0;
  background: transparent;
  color: var(--dim);
  cursor: pointer;
}
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 9px 18px;
  border-radius: 8px;
  font-size: 13.5px;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid var(--line2);
  background: transparent;
  color: var(--txt);
  text-decoration: none;
  transition: all 0.25s;
  position: relative;
  overflow: hidden;
}
.btn:hover {
  border-color: rgba(255, 255, 255, 0.32);
  background: rgba(255, 255, 255, 0.05);
}
.btn.primary {
  background: var(--acc);
  color: #fff;
  border-color: var(--acc);
  box-shadow: 0 0 0 rgba(18, 167, 232, 0);
}
.btn.primary:hover {
  background: #3fb9f0;
  transform: translateY(-1px);
  box-shadow: 0 8px 28px -8px rgba(18, 167, 232, 0.5), 0 0 24px rgba(18, 167, 232, 0.25);
}
.btn.sm {
  padding: 7px 14px;
  font-size: 12.5px;
}

/* ---------- Hero ---------- */
.hero {
  padding: 150px 0 60px;
  text-align: center;
  position: relative;
  overflow: hidden;
  cursor: default;
}
.hero .wrap {
  position: relative;
  z-index: 2;
}
/* 贯穿 Hero 的电光扫掠光束 */
.sweep {
  position: absolute;
  left: 0;
  width: 36vw;
  height: 3px;
  top: 38%;
  pointer-events: none;
  background: linear-gradient(90deg, transparent, rgba(18, 167, 232, 0.85) 40%, rgba(18, 167, 232, 0.5) 70%, transparent);
  filter: blur(1px);
  opacity: 0;
  animation: electricSweep 7.5s ease-in-out infinite;
}
.sweep.s2 {
  top: 52%;
  animation-delay: 3.4s;
  height: 2px;
  opacity: 0;
}
@keyframes electricSweep {
  0%,
  14% {
    opacity: 0;
    transform: translate(0) scale(0.7);
  }
  24% {
    opacity: 0.9;
  }
  55% {
    opacity: 1;
  }
  76%,
  100% {
    opacity: 0;
    transform: translate(115vw) scale(1.05);
  }
}
.badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 5px 14px 5px 10px;
  border-radius: 99px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.03);
  font-size: 13px;
  color: var(--dim);
  font-family: var(--mono);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  animation: heroFadeIn 0.9s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}
.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--acc);
  box-shadow: 0 0 8px var(--acc);
  animation: pulse 2s infinite;
}
.rec {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--red);
  box-shadow: 0 0 8px var(--red);
  animation: pulse 1.4s infinite;
  flex: none;
}
@keyframes pulse {
  50% {
    opacity: 0.35;
  }
}
@keyframes heroFadeIn {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.hero h1 {
  font-size: clamp(44px, 6.8vw, 88px);
  font-weight: 700;
  letter-spacing: -0.03em;
  line-height: 1.05;
  margin: 26px 0 20px;
  animation: heroFadeIn 0.9s 0.08s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}
.hero h1 .grad {
  background: linear-gradient(90deg, var(--acc) 0%, var(--acc2) 30%, #a5f3fc 50%, var(--acc2) 70%, var(--acc) 100%);
  background-size: 200% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: gradFlow 6s linear infinite;
}
@keyframes gradFlow {
  to {
    background-position: 200% 0;
  }
}
.hero p.sub {
  font-size: clamp(16px, 2vw, 21px);
  color: var(--dim);
  max-width: 640px;
  margin: 0 auto 36px;
  animation: heroFadeIn 0.9s 0.16s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}
.hero-ctas {
  display: flex;
  gap: 14px;
  justify-content: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  animation: heroFadeIn 0.9s 0.24s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}

/* Hero 下方的观测面板与供应商区块：独立于动画区域 */
.hero-extras {
  padding: 36px 0 0;
  position: relative;
  z-index: 1;
}

/* ---------- 观测面板：玻璃光泽 ---------- */
.obs {
  border: 1px solid var(--line);
  border-radius: 14px;
  background: rgba(21, 26, 38, 0.72);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  overflow: hidden;
  text-align: left;
  position: relative;
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.5);
}
.obs::after {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  width: 45%;
  height: 100%;
  pointer-events: none;
  background: linear-gradient(105deg, transparent, rgba(255, 255, 255, 0.06), transparent);
  transform: translate(-100%);
  animation: glassSheen 7s ease-in-out infinite;
}
@keyframes glassSheen {
  0%,
  18% {
    transform: translate(-100%);
  }
  68%,
  100% {
    transform: translate(360%);
  }
}
.obs-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 13px 18px;
  border-bottom: 1px solid var(--line);
  font-family: var(--mono);
  font-size: 11px;
  color: var(--dim2);
  letter-spacing: 0.12em;
  text-transform: uppercase;
}
.obs-head .regions {
  margin-left: auto;
  letter-spacing: 0.03em;
}
.obs-body {
  display: grid;
  grid-template-columns: 1.6fr 1fr;
  min-height: 230px;
}
.obs-stream {
  padding: 18px;
  border-right: 1px solid var(--line);
  position: relative;
  overflow: hidden;
}
.obs-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 12px;
  border-radius: 8px;
  font-family: var(--mono);
  font-size: 14px;
  margin-bottom: 6px;
  background: rgba(255, 255, 255, 0.02);
  animation: slideIn 0.5s cubic-bezier(0.2, 0.8, 0.2, 1);
}
@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.obs-row .prov {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 150px;
  color: var(--txt);
}
.obs-row .lat {
  margin-left: auto;
  color: var(--dim);
}
.obs-row .lat b {
  color: var(--acc);
  font-weight: 600;
}
.obs-row .bar {
  position: relative;
  flex: 1;
  max-width: 120px;
  height: 4px;
  border-radius: 2px;
  background: rgba(255, 255, 255, 0.06);
  overflow: hidden;
}
.obs-row .bar i {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  border-radius: 2px;
  background: linear-gradient(90deg, var(--acc2), var(--acc));
  animation: grow 0.8s ease;
}
@keyframes grow {
  from {
    transform: scaleX(0);
    transform-origin: left;
  }
}
.obs-side {
  padding: 18px;
  display: flex;
  flex-direction: column;
}
.stat-cell {
  padding: 14px 6px;
  border-bottom: 1px solid var(--line);
}
.stat-cell:last-child {
  border-bottom: none;
}
.stat-cell .k {
  font-family: var(--mono);
  font-size: 10.5px;
  color: var(--dim2);
  letter-spacing: 0.1em;
  text-transform: uppercase;
}
.stat-cell .v {
  font-size: 28px;
  font-weight: 700;
  font-family: var(--mono);
  letter-spacing: -0.02em;
  margin-top: 2px;
}
.stat-cell .v.g {
  color: var(--acc);
  text-shadow: 0 0 18px rgba(18, 167, 232, 0.4);
}
.stat-cell .v.c {
  color: var(--acc2);
  text-shadow: 0 0 18px rgba(125, 211, 252, 0.35);
}

/* ---------- 供应商：双向跑马灯 ---------- */
.providers {
  margin: 44px 0 20px;
}
.provider-error {
  margin: 0 0 12px;
  color: var(--dim2);
  font-size: 11px;
  text-align: center;
}
.providers .lbl {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--dim2);
  letter-spacing: 0.14em;
  text-transform: uppercase;
  text-align: center;
  margin-bottom: 22px;
}
.marquee {
  overflow: hidden;
  padding: 4px 0;
  -webkit-mask-image: linear-gradient(90deg, transparent, #000 12%, #000 88%, transparent);
  mask-image: linear-gradient(90deg, transparent, #000 12%, #000 88%, transparent);
}
.marquee-track {
  display: flex;
  gap: 10px;
  width: max-content;
  animation: marqueeLTR 36s linear infinite;
}
.marquee.rev .marquee-track {
  animation: marqueeRTL 42s linear infinite;
}
.marquee:hover .marquee-track {
  animation-play-state: paused;
}
@keyframes marqueeLTR {
  to {
    transform: translate(-50%);
  }
}
@keyframes marqueeRTL {
  from {
    transform: translate(-50%);
  }
  to {
    transform: translate(0);
  }
}
.marquee + .marquee {
  margin-top: 10px;
}
.prov-chip {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  padding: 10px 19px;
  border-radius: 99px;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.02);
  font-size: 14.5px;
  color: var(--dim);
  transition: all 0.25s;
  cursor: default;
  animation: provFloat 5s ease-in-out infinite;
  white-space: nowrap;
}
.marquee.rev .prov-chip {
  animation-delay: -2.5s;
}
.prov-chip:hover {
  color: var(--txt);
  border-color: rgba(18, 167, 232, 0.35);
  background: rgba(18, 167, 232, 0.06);
  box-shadow: 0 0 20px rgba(18, 167, 232, 0.12);
}
@keyframes provFloat {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-4px);
  }
}

/* ---------- 区块骨架 ---------- */
section.block {
  padding: 110px 0;
  position: relative;
  z-index: 1;
}
.sec-tag {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--acc);
  letter-spacing: 0.16em;
  text-transform: uppercase;
  margin-bottom: 16px;
}
.sec-title {
  font-size: clamp(32px, 4.5vw, 54px);
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.15;
  margin: 0 0 16px;
}
.sec-sub {
  color: var(--dim);
  font-size: 17px;
  max-width: 680px;
  line-height: 1.7;
}

/* ---------- 三大支柱 ---------- */
.pillars {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-top: 48px;
}
.pillar {
  border: 1px solid var(--line);
  border-radius: 14px;
  padding: 28px;
  background: var(--panel);
  position: relative;
  overflow: hidden;
  transition: all 0.3s;
}
.pillar:hover {
  border-color: rgba(18, 167, 232, 0.35);
  transform: translateY(-3px);
  box-shadow: 0 20px 50px -20px rgba(0, 0, 0, 0.6), 0 0 30px rgba(18, 167, 232, 0.06);
}
.pillar .num {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--dim2);
  letter-spacing: 0.14em;
  margin: 18px 0 0;
}
.pillar h3 {
  font-size: 19px;
  margin: 10px 0;
  letter-spacing: -0.01em;
}
.pillar p {
  font-size: 14.5px;
  color: var(--dim);
  line-height: 1.7;
  margin: 0;
}
.pillar .icon {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--line);
  background: rgba(255, 255, 255, 0.03);
}
.pillar .glow {
  position: absolute;
  top: -60px;
  right: -60px;
  width: 160px;
  height: 160px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(18, 167, 232, 0.14), transparent 70%);
  opacity: 0;
  transition: opacity 0.4s;
  pointer-events: none;
}
.pillar:hover .glow {
  opacity: 1;
}
/* 悬停时掠过卡片的斜向光泽 */
.pillar::after {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  width: 50%;
  height: 100%;
  pointer-events: none;
  background: linear-gradient(105deg, transparent, rgba(255, 255, 255, 0.04), transparent);
  transform: translate(-100%) skewX(-8deg);
}
.pillar:hover::after {
  animation: glassSheen 1.2s ease;
}

/* ---------- 统计带 ---------- */
.stats-band {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 0;
  margin-top: 56px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--panel);
  overflow: hidden;
}
.stats-band .cell {
  padding: 30px 24px;
  border-right: 1px solid var(--line);
  text-align: left;
  position: relative;
  overflow: hidden;
}
.stats-band .cell:last-child {
  border-right: none;
}
.stats-band .n {
  font-family: var(--mono);
  font-size: 46px;
  font-weight: 700;
  letter-spacing: -0.03em;
  background: linear-gradient(120deg, #fff 20%, #7dd3fc 50%, #fff 80%);
  background-size: 220% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: gradFlow 5s linear infinite;
}
.stats-band .l {
  font-size: 14px;
  color: var(--dim);
  margin-top: 6px;
}
.stats-error {
  margin: 12px 0 0;
  color: var(--dim2);
  font-size: 11px;
}

/* ---------- 全球网络 ---------- */
#network {
  background: linear-gradient(180deg, transparent, rgba(255, 255, 255, 0.012), transparent);
}
.net-grid {
  display: grid;
  grid-template-columns: 1fr 1.2fr;
  gap: 60px;
  align-items: center;
}
.net-mini {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-top: 28px;
}
.net-mini .m {
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 16px;
  background: rgba(255, 255, 255, 0.02);
  transition: border-color 0.3s;
}
.net-mini .m:hover {
  border-color: rgba(125, 211, 252, 0.3);
}
.net-mini .m .k {
  font-family: var(--mono);
  font-size: 10.5px;
  color: var(--dim2);
  text-transform: uppercase;
  letter-spacing: 0.1em;
}
.net-mini .m .v {
  font-size: 23px;
  font-weight: 700;
  margin-top: 4px;
  font-family: var(--mono);
}
/* 无边框效果：地球直接悬浮于页面背景之上，不做卡片式裁剪。 */
.net-panel {
  padding: 0;
  position: relative;
}
.globe-canvas {
  display: block;
  width: 100%;
  height: 480px;
  cursor: grab;
  touch-action: pan-y;
  position: relative;
  z-index: 1;
}
.globe-canvas:active {
  cursor: grabbing;
}
.globe-hint {
  position: absolute;
  bottom: 16px;
  right: 16px;
  font-family: var(--mono);
  font-size: 10.5px;
  color: var(--dim2);
  letter-spacing: 0.08em;
  pointer-events: none;
  z-index: 2;
}
.net-foot {
  display: flex;
  gap: 20px;
  padding: 14px 18px 0;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--dim);
  flex-wrap: wrap;
  position: relative;
  z-index: 2;
}
.net-foot span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.net-foot .sw {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}

/* ---------- 请求管线 ---------- */
.pipe-margin {
  margin-top: 48px;
}
.pipe-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
  border: 1px solid var(--line);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.02);
  padding: 13px 20px;
  margin-bottom: 12px;
  font-family: var(--mono);
  font-size: 13.5px;
}
.pipe-head .method {
  color: var(--acc);
  font-weight: 600;
}
.pipe-head .path {
  color: var(--dim);
}
.pipe-head .status {
  color: var(--dim);
}
.pipe-head .ms {
  color: var(--txt);
}
.pipe-body {
  border: 1px solid var(--line);
  border-radius: 20px;
  background: var(--panel);
  padding: 14px;
}
/* 阶段轨道：六段同处一条连续轨道，读作一个整体进程而非独立卡片。 */
.pipe-stages {
  position: relative;
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 6px;
  padding: 8px 10px 20px;
  border: 1px solid var(--line);
  border-radius: 18px;
  background: var(--panel2);
}
.stage {
  position: relative;
  padding: 12px 14px 16px;
  border: 0;
  border-radius: 12px;
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background 0.3s;
}
.stage:hover {
  background: rgba(255, 255, 255, 0.03);
}
.stage.active {
  background: rgba(18, 167, 232, 0.08);
}
.stage.done .idx {
  color: var(--acc);
}
.stage .idx {
  font-family: var(--mono);
  font-size: 10px;
  color: var(--dim2);
  letter-spacing: 0.1em;
  transition: color 0.3s;
}
.stage .nm {
  font-size: 14.5px;
  font-weight: 600;
  margin-top: 8px;
}
.stage .t {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--acc);
  margin-top: 4px;
  opacity: 0.85;
}
/* 贯穿整条轨道的连续进度线：六个阶段各占 1/6 宽度，随请求推进从左向右整体流动。 */
.track-fill {
  position: absolute;
  left: 10px;
  bottom: 8px;
  width: 0;
  max-width: calc(100% - 20px);
  height: 3px;
  border-radius: 99px;
  background: linear-gradient(90deg, var(--acc), var(--acc2));
  transition: width 0.15s linear;
  pointer-events: none;
}
.stage-detail {
  margin-top: 12px;
  padding: 22px 24px;
  border: 1px solid var(--line);
  border-radius: 16px;
  background: var(--panel2);
  min-height: 110px;
  display: flex;
  gap: 20px;
  align-items: flex-start;
}
.stage-detail .big-idx {
  font-family: var(--mono);
  font-size: 13px;
  color: var(--dim2);
  padding-top: 3px;
  white-space: nowrap;
}
.stage-detail h4 {
  font-size: 19px;
  margin: 0 0 8px;
}
.stage-detail p {
  font-size: 15px;
  color: var(--dim);
  max-width: 680px;
  margin: 0;
}
.stage-detail .chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-top: 14px;
  padding: 6px 14px;
  border-radius: 99px;
  border: 1px solid rgba(18, 167, 232, 0.35);
  background: rgba(18, 167, 232, 0.07);
  font-family: var(--mono);
  font-size: 12px;
  color: var(--acc);
}

/* ---------- 控制面 ---------- */
#control {
  background: linear-gradient(180deg, transparent, rgba(255, 255, 255, 0.012), transparent);
}
.cp-tabs {
  display: flex;
  gap: 8px;
  margin-top: 44px;
  flex-wrap: wrap;
}
.cp-tab {
  padding: 8px 16px;
  border-radius: 8px;
  border: 1px solid var(--line);
  background: transparent;
  color: var(--dim);
  font-size: 13px;
  font-family: var(--mono);
  letter-spacing: 0.06em;
  cursor: pointer;
  transition: all 0.25s;
  text-transform: uppercase;
}
.cp-tab:hover {
  color: var(--txt);
  border-color: var(--line2);
}
.cp-tab.on {
  color: var(--acc);
  border-color: rgba(18, 167, 232, 0.4);
  background: rgba(18, 167, 232, 0.06);
  box-shadow: 0 0 18px rgba(18, 167, 232, 0.1);
}
.cp-body {
  margin-top: 22px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--panel);
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: 300px;
  overflow: hidden;
}
.cp-left {
  padding: 36px;
  border-right: 1px solid var(--line);
}
.cp-left .cp-no {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--acc);
  letter-spacing: 0.14em;
  margin-bottom: 14px;
}
.cp-left h3 {
  font-size: 30px;
  letter-spacing: -0.02em;
  margin: 0 0 12px;
}
.cp-left p {
  color: var(--dim);
  font-size: 15px;
  margin: 0;
}
.cp-features {
  margin-top: 24px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.cp-features .f {
  display: flex;
  gap: 10px;
  align-items: center;
  font-size: 14px;
  color: var(--dim);
}
/* 特性列表前置的对勾徽标 */
.cp-features .f::before {
  content: "";
  width: 14px;
  height: 14px;
  border-radius: 4px;
  flex: none;
  background: rgba(18, 167, 232, 0.12);
  border: 1px solid rgba(18, 167, 232, 0.3);
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%2312a7e8' stroke-width='3'%3E%3Cpath d='M5 13l4 4L19 7'/%3E%3C/svg%3E");
  background-size: 9px;
  background-position: center;
  background-repeat: no-repeat;
}
.cp-right {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 30px;
  background: radial-gradient(ellipse 80% 60% at 60% 40%, rgba(125, 211, 252, 0.05), transparent 70%);
}
.cp-viz {
  width: 100%;
  max-width: 380px;
}
.viz-line {
  stroke-dasharray: 4 6;
  animation: dashmove 1.6s linear infinite;
}

/* ---------- 混沌 → 有序 流量图 ---------- */
.flow-wrap {
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--panel);
  margin-top: 48px;
  overflow: hidden;
  position: relative;
}
.flow-wrap svg {
  display: block;
  width: 100%;
  height: auto;
}
.core-pulse {
  transform-box: fill-box;
  transform-origin: center;
  animation: corePulse 3.2s ease-in-out infinite;
}
@keyframes corePulse {
  0%,
  100% {
    filter: drop-shadow(0 0 6px rgba(18, 167, 232, 0.25));
  }
  50% {
    filter: drop-shadow(0 0 20px rgba(18, 167, 232, 0.55));
  }
}
.flow-arc {
  stroke-dasharray: 6 8;
  animation: dashmove 1.2s linear infinite;
}
@keyframes dashmove {
  to {
    stroke-dashoffset: -28;
  }
}
.ripple {
  transform-box: fill-box;
  transform-origin: center;
  animation: ripple 2.8s ease-out infinite;
}
@keyframes ripple {
  0% {
    transform: scale(0.4);
    opacity: 0.9;
  }
  100% {
    transform: scale(2.6);
    opacity: 0;
  }
}
.flow-note {
  margin-top: 22px;
  font-size: 14px;
}

/* ---------- 页脚 ---------- */
footer {
  border-top: 1px solid var(--line);
  padding: 60px 0 40px;
  margin-top: 40px;
  position: relative;
  z-index: 1;
}
.foot-grid {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr;
  gap: 40px;
}
.foot-grid h5 {
  font-size: 12px;
  color: var(--dim2);
  letter-spacing: 0.1em;
  text-transform: uppercase;
  margin: 0 0 16px;
  font-family: var(--mono);
}
.foot-grid a {
  display: block;
  color: var(--dim);
  text-decoration: none;
  font-size: 14px;
  margin-bottom: 10px;
  transition: color 0.2s;
}
.foot-grid a:hover {
  color: var(--txt);
}
.foot-logo {
  margin-bottom: 14px;
}
.foot-desc {
  color: var(--dim);
  font-size: 13px;
  max-width: 260px;
  margin: 0;
}
.foot-bottom {
  margin-top: 50px;
  padding-top: 24px;
  border-top: 1px solid var(--line);
  display: flex;
  justify-content: space-between;
  font-size: 12.5px;
  color: var(--dim2);
  font-family: var(--mono);
  flex-wrap: wrap;
  gap: 10px;
}

/* ---------- 滚动显现 ---------- */
.reveal {
  opacity: 0;
  transform: translateY(2rem);
  transition: opacity 0.8s cubic-bezier(0.2, 0.8, 0.2, 1), transform 0.8s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.reveal.in {
  opacity: 1;
  transform: none;
}

/* ---------- WebGL 降级 ---------- */
.no-webgl .globe-canvas {
  display: none;
}
.no-webgl .net-panel {
  background: radial-gradient(ellipse 80% 70% at 60% 40%, rgba(18, 167, 232, 0.08), rgba(21, 26, 38, 1) 70%);
}
.no-webgl .net-panel::before {
  content: "";
  display: block;
  height: 430px;
}

@media (max-width: 900px) {
  .obs-body {
    grid-template-columns: 1fr;
  }
  .obs-stream {
    border-right: none;
    border-bottom: 1px solid var(--line);
  }
  .pillars {
    grid-template-columns: 1fr;
  }
  .stats-band {
    grid-template-columns: 1fr 1fr;
  }
  .stats-band .cell:nth-child(2) {
    border-right: none;
  }
  .stats-band .cell:nth-child(-n + 2) {
    border-bottom: 1px solid var(--line);
  }
  .net-grid {
    grid-template-columns: 1fr;
  }
  .pipe-stages {
    grid-template-columns: repeat(3, 1fr);
  }
  .cp-body {
    grid-template-columns: 1fr;
  }
  .cp-left {
    border-right: none;
    border-bottom: 1px solid var(--line);
  }
  .foot-grid {
    grid-template-columns: 1fr 1fr;
  }
  .nav-links {
    display: none;
  }
}
@media (max-width: 560px) {
  .icon-btn,
  :deep(.locale-switcher) {
    display: none;
  }
  .globe-canvas,
  .no-webgl .net-panel::before {
    height: 330px;
  }
  .globe-hint {
    bottom: 52px;
  }
}

/* ---------- 浅色主题：跟随 useTheme 的 html.dark 状态 ---------- */
html:not(.dark) .meteor-page {
  --bg: #f5f7fa;
  --bg2: #eef1f6;
  --panel: #ffffff;
  --panel2: #f8fafc;
  --line: rgba(15, 23, 42, 0.12);
  --line2: rgba(15, 23, 42, 0.22);
  --txt: #101828;
  --dim: #475467;
  --dim2: #667085;
  --acc: #0c8ecb;
  --acc2: #0284c7;
  --amber: #d97706;
  --red: #dc2626;
  background: var(--bg);
  color: var(--txt);
}
html:not(.dark) .meteor-page nav {
  background: rgba(255, 255, 255, 0.72);
}
html:not(.dark) .meteor-page nav.scrolled {
  background: rgba(255, 255, 255, 0.92);
}
html:not(.dark) .meteor-page .obs,
html:not(.dark) .meteor-page .pipe-body,
html:not(.dark) .meteor-page .cp-body,
html:not(.dark) .meteor-page .flow-wrap,
html:not(.dark) .meteor-page .pillar,
html:not(.dark) .meteor-page .stats-band {
  background: rgba(255, 255, 255, 0.86);
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.1);
}
html:not(.dark) .meteor-page .grid-fx {
  opacity: 0.25;
  background-image: linear-gradient(rgba(15, 23, 42, 0.06) 1px, transparent 1px), linear-gradient(90deg, rgba(15, 23, 42, 0.06) 1px, transparent 1px);
}
html:not(.dark) .meteor-page .btn:hover {
  border-color: rgba(15, 23, 42, 0.32);
  background: rgba(15, 23, 42, 0.05);
}
html:not(.dark) .meteor-page .stats-band .n {
  background: linear-gradient(120deg, #101828 20%, #0284c7 50%, #101828 80%);
  background-size: 220% 100%;
  -webkit-background-clip: text;
  background-clip: text;
}
</style>
