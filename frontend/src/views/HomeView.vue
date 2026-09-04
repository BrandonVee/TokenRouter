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
import * as THREE from "three";

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
const userInitial = computed(() => authStore.user?.email?.charAt(0).toUpperCase() || "");
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

const marketplaceGroups = ref<MarketplaceGroup[]>([]);
const marketplaceStats = ref<MarketplaceStats | null>(null);
const marketplaceLoading = ref(true);
const marketplaceError = ref(false);
const statsError = ref(false);
const totalModelCount = computed(() => marketplaceGroups.value.reduce((total, group) => total + group.models.length, 0));
const providerRowsFromApi = computed(() => {
  const rows = marketplaceGroups.value
    .filter((group) => group.models.length > 0)
    .slice(0, 14)
    .map((group) => {
      const label = group.display_brand?.trim() || group.name.trim() || group.platform;
      return [label, providerColor(label), label.slice(0, 2).toUpperCase()] as [string, string, string];
    });
  return rows.length ? rows : providers;
});
const displayProviderRows = computed(() => [...providerRowsFromApi.value, ...providerRowsFromApi.value]);
const statValue = (value: number | undefined, fallback: string) => (value == null ? fallback : new Intl.NumberFormat(isZh.value ? "zh-CN" : "en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value));
const displayStats = computed(() => [
  [statValue(marketplaceStats.value?.today_tokens, "..."), t("home.meteor.stats.tokensToday")],
  [statValue(marketplaceStats.value?.total_tokens, "..."), t("home.meteor.stats.tokensRouted")],
  [statValue(marketplaceStats.value?.total_users, "..."), t("home.meteor.stats.activeUsers")],
  [marketplaceLoading.value ? "..." : new Intl.NumberFormat(isZh.value ? "zh-CN" : "en-US").format(totalModelCount.value), t("home.meteor.stats.supportedModels")],
]);
const requestPath = computed(() => t("home.meteor.pipeline.requestPath"));
const visualProviders = computed(() => ({
  openai: t("home.meteor.control.visual.providers.openai"),
  gemini: t("home.meteor.control.visual.providers.gemini"),
  grok: t("home.meteor.control.visual.providers.grok"),
}));
const networkNodes = computed(() => ({
  california: t("home.meteor.network.nodes.california"),
  tokyo: t("home.meteor.network.nodes.tokyo"),
  london: t("home.meteor.network.nodes.london"),
}));
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

const providers: Array<[string, string, string]> = [
  ["OpenAI", "#10a37f", "OA"],
  ["Anthropic", "#d97757", "AN"],
  ["Google", "#4796e3", "GO"],
  ["xAI", "#94a3b8", "XA"],
  ["Meta", "#0668e1", "ME"],
  ["Mistral", "#fa520f", "MI"],
  ["Cohere", "#39594d", "CO"],
  ["DeepSeek", "#4d6bfe", "DS"],
  ["Qwen", "#615ced", "QW"],
  ["Moonshot", "#a855f7", "MO"],
  ["Baidu", "#2932e1", "BD"],
  ["Gemini", "#8ab4f8", "GE"],
];
const streamRows = ref<Array<[string, string, string, number]>>([
  ["OpenAI", "#10a37f", "14ms", 76],
  ["Anthropic", "#d97757", "21ms", 58],
  ["Google", "#4796e3", "18ms", 68],
  ["xAI", "#94a3b8", "33ms", 44],
]);
const pillars = [
  { key: "routing", icon: "arrowsUpDown", tone: "green" },
  { key: "governance", icon: "shield", tone: "blue" },
  { key: "observability", icon: "trendingUp", tone: "amber" },
] as const;
const stages = [
  { key: "ingress", duration: "2ms" },
  { key: "auth", duration: "3ms" },
  { key: "policy", duration: "4ms" },
  { key: "routing", duration: "4ms" },
  { key: "upstream", duration: "38ms" },
  { key: "stream", duration: "—" },
];
const controlTabs = [
  { key: "route", type: "route" },
  { key: "govern", type: "govern" },
  { key: "observe", type: "observe" },
];
const activeStage = ref(0);
const activeControl = ref(0);
const progress = ref(0);
const scrolled = ref(false);
const heroCanvas = ref<HTMLCanvasElement | null>(null);
const globeCanvas = ref<HTMLCanvasElement | null>(null);
let timer: number | undefined;
let streamTimer: number | undefined;
let visualCleanup: (() => void) | undefined;
let globeCleanup: (() => void) | undefined;
let revealObserver: IntersectionObserver | undefined;
const onScroll = () => {
  scrolled.value = window.scrollY > 10;
};
const selectStage = (index: number) => {
  activeStage.value = index;
  progress.value = 0;
};

// 使用轻量 Canvas 复刻静态设计稿中的粒子和网络动效，避免引入额外运行时依赖。
function setupVisualEffects(): () => void {
  const hero = heroCanvas.value;
  const globe = globeCanvas.value;
  if (!hero || !globe) return () => undefined;
  const heroContext = hero.getContext("2d");
  if (!heroContext) return () => undefined;
  setupThreeGlobe(globe, {
    california: t("home.meteor.network.labels.california"),
    tokyo: t("home.meteor.network.labels.tokyo"),
    frankfurt: t("home.meteor.network.labels.frankfurt"),
    singapore: t("home.meteor.network.labels.singapore"),
    sydney: t("home.meteor.network.labels.sydney"),
    virginia: t("home.meteor.network.labels.virginia"),
  });
  const particles = Array.from({ length: 120 }, (_, index) => ({
    x: (index * 83) % 1000,
    y: (index * 137) % 620,
    r: 0.5 + (index % 4) * 0.35,
    speed: 0.08 + (index % 5) * 0.025,
  }));
  let frame = 0;
  let width = 0;
  let height = 0;
  const resize = () => {
    const heroRect = hero.getBoundingClientRect();
    const ratio = window.devicePixelRatio || 1;
    hero.width = Math.max(1, Math.floor(heroRect.width * ratio));
    hero.height = Math.max(1, Math.floor(heroRect.height * ratio));
    width = heroRect.width;
    height = heroRect.height;
    heroContext.setTransform(ratio, 0, 0, ratio, 0, 0);
  };
  const draw = () => {
    const t = frame * 0.012;
    heroContext.clearRect(0, 0, width, height);
    const glow = heroContext.createRadialGradient(width * 0.5, height * 0.36, 0, width * 0.5, height * 0.36, width * 0.52);
    glow.addColorStop(0, "rgba(16,185,129,.11)");
    glow.addColorStop(0.48, "rgba(125,211,252,.035)");
    glow.addColorStop(1, "rgba(16,185,129,0)");
    heroContext.fillStyle = glow;
    heroContext.fillRect(0, 0, width, height);
    for (const particle of particles) {
      const x = (particle.x + t * particle.speed * 90) % Math.max(width, 1);
      const y = (particle.y + Math.sin(t + particle.x) * 10) % Math.max(height, 1);
      heroContext.fillStyle = `rgba(125,211,252,${0.18 + (particle.r / 4) * 0.45})`;
      heroContext.beginPath();
      heroContext.arc(x, y, particle.r, 0, Math.PI * 2);
      heroContext.fill();
    }
    frame += 1;
    raf = window.requestAnimationFrame(draw);
  };
  resize();
  window.addEventListener("resize", resize, { passive: true });
  let raf = 0;
  draw();
  return () => {
    window.removeEventListener("resize", resize);
    window.cancelAnimationFrame(raf);
    globeCleanup?.();
  };
}

// 复刻 index.html 的 Three.js 地球：点阵球体、城市节点、数据弧线和可拖拽旋转。
function setupThreeGlobe(canvas: HTMLCanvasElement, labels: Record<string, string>): void {
  if (!canvas.isConnected) return;
  canvas.parentElement?.classList.add("webgl-ready");
  let renderer: any;
  try {
    renderer = new THREE.WebGLRenderer({ canvas, alpha: false, antialias: true });
  } catch {
    return;
  }
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  const updateRendererTheme = () => renderer.setClearColor(document.documentElement.classList.contains("dark") ? 0x10141d : 0xf5f7fa, 1);
  updateRendererTheme();
  const themeObserver = new MutationObserver(updateRendererTheme);
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(45, 1, 0.1, 100);
  camera.position.set(0, 0, 4.6);
  const globe = new THREE.Group();
  scene.add(globe);
  const radius = 1.5;
  const starPositions = new Float32Array(900 * 3);
  for (let i = 0; i < 900; i += 1) {
    const star = new THREE.Vector3(Math.random() - 0.5, Math.random() - 0.5, Math.random() - 0.5).normalize().multiplyScalar(16 + Math.random() * 12);
    starPositions[i * 3] = star.x;
    starPositions[i * 3 + 1] = star.y;
    starPositions[i * 3 + 2] = star.z;
  }
  const starGeometry = new THREE.BufferGeometry();
  starGeometry.setAttribute("position", new THREE.BufferAttribute(starPositions, 3));
  scene.add(new THREE.Points(starGeometry, new THREE.PointsMaterial({ color: 0x8ea3c9, size: 0.05, transparent: true, opacity: 0.75, depthWrite: false })));
  globe.add(new THREE.Mesh(new THREE.SphereGeometry(radius * 0.996, 48, 32), new THREE.MeshBasicMaterial({ color: 0x0c1220 })));

  const dotCount = 2800;
  const positions = new Float32Array(dotCount * 3);
  const colors = new Float32Array(dotCount * 3);
  const accent = new THREE.Color("#10b981");
  const dim = new THREE.Color("#3d4a63");
  const goldenAngle = Math.PI * (3 - Math.sqrt(5));
  for (let i = 0; i < dotCount; i += 1) {
    const y = 1 - (i / (dotCount - 1)) * 2;
    const ring = Math.sqrt(1 - y * y);
    const theta = goldenAngle * i;
    positions[i * 3] = Math.cos(theta) * ring * radius;
    positions[i * 3 + 1] = y * radius;
    positions[i * 3 + 2] = Math.sin(theta) * ring * radius;
    const color = dim.clone().lerp(accent, Math.random() * 0.22);
    colors[i * 3] = color.r;
    colors[i * 3 + 1] = color.g;
    colors[i * 3 + 2] = color.b;
  }
  const dotGeometry = new THREE.BufferGeometry();
  dotGeometry.setAttribute("position", new THREE.BufferAttribute(positions, 3));
  dotGeometry.setAttribute("color", new THREE.BufferAttribute(colors, 3));
  globe.add(new THREE.Points(dotGeometry, new THREE.PointsMaterial({ size: 0.016, vertexColors: true, transparent: true, opacity: 0.95, depthWrite: false })));

  const toVector = (lat: number, lng: number, r = radius * 1.004) => {
    const phi = (90 - lat) * Math.PI / 180;
    const theta = (lng + 180) * Math.PI / 180;
    return new THREE.Vector3(-r * Math.sin(phi) * Math.cos(theta), r * Math.cos(phi), r * Math.sin(phi) * Math.sin(theta));
  };
  const cities = [
    { key: "california", lat: 37.77, lng: -122.42, color: 0x10b981, big: true },
    { key: "tokyo", lat: 35.68, lng: 139.69, color: 0x7dd3fc, big: true },
    { key: "frankfurt", lat: 50.11, lng: 8.68, color: 0x8b93a3 },
    { key: "singapore", lat: 1.35, lng: 103.82, color: 0x8b93a3 },
    { key: "sydney", lat: -33.87, lng: 151.21, color: 0x8b93a3 },
    { key: "virginia", lat: 38.9, lng: -77.04, color: 0x8b93a3 },
  ];
  const markers: Array<{ ring: any; material: any; phase: number }> = [];
  for (const city of cities) {
    const point = toVector(city.lat, city.lng);
    globe.add(new THREE.Mesh(new THREE.SphereGeometry(city.big ? 0.038 : 0.026, 12, 12), new THREE.MeshBasicMaterial({ color: city.color })));
    const ring = new THREE.Mesh(new THREE.RingGeometry(0.05, 0.06, 32), new THREE.MeshBasicMaterial({ color: city.color, transparent: true, opacity: 0.8, side: THREE.DoubleSide }));
    ring.position.copy(point);
    ring.lookAt(point.clone().multiplyScalar(2));
    globe.add(ring);
    markers.push({ ring, material: ring.material, phase: Math.random() * 6 });
    const labelCanvas = document.createElement("canvas");
    labelCanvas.width = 384;
    labelCanvas.height = 64;
    const labelContext = labelCanvas.getContext("2d");
    if (labelContext) {
      const color = `#${new THREE.Color(city.color).getHexString()}`;
      labelContext.font = "600 30px ui-monospace, Consolas, monospace";
      labelContext.textAlign = "center";
      labelContext.textBaseline = "middle";
      labelContext.shadowColor = color;
      labelContext.shadowBlur = 10;
      labelContext.fillStyle = color;
      labelContext.fillText(labels[city.key] || city.key, 192, 34);
      const label = new THREE.Sprite(new THREE.SpriteMaterial({ map: new THREE.CanvasTexture(labelCanvas), transparent: true, depthWrite: false }));
      label.position.copy(point.clone().normalize().multiplyScalar(radius * 1.24));
      const scale = city.big ? 1 : 0.78;
      label.scale.set(1.15 * scale, 0.19 * scale, 1);
      globe.add(label);
    }
  }
  const arcPairs = [[0, 1], [0, 5], [1, 3], [1, 4], [0, 2]];
  const packets: Array<{ curve: any; mesh: any; offset: number; speed: number }> = [];
  arcPairs.forEach(([a, b], index) => {
    const start = toVector(cities[a].lat, cities[a].lng);
    const end = toVector(cities[b].lat, cities[b].lng);
    const middle = start.clone().add(end).multiplyScalar(0.5).normalize().multiplyScalar(radius + start.distanceTo(end) * 0.42);
    const curve = new THREE.QuadraticBezierCurve3(start, middle, end);
    globe.add(new THREE.Line(new THREE.BufferGeometry().setFromPoints(curve.getPoints(64)), new THREE.LineBasicMaterial({ color: index === 0 ? 0x10b981 : 0x7dd3fc, transparent: true, opacity: 0.6 })));
    const mesh = new THREE.Mesh(new THREE.SphereGeometry(0.024, 8, 8), new THREE.MeshBasicMaterial({ color: index === 0 ? 0xffffff : 0x7dd3fc, transparent: true }));
    globe.add(mesh);
    packets.push({ curve, mesh, offset: Math.random(), speed: 0.12 + Math.random() * 0.1 });
  });
  const orbit = new THREE.Group();
  orbit.add(new THREE.Mesh(new THREE.TorusGeometry(2.3, 0.005, 8, 160), new THREE.MeshBasicMaterial({ color: 0x7dd3fc, transparent: true, opacity: 0.35 })));
  [0xffffff, 0x10b981].forEach((color, index) => {
    const satellite = new THREE.Mesh(new THREE.SphereGeometry(0.035, 10, 10), new THREE.MeshBasicMaterial({ color }));
    orbit.add(satellite);
    (satellite as any).orbitAngle = index * Math.PI;
  });
  orbit.rotation.set(Math.PI / 2.25, 0.35, 0);
  scene.add(orbit);
  let dragging = false;
  let lastX = 0;
  let lastY = 0;
  let velocityX = 0;
  const onDown = (event: PointerEvent) => { dragging = true; lastX = event.clientX; lastY = event.clientY; canvas.setPointerCapture(event.pointerId); };
  const onMove = (event: PointerEvent) => { if (!dragging) return; velocityX = (event.clientX - lastX) * 0.005; globe.rotation.y += velocityX; globe.rotation.x += (event.clientY - lastY) * 0.005; lastX = event.clientX; lastY = event.clientY; };
  const onUp = () => { dragging = false; };
  canvas.addEventListener("pointerdown", onDown);
  canvas.addEventListener("pointermove", onMove);
  canvas.addEventListener("pointerup", onUp);
  const resize = () => { const width = canvas.clientWidth; const height = canvas.clientHeight; if (!width || !height) return; renderer.setSize(width, height, false); camera.aspect = width / height; camera.updateProjectionMatrix(); };
  resize();
  window.addEventListener("resize", resize, { passive: true });
  let frame = 0;
  let raf = 0;
  const render = () => {
    frame += 1;
    if (!dragging) { globe.rotation.y += 0.0022 + velocityX; globe.rotation.x *= 0.99; velocityX *= 0.95; }
    markers.forEach((marker) => { const pulse = 1 + Math.sin(frame * 0.035 + marker.phase) * 0.45; marker.ring.scale.set(pulse, pulse, 1); marker.material.opacity = 0.75 - Math.sin(frame * 0.035 + marker.phase) * 0.4; });
    packets.forEach((packet) => { packet.offset = (packet.offset + packet.speed * 0.016) % 1; packet.mesh.position.copy(packet.curve.getPoint(packet.offset)); packet.mesh.material.opacity = Math.min(1, (1 - packet.offset) * 2); });
    orbit.children.slice(1).forEach((satellite: any) => { satellite.orbitAngle += 0.008; satellite.position.set(Math.cos(satellite.orbitAngle) * 2.3, Math.sin(satellite.orbitAngle) * 2.3, 0); });
    orbit.rotation.z += 0.0004;
    renderer.render(scene, camera);
    raf = window.requestAnimationFrame(render);
  };
  render();
  globeCleanup = () => { window.cancelAnimationFrame(raf); themeObserver.disconnect(); window.removeEventListener("resize", resize); canvas.removeEventListener("pointerdown", onDown); canvas.removeEventListener("pointermove", onMove); canvas.removeEventListener("pointerup", onUp); renderer.dispose(); dotGeometry.dispose(); starGeometry.dispose(); };
}

// 首页动画只在组件挂载期间运行，离开页面时统一释放监听器和定时器。
onMounted(() => {
  window.addEventListener("scroll", onScroll, { passive: true });
  authStore.checkAuth();
  void Promise.all([loadMarketplace(), loadStats()]);
  revealObserver = new IntersectionObserver((entries) => entries.forEach((entry) => entry.isIntersecting && entry.target.classList.add("is-visible")), { threshold: 0.12 });
  document.querySelectorAll(".reveal").forEach((el) => revealObserver?.observe(el));
  visualCleanup = setupVisualEffects();
  streamTimer = window.setInterval(() => {
    const next = [...streamRows.value.slice(1), streamRows.value[0]];
    streamRows.value = next.map(([name, color, latency], index) => [name, color, `${Math.max(10, Number.parseInt(latency, 10) + ((index % 3) - 1) * 2)}ms`, 42 + ((index * 17 + Date.now() / 1000) % 44)] as [string, string, string, number]);
  }, 2200);
  timer = window.setInterval(() => {
    progress.value += 2;
    if (progress.value >= 100) selectStage((activeStage.value + 1) % stages.length);
  }, 80);
});
onBeforeUnmount(() => {
  window.removeEventListener("scroll", onScroll);
  if (timer) window.clearInterval(timer);
  if (streamTimer) window.clearInterval(streamTimer);
  visualCleanup?.();
  revealObserver?.disconnect();
});
</script>

<template>
  <GoogleOneTap :enabled="googleOneTapEligible" :client-id="googleOneTapClientID" />
  <div v-if="homeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0" allowfullscreen />
    <div v-else v-html="homeContent" />
  </div>
  <div v-else class="meteor-page">
    <div class="aurora" aria-hidden="true"><i class="b1" /><i class="b2" /><i class="b3" /></div>
    <div class="grid-fx" aria-hidden="true" />
    <nav class="meteor-nav" :class="{ scrolled }">
      <div class="nav-inner">
        <router-link to="/home" class="brand"
          ><span class="brand-mark"><img v-if="siteLogo" :src="siteLogo" alt="" /><span v-else class="brand-glyph" aria-hidden="true" /></span><span>{{ siteName }}</span></router-link
        >
        <div class="nav-links"><a href="#pillars">{{ t("home.meteor.nav.platform") }}</a><a href="#network">{{ t("home.meteor.nav.network") }}</a><a href="#pipeline">{{ t("home.meteor.nav.pipeline") }}</a><a href="#control">{{ t("home.meteor.nav.controlPlane") }}</a><a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t("home.docs") }}</a></div>
        <div class="nav-actions">
          <LocaleSwitcher /><button class="icon-btn" type="button" :title="isDark ? t('home.switchToLight') : t('home.switchToDark')" @click="toggleTheme"><Icon :name="isDark ? 'sun' : 'moon'" size="sm" /></button
          ><router-link v-if="isAuthenticated" :to="dashboardPath" class="btn small"
            ><span class="avatar">{{ userInitial }}</span
            >{{ t("home.dashboard") }}</router-link
          ><router-link v-else to="/login" class="btn small">{{ t("home.login") }}</router-link
          ><router-link v-if="!isAuthenticated" to="/register" class="btn small primary">{{ t("home.meteor.nav.getApiKey") }}</router-link>
        </div>
      </div>
    </nav>

    <header class="hero">
      <canvas ref="heroCanvas" class="hero-webgl" aria-hidden="true" />
      <div class="hero-particles" aria-hidden="true"><i v-for="n in 34" :key="n" :style="{ '--i': n }" /></div>
      <div class="sweep" />
      <div class="sweep second" />
      <div class="wrap hero-content">
        <div class="badge"><span class="dot" />{{ t("home.meteor.hero.badge") }}</div>
        <h1>
          {{ heroTitleParts[0] }}<br /><span class="grad">{{ heroTitleParts[1] }}</span>
        </h1>
        <p class="hero-sub">{{ t("home.meteor.hero.subtitle") }}</p>
        <p class="hero-support">{{ t("home.meteor.hero.supporting") }}</p>
        <p class="hero-footnote">{{ t("home.meteor.hero.footnote") }}</p>
        <div class="hero-ctas">
          <router-link :to="isAuthenticated ? dashboardPath : '/register'" class="btn primary">{{ t("home.meteor.hero.startRouting") }} <span>→</span></router-link
          ><a href="#pipeline" class="btn">{{ t("home.meteor.hero.seeHow") }}</a>
        </div>
        <div class="obs-panel">
          <div class="obs-head">
            <span class="rec" />{{ t("home.meteor.observability.title") }} <span>{{ t("home.meteor.observability.regions") }}</span>
          </div>
          <div class="obs-body">
            <div class="obs-stream">
              <div v-for="row in streamRows" :key="row[0]" class="obs-row">
                <span class="provider-badge" :style="{ background: row[1] }">{{ row[0].slice(0, 2).toUpperCase() }}</span
                ><strong>{{ row[0] }}</strong>
                <div class="bar"><i :style="{ width: `${row[3]}%` }" /></div>
                <em
                  ><b>{{ row[2] }}</b></em
                >
              </div>
            </div>
            <div class="obs-side">
              <div>
                <small>{{ t("home.meteor.metrics.averageLatency") }}</small
                ><strong class="green">{{ metricValues.averageLatency }}</strong>
              </div>
              <div>
                <small>{{ t("home.meteor.metrics.uptime") }}</small
                ><strong>{{ metricValues.uptime }}</strong>
              </div>
              <div>
                <small>{{ t("home.meteor.metrics.providers") }}</small
                ><strong class="blue">{{ metricValues.providers }}</strong>
              </div>
            </div>
          </div>
        </div>
        <div class="providers">
          <div class="eyebrow">{{ t("home.meteor.providers.heading") }}</div>
          <p v-if="marketplaceError" class="provider-error">{{ t("home.meteor.providers.unavailable") }}</p>
          <div class="marquee">
            <div class="marquee-track">
              <span v-for="(p, i) in displayProviderRows" :key="`${p[0]}-${i}`" class="provider-chip"
                ><b :style="{ background: p[1] }"><ProviderIcon :brand="p[0]" size="12px" /></b>{{ p[0] }}</span
              >
            </div>
          </div>
          <div class="marquee reverse">
            <div class="marquee-track">
              <span v-for="(p, i) in [...displayProviderRows].reverse()" :key="`${p[0]}-r-${i}`" class="provider-chip"
                ><b :style="{ background: p[1] }"><ProviderIcon :brand="p[0]" size="12px" /></b>{{ p[0] }}</span
              >
            </div>
          </div>
        </div>
      </div>
    </header>

    <main>
      <section id="pillars" class="section">
        <div class="wrap">
          <div class="reveal">
            <div class="eyebrow accent">{{ t("home.meteor.pillars.eyebrow") }}</div>
            <h2>{{ t("home.meteor.pillars.titlePrimary") }}<br />{{ t("home.meteor.pillars.titleSecondary") }}</h2>
          </div>
          <div class="pillars-grid">
            <article v-for="pillar in pillars" :key="pillar.key" class="pillar reveal" :class="pillar.tone">
              <div class="pillar-glow" :class="pillar.tone" aria-hidden="true" />
              <div class="pillar-icon"><Icon :name="pillar.icon" size="md" /></div>
              <small>{{ t(`home.meteor.pillars.items.${pillar.key}.number`) }}</small>
              <h3>{{ t(`home.meteor.pillars.items.${pillar.key}.title`) }}</h3>
              <p>{{ t(`home.meteor.pillars.items.${pillar.key}.description`) }}</p>
            </article>
          </div>
          <div class="stats-band reveal">
            <div v-for="stat in displayStats" :key="stat[1]">
              <strong>{{ stat[0] }}</strong
              ><span>{{ stat[1] }}</span>
            </div>
          </div>
          <p v-if="statsError" class="stats-error">{{ t("home.meteor.stats.unavailable") }}</p>
        </div>
      </section>
      <section id="network" class="section network-section">
        <div class="wrap network-grid">
          <div class="reveal">
            <div class="eyebrow accent">{{ t("home.meteor.network.eyebrow") }}</div>
            <h2>{{ t("home.meteor.network.titlePrimary") }}<br />{{ t("home.meteor.network.titleSecondary") }}</h2>
            <p class="muted">{{ t("home.meteor.network.description") }}</p>
            <div class="net-mini">
              <div>
                <small>{{ t("home.meteor.network.metrics.routedMonthly") }}</small
                ><b>{{ t("home.meteor.network.metrics.routedMonthlyValue") }}</b>
              </div>
              <div>
                <small>{{ t("home.meteor.metrics.providers") }}</small
                ><b>{{ metricValues.providers }}</b>
              </div>
              <div>
                <small>{{ t("home.meteor.metrics.uptime") }}</small
                ><b>{{ metricValues.uptime }}</b>
              </div>
              <div>
                <small>{{ t("home.meteor.metrics.averageLatency") }}</small
                ><b>{{ metricValues.averageLatency }}</b>
              </div>
            </div>
          </div>
          <div class="network-panel reveal">
            <div class="globe">
              <canvas ref="globeCanvas" class="globe-canvas" aria-hidden="true" />
              <div class="globe-ring ring-a" />
              <div class="globe-ring ring-b" />
              <div class="globe-core" />
              <span class="node california">{{ networkNodes.california }}</span><span class="node tokyo">{{ networkNodes.tokyo }}</span><span class="node london">{{ networkNodes.london }}</span><span class="arc arc-a" /><span class="arc arc-b" />
            </div>
            <div class="network-legend">
              <span><i class="green-dot" />{{ t("home.meteor.network.primary") }}</span
              ><span><i class="blue-dot" />{{ t("home.meteor.network.acceleration") }}</span
              ><span><i class="gray-dot" />{{ t("home.meteor.network.liveRequests") }}</span>
            </div>
            <div class="globe-hint">{{ t("home.meteor.network.canvasHint") }}</div>
          </div>
        </div>
      </section>
      <section id="pipeline" class="section">
        <div class="wrap">
          <div class="reveal">
            <div class="eyebrow accent">{{ t("home.meteor.pipeline.eyebrow") }}</div>
            <h2>{{ t("home.meteor.pipeline.titlePrimary") }}<br />{{ t("home.meteor.pipeline.titleSecondary") }}</h2>
            <p class="muted wide">{{ t("home.meteor.pipeline.description") }}</p>
          </div>
          <div class="pipeline reveal">
            <div class="pipeline-head">
              <span><b>POST</b> {{ requestPath }}</span
              ><span
                >{{ t("home.meteor.pipeline.inFlight") }} <strong>{{ Math.round(activeStage * 8 + progress / 8) }}ms</strong></span
              >
            </div>
            <div class="stage-grid">
              <button v-for="(stage, i) in stages" :key="stage.key" type="button" class="stage" :class="{ active: activeStage === i, done: activeStage > i }" @click="selectStage(i)">
                <small>0{{ i + 1 }}</small
                ><b>{{ t(`home.meteor.pipeline.stages.${stage.key}.name`) }}</b
                ><em>{{ stage.duration }}</em
                ><i :style="{ transform: `scaleX(${activeStage > i || activeStage === i ? 1 : 0})` }" />
              </button>
            </div>
            <div class="stage-detail">
              <small>0{{ activeStage + 1 }}</small>
              <div>
                <h3>{{ t(`home.meteor.pipeline.stages.${stages[activeStage].key}.title`) }}</h3>
                <p>{{ t(`home.meteor.pipeline.stages.${stages[activeStage].key}.description`) }}</p>
                <span v-if="t(`home.meteor.pipeline.stages.${stages[activeStage].key}.chip`)" class="detail-chip">{{ t(`home.meteor.pipeline.stages.${stages[activeStage].key}.chip`) }}</span>
              </div>
            </div>
          </div>
        </div>
      </section>
      <section id="control" class="section control-section">
        <div class="wrap">
          <div class="reveal">
            <div class="eyebrow accent">{{ t("home.meteor.control.eyebrow") }}</div>
            <h2>{{ t("home.meteor.control.title") }}</h2>
            <p class="muted wide">{{ t("home.meteor.control.description") }}</p>
          </div>
          <div class="control-tabs reveal">
            <button v-for="(tab, i) in controlTabs" :key="tab.key" type="button" :class="{ on: activeControl === i }" @click="activeControl = i">{{ t(`home.meteor.control.tabs.${tab.key}.tag`) }}</button>
          </div>
          <div class="control-body reveal">
            <div class="control-copy">
              <div class="eyebrow accent">{{ t(`home.meteor.control.tabs.${controlTabs[activeControl].key}.tag`) }}</div>
              <h3>{{ t(`home.meteor.control.tabs.${controlTabs[activeControl].key}.title`) }}</h3>
              <p>{{ t(`home.meteor.control.tabs.${controlTabs[activeControl].key}.description`) }}</p>
              <ul>
                <li v-for="feature in 4" :key="feature">{{ t(`home.meteor.control.tabs.${controlTabs[activeControl].key}.features.${feature - 1}`) }}</li>
              </ul>
            </div>
            <div class="control-viz">
              <div v-if="controlTabs[activeControl].type === 'route'" class="route-viz"><span class="viz-request">{{ t("home.meteor.control.visual.request") }}</span><span class="viz-router">{{ t("home.meteor.control.visual.router") }}</span><span class="viz-provider p1">{{ visualProviders.openai }}</span><span class="viz-provider p2">{{ visualProviders.gemini }}</span><span class="viz-provider p3">{{ visualProviders.grok }}</span></div>
              <div v-else-if="controlTabs[activeControl].type === 'govern'" class="govern-viz"><strong>{{ t("home.meteor.control.visual.policyEngine") }}</strong><span>{{ t("home.meteor.control.visual.budget") }}</span><span>{{ t("home.meteor.control.visual.rate") }}</span><span>{{ t("home.meteor.control.visual.allow") }}</span><span>{{ t("home.meteor.control.visual.deny") }}</span><b>{{ t("home.meteor.control.visual.pass") }}</b></div>
              <div v-else class="observe-viz">
                <div class="trace-line" />
                <span>{{ t("home.meteor.control.visual.ingress") }}</span><span>{{ t("home.meteor.control.visual.auth") }}</span><span>{{ t("home.meteor.control.visual.route") }}</span><strong>{{ t("home.meteor.control.visual.done") }}</strong>
              </div>
            </div>
          </div>
          <div class="flow-wrap reveal">
            <!-- 直接复用静态设计稿的流量图，保留其信息层级与动画节奏。 -->
            <svg viewBox="0 0 1060 300" fill="none" aria-hidden="true">
              <g font-family="monospace" font-size="11">
                <text x="60" y="40" fill="#f87171">{{ t("home.meteor.flow.chaoticTraffic") }}</text>
                <g stroke="rgba(248,113,113,.35)" stroke-width="1.2"><path d="M60 70 C 180 60, 240 110, 430 140" /><path d="M60 100 C 190 130, 250 90, 430 145" /><path d="M60 130 C 170 90, 260 150, 430 150" /><path d="M60 160 C 200 200, 240 130, 430 155" /><path d="M60 190 C 170 160, 260 210, 430 160" /><path d="M60 220 C 190 250, 250 180, 430 165" /></g>
                <g fill="#f87171" opacity=".7"><circle cx="60" cy="66" r="3" /><circle cx="60" cy="96" r="3" /><circle cx="60" cy="126" r="3" /><circle cx="60" cy="156" r="3" /><circle cx="60" cy="186" r="3" /><circle cx="60" cy="216" r="3" /></g>
              </g>
              <g class="core-pulse"><rect x="430" y="105" width="200" height="90" rx="12" fill="rgba(16,185,129,.05)" stroke="rgba(16,185,129,.45)" stroke-width="1.2" /><text x="530" y="140" text-anchor="middle" fill="#eef1f6" font-size="13" font-weight="600" font-family="sans-serif">{{ t("home.meteor.flow.platform") }}</text><text x="530" y="160" text-anchor="middle" fill="#8b93a3" font-size="10" font-family="monospace">{{ t("home.meteor.flow.stages") }}</text></g>
              <circle cx="530" cy="150" r="46" fill="none" stroke="rgba(16,185,129,.25)" stroke-width="1" class="ripple" />
              <g font-family="monospace" font-size="11"><text x="915" y="40" fill="#10b981">{{ t("home.meteor.flow.orderedOutput") }}</text><g stroke="rgba(16,185,129,.5)" stroke-width="1.4"><path d="M630 140 C 760 148, 830 150, 890 150" class="flow-arc" /><path d="M630 145 C 760 150, 830 152, 890 152" class="flow-arc" /></g><circle cx="900" cy="150" r="4" fill="#10b981" /><g fill="#565e6e"><circle cx="930" cy="130" r="2.5" /><circle cx="945" cy="150" r="2.5" /><circle cx="930" cy="170" r="2.5" /></g><text x="915" y="200" fill="#565e6e" font-size="10">{{ t("home.meteor.flow.protocol") }}</text></g>
              <g font-family="monospace" font-size="10.5" fill="#565e6e"><text x="70" y="265">{{ t("home.meteor.flow.route") }}</text><text x="460" y="265">{{ t("home.meteor.flow.govern") }}</text><text x="860" y="265">{{ t("home.meteor.flow.observe") }}</text></g>
            </svg>
          </div>
          <p class="muted flow-note reveal">{{ t("home.meteor.flow.description") }}</p>
        </div>
      </section>
    </main>
    <footer>
      <div class="wrap footer-grid">
        <div>
          <router-link to="/home" class="brand"
            ><span class="brand-mark"><img v-if="siteLogo" :src="siteLogo" alt="" /><span v-else class="brand-glyph" aria-hidden="true" /></span><span>{{ siteName }}</span></router-link
          >
          <p>{{ t("home.meteor.footer.description") }}</p>
        </div>
        <div>
          <h4>{{ t("home.meteor.footer.product") }}</h4>
          <a href="#pillars">{{ t("home.meteor.footer.routing") }}</a
          ><a href="#control">{{ t("home.meteor.footer.governance") }}</a
          ><a href="#pipeline">{{ t("home.meteor.footer.observability") }}</a
          ><a href="#control">{{ t("home.meteor.footer.costControl") }}</a>
        </div>
        <div>
          <h4>{{ t("home.meteor.footer.developers") }}</h4>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener">{{ t("home.meteor.footer.documentation") }}</a
          ><a href="#pipeline">{{ t("home.meteor.footer.apiReference") }}</a><a href="#network">{{ t("home.meteor.footer.sdks") }}</a><a href="#network">{{ t("home.meteor.footer.status") }}</a>
        </div>
        <div>
          <h4>{{ t("home.meteor.footer.company") }}</h4>
          <a href="#">{{ t("home.meteor.footer.about") }}</a
          ><a href="#">{{ t("home.meteor.footer.blog") }}</a><a href="#">{{ t("home.meteor.footer.careers") }}</a><a href="#">{{ t("home.meteor.footer.contact") }}</a>
        </div>
      </div>
      <div class="wrap footer-bottom">
        <span>© {{ new Date().getFullYear() }} {{ siteName }}</span><span>{{ t("home.meteor.footer.regions") }}</span>
      </div>
    </footer>
  </div>
</template>

<style scoped>
:global(*) {
  box-sizing: border-box;
}
:global(html) {
  scroll-behavior: smooth;
}
:global(body) {
  margin: 0;
  font-family: -apple-system, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
}
.meteor-page {
  --bg: #10141d;
  --panel: #151a26;
  --line: rgba(255, 255, 255, 0.09);
  --txt: #eef1f6;
  --dim: #8b93a3;
  --muted: #687183;
  --green: #10b981;
  --blue: #7dd3fc;
  --amber: #fbbf24;
  --mono: "SF Mono", "Cascadia Code", Consolas, "JetBrains Mono", ui-monospace, monospace;
  --sans: -apple-system, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  min-height: 100vh;
  overflow: hidden;
  background: var(--bg);
  color: var(--txt);
  font-family: var(--sans);
  line-height: 1.6;
}
.wrap {
  width: min(1320px, calc(100% - 64px));
  margin: 0 auto;
  position: relative;
  z-index: 2;
}
.aurora,
.grid-fx {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}
.aurora i {
  position: absolute;
  width: 42vw;
  height: 42vw;
  border-radius: 50%;
  filter: blur(72px);
  opacity: 0.55;
  animation: drift 22s ease-in-out infinite;
}
.aurora .b1 {
  top: -16%;
  left: 2%;
  background: radial-gradient(circle, rgba(16, 185, 129, 0.17), transparent 68%);
}
.aurora .b2 {
  top: -10%;
  right: -5%;
  background: radial-gradient(circle, rgba(125, 211, 252, 0.13), transparent 68%);
  animation-delay: -8s;
}
.aurora .b3 {
  top: 30%;
  left: 30%;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.08), transparent 68%);
  animation-delay: -13s;
}
.grid-fx {
  opacity: 0.45;
  background-image: linear-gradient(rgba(255, 255, 255, 0.035) 1px, transparent 1px), linear-gradient(90deg, rgba(255, 255, 255, 0.035) 1px, transparent 1px);
  background-size: 72px 72px;
  mask-image: radial-gradient(ellipse 90% 60% at 50% 0%, #000 20%, transparent 75%);
  animation: grid-flow 10s linear infinite;
}
@keyframes drift {
  50% {
    transform: translate(4%, -5%) scale(1.08);
  }
}
@keyframes grid-flow {
  to {
    background-position: 72px 72px;
  }
}
.meteor-nav {
  position: fixed;
  inset: 0 0 auto;
  z-index: 20;
  border-bottom: 1px solid transparent;
  background: rgba(13, 17, 25, 0.48);
  backdrop-filter: blur(16px);
  transition: 0.3s;
}
.meteor-nav.scrolled {
  border-bottom-color: var(--line);
  background: rgba(13, 17, 25, 0.82);
}
.nav-inner {
  width: min(1320px, calc(100% - 64px));
  height: 64px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  gap: 34px;
}
.brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--txt);
  text-decoration: none;
  font-weight: 700;
  letter-spacing: 0.02em;
}
.brand-mark {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border-radius: 0;
  background: transparent;
  animation: none;
}
.brand-mark span {
  display: block;
}
.brand-glyph {
  position: relative;
  width: 24px;
  height: 24px;
}
.brand-glyph::before,
.brand-glyph::after {
  position: absolute;
  content: "";
  background: currentColor;
  border-radius: 3px;
}
.brand-glyph::before {
  top: 4px;
  left: 2px;
  width: 20px;
  height: 5px;
}
.brand-glyph::after {
  top: 7px;
  left: 9px;
  width: 6px;
  height: 15px;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
.nav-links {
  display: flex;
  gap: 26px;
  font-size: 13px;
  color: var(--dim);
}
.nav-links a,
footer a {
  color: inherit;
  text-decoration: none;
  transition: color 0.2s;
}
.nav-links a:hover,
footer a:hover {
  color: var(--txt);
}
.nav-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-left: auto;
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
  justify-content: center;
  gap: 8px;
  min-height: 42px;
  padding: 9px 18px;
  border: 1px solid rgba(255, 255, 255, 0.17);
  border-radius: 8px;
  background: transparent;
  color: var(--txt);
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
  transition: 0.25s;
}
.btn:hover {
  border-color: rgba(255, 255, 255, 0.35);
  background: rgba(255, 255, 255, 0.05);
  transform: translateY(-1px);
}
.btn.small {
  min-height: 34px;
  padding: 6px 13px;
  font-size: 12px;
}
.btn.primary {
  border-color: var(--green);
  background: var(--green);
  color: #04110b;
  box-shadow: 0 8px 26px -12px rgba(16, 185, 129, 0.8);
}
.btn.primary:hover {
  background: #12c98e;
}
.avatar {
  width: 20px;
  height: 20px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.24);
  font-size: 10px;
}
.hero {
  position: relative;
  min-height: 820px;
  padding: 190px 0 96px;
  overflow: hidden;
  text-align: center;
}
.hero-webgl {
  position: absolute;
  inset: 0;
  z-index: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
.hero-content {
  z-index: 2;
}
.hero-particles {
  position: absolute;
  inset: 0;
  opacity: 0.55;
  pointer-events: none;
}
.hero-particles i {
  position: absolute;
  left: calc((var(--i) * 29) %100 * 1%);
  top: calc((var(--i) * 47) %100 * 1%);
  width: 2px;
  height: 2px;
  border-radius: 50%;
  background: var(--blue);
  box-shadow: 0 0 12px var(--blue);
  animation: float-particle calc(3s + (var(--i) %5) * 1s) ease-in-out infinite alternate;
}
@keyframes float-particle {
  to {
    transform: translate(24px, -18px);
    opacity: 0.2;
  }
}
.sweep {
  position: absolute;
  top: 40%;
  left: -40vw;
  width: 38vw;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--blue), var(--green), transparent);
  filter: blur(1px);
  animation: sweep 7s ease-in-out infinite;
}
.sweep.second {
  top: 58%;
  animation-delay: 3.2s;
}
@keyframes sweep {
  0%,
  18% {
    opacity: 0;
  }
  35% {
    opacity: 0.9;
  }
  78%,
  100% {
    opacity: 0;
    transform: translateX(180vw);
  }
}
.badge,
.eyebrow {
  color: var(--muted);
  font:
    11px/1.3 ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
  letter-spacing: 0.13em;
  text-transform: uppercase;
}
.badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 13px 6px 10px;
  border: 1px solid var(--line);
  border-radius: 99px;
  background: rgba(255, 255, 255, 0.025);
  animation: rise 0.8s both;
}
.dot,
.rec {
  display: inline-block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--green);
  box-shadow: 0 0 9px var(--green);
}
.rec {
  background: #f87171;
  box-shadow: 0 0 9px #f87171;
}
.hero h1 {
  max-width: 1080px;
  margin: 32px auto 24px;
  font-size: clamp(58px, 8.5vw, 124px);
  line-height: 1.04;
  letter-spacing: -0.04em;
  animation: rise 0.9s 0.08s both;
}
.hero h1 span {
  background: linear-gradient(90deg, var(--green), var(--blue) 34%, #a5f3fc 50%, var(--blue) 66%, var(--green));
  background-size: 220% 100%;
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  text-shadow: 0 0 26px rgba(125, 211, 252, 0.25);
  animation: gradient-flow 6s linear infinite;
}
@keyframes gradient-flow {
  to {
    background-position: 220% 0;
  }
}
.hero-sub {
  max-width: 920px;
  margin: 0 auto 12px;
  color: var(--dim);
  font-size: clamp(22px, 2.4vw, 32px);
  line-height: 1.35;
  animation: rise 0.9s 0.16s both;
}
.hero-support,
.hero-footnote {
  max-width: 920px;
  margin: 0 auto;
  color: var(--dim);
  font-size: clamp(17px, 1.5vw, 22px);
  line-height: 1.6;
}
.hero-footnote {
  margin-top: 4px;
  color: var(--muted);
}
.hero-ctas {
  display: flex;
  justify-content: center;
  gap: 13px;
  flex-wrap: wrap;
  margin-bottom: 66px;
  animation: rise 0.9s 0.24s both;
}
@keyframes rise {
  from {
    opacity: 0;
    transform: translateY(18px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
.obs-panel,
.pipeline,
.control-body,
.network-panel,
.flow-wrap {
  border: 1px solid var(--line);
  border-radius: 14px;
  background: rgba(21, 26, 38, 0.76);
  box-shadow: 0 28px 75px rgba(0, 0, 0, 0.34);
  backdrop-filter: blur(10px);
}
.obs-panel {
  overflow: hidden;
  text-align: left;
  animation: rise 1s 0.34s both;
}
.obs-head,
.pipeline-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 13px 18px;
  border-bottom: 1px solid var(--line);
  color: var(--muted);
  font:
    11px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
  letter-spacing: 0.08em;
}
.obs-head {
  justify-content: flex-start;
}
.obs-head span:last-child {
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
}
.obs-row {
  display: flex;
  align-items: center;
  gap: 11px;
  margin-bottom: 7px;
  padding: 9px 11px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.025);
  font:
    13px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
}
.provider-badge,
.provider-chip b {
  display: inline-grid;
  place-items: center;
  flex: none;
  color: #fff;
  font-size: 9px;
  font-weight: 700;
}
.provider-badge {
  width: 19px;
  height: 19px;
  border-radius: 5px;
}
.obs-row .bar {
  flex: 1;
  height: 4px;
  max-width: 140px;
  margin-left: auto;
  overflow: hidden;
  border-radius: 2px;
  background: rgba(255, 255, 255, 0.07);
}
.obs-row .bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--blue), var(--green));
}
.obs-row em {
  min-width: 48px;
  color: var(--dim);
  font-style: normal;
  text-align: right;
}
.obs-row em b {
  color: var(--green);
}
.obs-side {
  display: flex;
  flex-direction: column;
  padding: 18px;
}
.obs-side div {
  flex: 1;
  padding: 12px 6px;
  border-bottom: 1px solid var(--line);
}
.obs-side div:last-child {
  border: 0;
}
.obs-side small,
.net-mini small {
  display: block;
  color: var(--muted);
  font:
    10px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
  letter-spacing: 0.09em;
}
.obs-side strong {
  display: block;
  margin-top: 3px;
  font:
    700 24px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
}
.green {
  color: var(--green);
  text-shadow: 0 0 16px rgba(16, 185, 129, 0.36);
}
.blue {
  color: var(--blue);
}
.providers {
  margin-top: 65px;
}
.providers .eyebrow {
  margin-bottom: 20px;
  text-align: center;
}
.provider-error,
.stats-error {
  margin: -10px auto 14px;
  color: var(--muted);
  font-size: 11px;
  text-align: center;
}
.stats-error {
  margin: 12px 0 0;
  text-align: left;
}
.marquee {
  overflow: hidden;
  padding: 3px 0;
  mask-image: linear-gradient(90deg, transparent, #000 12%, #000 88%, transparent);
}
.marquee + .marquee {
  margin-top: 10px;
}
.marquee-track {
  display: flex;
  width: max-content;
  gap: 10px;
  animation: marquee 34s linear infinite;
}
.reverse .marquee-track {
  animation-direction: reverse;
  animation-duration: 42s;
}
.marquee:hover .marquee-track {
  animation-play-state: paused;
}
@keyframes marquee {
  to {
    transform: translateX(-50%);
  }
}
.provider-chip {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  padding: 9px 16px;
  border: 1px solid var(--line);
  border-radius: 99px;
  color: var(--dim);
  font-size: 13px;
  white-space: nowrap;
}
.provider-chip b {
  width: 20px;
  height: 20px;
  border-radius: 6px;
}
.section {
  position: relative;
  z-index: 1;
  padding: 108px 0;
}
.section h2 {
  margin: 13px 0 14px;
  font-size: clamp(31px, 4vw, 46px);
  line-height: 1.12;
  letter-spacing: -0.035em;
}
.accent {
  color: var(--green);
}
.muted {
  max-width: 620px;
  color: var(--dim);
  font-size: 15px;
}
.muted.wide {
  max-width: 680px;
}
.reveal {
  opacity: 0;
  transform: translateY(22px);
  transition:
    opacity 0.8s,
    transform 0.8s;
}
.reveal.is-visible {
  opacity: 1;
  transform: none;
}
.pillars-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-top: 46px;
}
.pillar {
  position: relative;
  overflow: hidden;
  min-height: 260px;
  padding: 27px;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--panel);
  transition: 0.3s;
}
.pillar-glow {
  position: absolute;
  top: -60px;
  right: -60px;
  width: 160px;
  height: 160px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(16, 185, 129, 0.14), transparent 70%);
  opacity: 0;
  transition: opacity 0.4s;
  pointer-events: none;
}
.pillar-glow.blue {
  background: radial-gradient(circle, rgba(125, 211, 252, 0.13), transparent 70%);
}
.pillar-glow.amber {
  background: radial-gradient(circle, rgba(251, 191, 36, 0.12), transparent 70%);
}
.pillar:hover {
  transform: translateY(-4px);
  border-color: rgba(16, 185, 129, 0.4);
  box-shadow: 0 18px 45px -22px rgba(0, 0, 0, 0.8);
}
.pillar:hover .pillar-glow {
  opacity: 1;
}
.pillar-icon {
  width: 39px;
  height: 39px;
  display: grid;
  place-items: center;
  margin-bottom: 20px;
  border: 1px solid var(--line);
  border-radius: 10px;
  color: var(--green);
  background: rgba(255, 255, 255, 0.03);
}
.pillar.blue .pillar-icon {
  color: var(--blue);
}
.pillar.amber .pillar-icon {
  color: var(--amber);
}
.pillar small {
  color: var(--muted);
  font:
    10px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
  letter-spacing: 0.1em;
}
.pillar h3 {
  margin: 14px 0 9px;
  font-size: 17px;
  line-height: 1.35;
}
.pillar p {
  margin: 0;
  color: var(--dim);
  font-size: 13.5px;
  line-height: 1.65;
}
.stats-band {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  margin-top: 55px;
  overflow: hidden;
  border: 1px solid var(--line);
  border-radius: 14px;
  background: var(--panel);
}
.stats-band div {
  padding: 28px 24px;
  border-right: 1px solid var(--line);
}
.stats-band div:last-child {
  border: 0;
}
.stats-band strong {
  display: block;
  background: linear-gradient(120deg, #fff 20%, var(--blue) 50%, #fff 80%);
  background-size: 220% 100%;
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  font:
    700 38px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
  animation: gradient-flow 5s linear infinite;
}
.stats-band span {
  display: block;
  margin-top: 4px;
  color: var(--dim);
  font-size: 13px;
}
.network-section,
.control-section {
  background: linear-gradient(180deg, transparent, rgba(255, 255, 255, 0.014), transparent);
}
.network-grid {
  display: grid;
  grid-template-columns: 1fr 1.2fr;
  gap: 58px;
  align-items: center;
}
.net-mini {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 11px;
  margin-top: 28px;
}
.net-mini div {
  padding: 15px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.02);
}
.net-mini b {
  display: block;
  margin-top: 4px;
  font:
    700 19px ui-monospace,
    SFMono-Regular,
    Menlo,
    monospace;
}
.network-panel {
  position: relative;
  overflow: hidden;
  padding: 0;
}
.globe {
  position: relative;
  height: 420px;
  display: grid;
  place-items: center;
  overflow: hidden;
  background: radial-gradient(circle at 52% 48%, rgba(16, 185, 129, 0.18), transparent 35%), radial-gradient(circle at 50% 50%, #172432, #10141d 68%);
}
.globe-core {
  width: 220px;
  height: 220px;
  border: 1px solid rgba(125, 211, 252, 0.38);
  border-radius: 50%;
  background: repeating-linear-gradient(0deg, transparent 0 26px, rgba(125, 211, 252, 0.11) 27px 28px), repeating-linear-gradient(90deg, transparent 0 26px, rgba(125, 211, 252, 0.08) 27px 28px), radial-gradient(circle at 35% 30%, rgba(125, 211, 252, 0.18), rgba(16, 185, 129, 0.06) 48%, #101923 75%);
  box-shadow: 0 0 70px rgba(16, 185, 129, 0.2);
  animation: globe-spin 18s linear infinite;
}
.webgl-ready .globe-core,
.webgl-ready .globe-ring,
.webgl-ready .node,
.webgl-ready .arc {
  display: none;
}
.globe-canvas {
  position: absolute;
  inset: 0;
  z-index: 1;
  width: 100%;
  height: 100%;
  pointer-events: auto;
  cursor: grab;
  touch-action: pan-y;
}
.globe-canvas:active {
  cursor: grabbing;
}
.globe-ring {
  position: absolute;
  width: 290px;
  height: 110px;
  border: 1px solid rgba(125, 211, 252, 0.28);
  border-radius: 50%;
  transform: rotate(-18deg);
  animation: ring-spin 12s linear infinite;
}
.ring-b {
  width: 250px;
  height: 330px;
  transform: rotate(35deg);
  animation-duration: 16s;
}
.node {
  position: absolute;
  z-index: 3;
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  color: #04110b;
  font:
    700 9px ui-monospace,
    monospace;
}
.california {
  left: 28%;
  top: 45%;
  background: var(--green);
  box-shadow: 0 0 22px var(--green);
}
.tokyo {
  right: 23%;
  top: 38%;
  background: var(--blue);
  box-shadow: 0 0 22px var(--blue);
}
.london {
  left: 41%;
  top: 25%;
  background: #8b93a3;
}
.arc {
  position: absolute;
  width: 205px;
  height: 90px;
  border-top: 1px dashed rgba(125, 211, 252, 0.65);
  border-radius: 50%;
  transform: rotate(-13deg);
}
.arc-a {
  top: 39%;
  left: 30%;
}
.arc-b {
  top: 36%;
  left: 39%;
  border-color: rgba(16, 185, 129, 0.6);
  transform: rotate(19deg);
}
.network-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  padding: 14px 18px;
  border-top: 1px solid var(--line);
  color: var(--dim);
  font:
    11px ui-monospace,
    monospace;
}
.network-legend span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.network-legend i {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}
.green-dot {
  background: var(--green);
}
.blue-dot {
  background: var(--blue);
}
.gray-dot {
  background: #565e6e;
}
.globe-hint {
  position: absolute;
  right: 15px;
  bottom: 52px;
  color: var(--muted);
  font:
    10px ui-monospace,
    monospace;
}
@keyframes globe-spin {
  to {
    transform: rotateY(360deg);
  }
}
@keyframes ring-spin {
  to {
    transform: rotate(342deg);
  }
}
.pipeline {
  margin-top: 46px;
  overflow: hidden;
}
.pipeline-head b {
  color: var(--green);
}
.pipeline-head strong {
  color: var(--txt);
}
.stage-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
}
.stage {
  position: relative;
  min-height: 116px;
  padding: 17px 14px;
  border: 0;
  border-right: 1px solid var(--line);
  background: transparent;
  color: var(--txt);
  text-align: left;
  cursor: pointer;
  transition: background 0.25s;
}
.stage:last-child {
  border-right: 0;
}
.stage:hover,
.stage.active {
  background: rgba(16, 185, 129, 0.05);
}
.stage small,
.stage em {
  display: block;
  color: var(--muted);
  font:
    10px ui-monospace,
    monospace;
}
.stage b {
  display: block;
  margin-top: 9px;
  font-size: 13px;
}
.stage em {
  margin-top: 4px;
  color: var(--green);
  font-style: normal;
}
.stage i {
  position: absolute;
  inset: auto 0 0;
  height: 2px;
  background: var(--green);
  transform-origin: left;
  transition: transform 0.4s;
}
.stage.active i {
  background: var(--blue);
  animation: pulse 1s infinite;
}
.stage-detail {
  display: flex;
  gap: 20px;
  min-height: 132px;
  padding: 26px;
  border-top: 1px solid var(--line);
}
.stage-detail > small {
  color: var(--muted);
  font:
    12px ui-monospace,
    monospace;
}
.stage-detail h3 {
  margin: 0 0 7px;
  font-size: 16px;
}
.stage-detail p {
  max-width: 680px;
  margin: 0;
  color: var(--dim);
  font-size: 13.5px;
}
.detail-chip {
  display: inline-flex;
  margin-top: 13px;
  padding: 5px 11px;
  border: 1px solid rgba(16, 185, 129, 0.35);
  border-radius: 7px;
  color: var(--green);
  background: rgba(16, 185, 129, 0.07);
  font:
    11px ui-monospace,
    monospace;
}
.control-tabs {
  display: flex;
  gap: 8px;
  margin-top: 43px;
  flex-wrap: wrap;
}
.control-tabs button {
  padding: 8px 15px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: transparent;
  color: var(--dim);
  font:
    12px ui-monospace,
    monospace;
  letter-spacing: 0.06em;
  cursor: pointer;
}
.control-tabs button.on {
  border-color: rgba(16, 185, 129, 0.45);
  color: var(--green);
  background: rgba(16, 185, 129, 0.07);
}
.control-body {
  display: grid;
  grid-template-columns: 1fr 1fr;
  min-height: 315px;
  margin-top: 21px;
  overflow: hidden;
}
.control-copy {
  padding: 35px;
  border-right: 1px solid var(--line);
}
.control-copy h3 {
  margin: 13px 0 10px;
  font-size: 26px;
}
.control-copy p {
  margin: 0;
  color: var(--dim);
  font-size: 14px;
}
.control-copy ul {
  display: grid;
  gap: 10px;
  margin: 23px 0 0;
  padding: 0;
  list-style: none;
  color: var(--dim);
  font-size: 13px;
}
.control-copy li::before {
  content: "✓";
  display: inline-grid;
  place-items: center;
  width: 16px;
  height: 16px;
  margin-right: 9px;
  border: 1px solid rgba(16, 185, 129, 0.35);
  border-radius: 4px;
  color: var(--green);
  font-size: 10px;
}
.control-viz {
  position: relative;
  display: grid;
  place-items: center;
  min-height: 300px;
  background: radial-gradient(ellipse at 60% 40%, rgba(125, 211, 252, 0.06), transparent 70%);
  font:
    10px ui-monospace,
    monospace;
}
.route-viz {
  position: relative;
  width: 320px;
  height: 230px;
}
.route-viz span {
  position: absolute;
  display: grid;
  place-items: center;
  border: 1px solid var(--line);
  border-radius: 7px;
}
.viz-request {
  top: 102px;
  left: 0;
  width: 58px;
  height: 32px;
  color: var(--muted);
}
.viz-router {
  top: 96px;
  left: 102px;
  width: 72px;
  height: 44px;
  border-color: rgba(16, 185, 129, 0.4) !important;
  color: var(--green);
}
.viz-provider {
  left: 225px;
  width: 78px;
  height: 32px;
}
.p1 {
  top: 63px;
  color: #10a37f;
}
.p2 {
  top: 99px;
  color: #4796e3;
}
.p3 {
  top: 135px;
  color: #94a3b8;
}
.route-viz:before {
  content: "";
  position: absolute;
  left: 58px;
  top: 118px;
  width: 44px;
  border-top: 1px dashed var(--dim);
  box-shadow:
    72px -35px 0 -0.5px rgba(16, 163, 127, 0.5),
    72px 0 0 -0.5px rgba(71, 150, 227, 0.5),
    72px 35px 0 -0.5px rgba(148, 163, 184, 0.5);
}
.govern-viz {
  width: 145px;
  min-height: 220px;
  display: grid;
  align-content: center;
  gap: 10px;
  padding: 19px;
  border: 1px solid rgba(125, 211, 252, 0.35);
  border-radius: 12px;
  color: var(--dim);
  text-align: center;
}
.govern-viz strong {
  color: var(--blue);
}
.govern-viz span {
  padding: 6px;
  border: 1px solid rgba(125, 211, 252, 0.2);
  border-radius: 6px;
}
.govern-viz b {
  margin-top: 5px;
  color: var(--green);
}
.observe-viz {
  position: relative;
  width: 320px;
  height: 220px;
}
.trace-line {
  position: absolute;
  left: 20px;
  right: 10px;
  top: 130px;
  height: 55px;
  border-top: 2px solid var(--green);
  border-right: 2px solid var(--green);
  transform: skewY(-27deg);
  box-shadow: 0 0 12px rgba(16, 185, 129, 0.35);
}
.observe-viz span,
.observe-viz strong {
  position: absolute;
  color: var(--dim);
}
.observe-viz span:nth-of-type(1) {
  left: 32px;
  top: 175px;
}
.observe-viz span:nth-of-type(2) {
  left: 112px;
  top: 110px;
}
.observe-viz span:nth-of-type(3) {
  left: 195px;
  top: 153px;
}
.observe-viz strong {
  right: 0;
  top: 68px;
  color: var(--green);
}
.flow-wrap {
  position: relative;
  min-height: 280px;
  margin-top: 46px;
  overflow: hidden;
}
.flow-wrap svg {
  display: block;
  width: 100%;
  height: auto;
  min-width: 720px;
}
.core-pulse {
  transform-box: fill-box;
  transform-origin: center;
  animation: core-pulse 3.2s ease-in-out infinite;
}
.flow-arc {
  stroke-dasharray: 6 8;
  animation: flow-dash 1.2s linear infinite;
}
.ripple {
  transform-box: fill-box;
  transform-origin: center;
  animation: flow-ripple 2.8s ease-out infinite;
}
@keyframes core-pulse {
  0%,
  100% {
    filter: drop-shadow(0 0 6px rgba(16, 185, 129, 0.25));
  }
  50% {
    filter: drop-shadow(0 0 20px rgba(16, 185, 129, 0.55));
  }
}
@keyframes flow-dash {
  to {
    stroke-dashoffset: -28;
  }
}
@keyframes flow-ripple {
  0% {
    transform: scale(0.4);
    opacity: 0.9;
  }
  100% {
    transform: scale(2.6);
    opacity: 0;
  }
}
.flow-label {
  position: absolute;
  top: 29px;
  font:
    11px ui-monospace,
    monospace;
}
.chaotic {
  left: 7%;
  color: #f87171;
}
.ordered {
  right: 7%;
  color: var(--green);
}
.flow-lines {
  position: absolute;
  left: 7%;
  top: 77px;
  width: 39%;
  height: 130px;
}
.flow-lines i {
  position: absolute;
  left: 0;
  width: 100%;
  height: 1px;
  background: rgba(248, 113, 113, 0.4);
}
.flow-lines i:nth-child(1) {
  top: 0;
  transform: rotate(16deg);
}
.flow-lines i:nth-child(2) {
  top: 24px;
  transform: rotate(9deg);
}
.flow-lines i:nth-child(3) {
  top: 48px;
  transform: rotate(3deg);
}
.flow-lines i:nth-child(4) {
  top: 72px;
  transform: rotate(-4deg);
}
.flow-lines i:nth-child(5) {
  top: 96px;
  transform: rotate(-10deg);
}
.flow-lines i:nth-child(6) {
  top: 120px;
  transform: rotate(-16deg);
}
.flow-core {
  position: absolute;
  left: 50%;
  top: 50%;
  display: grid;
  place-items: center;
  width: 200px;
  height: 88px;
  transform: translate(-50%, -50%);
  border: 1px solid rgba(16, 185, 129, 0.45);
  border-radius: 12px;
  background: rgba(16, 185, 129, 0.06);
  box-shadow: 0 0 28px rgba(16, 185, 129, 0.13);
}
.flow-core strong {
  font-size: 13px;
}
.flow-core small {
  color: var(--dim);
  font:
    10px ui-monospace,
    monospace;
}
.flow-output {
  position: absolute;
  right: 8%;
  top: 50%;
  display: flex;
  gap: 14px;
  align-items: center;
  transform: translateY(-50%);
}
.flow-output:before {
  content: "";
  position: absolute;
  right: 43px;
  width: 130px;
  border-top: 1px dashed var(--green);
}
.flow-output i {
  position: relative;
  z-index: 1;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #565e6e;
}
.flow-output i:first-child {
  background: var(--green);
  box-shadow: 0 0 12px var(--green);
}
.flow-steps {
  position: absolute;
  inset: auto 7% 17px;
  display: flex;
  justify-content: space-between;
  color: var(--muted);
  font:
    10px ui-monospace,
    monospace;
}
.flow-note {
  margin-top: 20px;
  font-size: 13px;
}
footer {
  position: relative;
  z-index: 1;
  padding: 60px 0 38px;
  border-top: 1px solid var(--line);
}
.footer-grid {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr;
  gap: 40px;
}
.footer-grid p {
  max-width: 270px;
  margin-top: 14px;
  color: var(--dim);
  font-size: 13px;
}
.footer-grid h4 {
  margin: 0 0 15px;
  color: var(--muted);
  font:
    11px ui-monospace,
    monospace;
  letter-spacing: 0.1em;
}
.footer-grid a {
  display: block;
  margin-bottom: 10px;
  color: var(--dim);
  font-size: 13px;
}
.footer-bottom {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-top: 50px;
  padding-top: 22px;
  border-top: 1px solid var(--line);
  color: var(--muted);
  font:
    11px ui-monospace,
    monospace;
}
@keyframes pulse {
  50% {
    opacity: 0.35;
  }
}
@media (max-width: 900px) {
  .nav-links {
    display: none;
  }
  .obs-body,
  .control-body,
  .network-grid {
    grid-template-columns: 1fr;
  }
  .obs-stream,
  .control-copy {
    border-right: 0;
    border-bottom: 1px solid var(--line);
  }
  .pillars-grid {
    grid-template-columns: 1fr;
  }
  .stats-band {
    grid-template-columns: 1fr 1fr;
  }
  .stats-band div:nth-child(2) {
    border-right: 0;
  }
  .stats-band div:nth-child(-n + 2) {
    border-bottom: 1px solid var(--line);
  }
  .stage-grid {
    grid-template-columns: repeat(3, 1fr);
  }
  .stage:nth-child(3) {
    border-right: 0;
  }
  .stage:nth-child(-n + 3) {
    border-bottom: 1px solid var(--line);
  }
  .footer-grid {
    grid-template-columns: 1fr 1fr;
  }
  .footer-grid > div:first-child {
    grid-column: span 2;
  }
}
@media (max-width: 560px) {
  .wrap,
  .nav-inner {
    width: min(100% - 30px, 1120px);
  }
  .hero {
    padding-top: 122px;
  }
  .hero h1 {
    font-size: 42px;
  }
  .obs-head span:last-child {
    display: none;
  }
  .obs-row strong {
    font-size: 12px;
  }
  .obs-row .bar {
    max-width: 70px;
  }
  .nav-actions .icon-btn,
  :deep(.locale-switcher) {
    display: none;
  }
  .network-panel .globe {
    height: 330px;
  }
  .globe-core {
    width: 180px;
    height: 180px;
  }
  .stage-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .stage:nth-child(2),
  .stage:nth-child(4),
  .stage:nth-child(6) {
    border-right: 0;
  }
  .stage:nth-child(3) {
    border-right: 1px solid var(--line);
  }
  .stage:nth-child(-n + 4) {
    border-bottom: 1px solid var(--line);
  }
  .control-copy {
    padding: 25px;
  }
  .footer-grid {
    gap: 25px;
  }
  .footer-bottom {
    flex-direction: column;
  }
  .flow-core {
    width: 150px;
  }
  .flow-lines {
    width: 31%;
  }
  .flow-output {
    right: 4%;
    gap: 7px;
  }
}
.brand-mark img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 5px;
}
html:not(.dark) .meteor-page {
  --bg: #f5f7fa;
  --panel: #ffffff;
  --line: rgba(15, 23, 42, 0.12);
  --txt: #101828;
  --dim: #475467;
  --muted: #667085;
  --green: #059669;
  --blue: #0284c7;
  --amber: #d97706;
  background: var(--bg);
  color: var(--txt);
}
html:not(.dark) .meteor-page .meteor-nav {
  background: rgba(255, 255, 255, 0.72);
}
html:not(.dark) .meteor-page .meteor-nav.scrolled {
  background: rgba(255, 255, 255, 0.92);
}
html:not(.dark) .meteor-page .obs-panel,
html:not(.dark) .meteor-page .pipeline,
html:not(.dark) .meteor-page .control-body,
html:not(.dark) .meteor-page .network-panel,
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
html:not(.dark) .meteor-page .globe {
  background: radial-gradient(circle at 52% 48%, rgba(16, 185, 129, 0.16), transparent 35%), radial-gradient(circle at 50% 50%, #e9f1f5, #f5f7fa 68%);
}
</style>
