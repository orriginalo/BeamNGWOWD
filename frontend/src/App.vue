<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import {
  CancelMod,
  ClearCompleted,
  DeleteInstalledMod,
  DetectGamePath,
  DisableMod,
  EnableMod,
  GetSettings,
  ListInstalledModRecords,
  GetModDetails,
  ListInstalledMods,
  PauseMod,
  QueueMods,
  ResumeMod,
  RemoveQueueItem,
  SaveSettings,
  SearchCatalog,
  SelectFolder,
  ValidateGamePath,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'

const tabs = [
  { id: 'discover', label: 'Каталог', icon: '⌕' },
  { id: 'library', label: 'Моды', icon: '▣' },
  { id: 'downloads', label: 'Загрузки', icon: '↓' },
  { id: 'settings', label: 'Настройки', icon: '⚙' },
]

const activeTab = ref('discover')
const outputPath = ref('./downloaded')
const gamePath = ref('')
const gamePathValid = ref(false)
const query = ref('')
const nextPage = ref(320)
const hasMore = ref(true)
const loadingCatalog = ref(false)
const catalogError = ref('')
const catalogMods = ref([])
const selectedCatalogUrls = ref(new Set())
const queue = ref([])
const installedMods = ref([])
const installedUrls = ref(new Set())
const quickView = ref(null)
const quickViewLoading = ref(false)
const quickViewError = ref('')
const quickViewIndex = ref(0)
const quickViewHoverX = ref(null)
const toasts = ref([])
const loadSentinel = ref(null)
const contentRoot = ref(null)
let searchTimer = 0
let settingsTimer = 0
let catalogFillTimer = 0
let catalogObserver

const activeDownloads = computed(() => queue.value.filter(item => !['Завершено.', 'Отменено', 'Ошибка'].includes(item.status)))
const completedDownloads = computed(() => queue.value.filter(item => item.status === 'Завершено.' || item.done))
const selectedCatalogMods = computed(() => catalogMods.value.filter(mod => selectedCatalogUrls.value.has(mod.url)))

function bytes(value) {
  if (!value) return '0 MB'
  const units = ['B', 'KB', 'MB', 'GB']
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index++
  }
  return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function eta(seconds) {
  if (!seconds) return '—'
  if (seconds < 60) return `${Math.ceil(seconds)} c`
  return `${Math.ceil(seconds / 60)} мин`
}

function notify(message, type = 'info') {
  const id = Date.now() + Math.random()
  toasts.value.push({ id, message, type })
  window.setTimeout(() => {
    toasts.value = toasts.value.filter(toast => toast.id !== id)
  }, 4200)
}

function upsertQueue(patch) {
  const existing = queue.value.find(item => item.url === patch.url)
  if (existing) Object.assign(existing, patch)
  else queue.value.unshift({ progress: 0, status: 'В очереди', ...patch })
}

function statusFor(url) {
  return queue.value.find(item => item.url === url)
}

function isInstalled(url) {
  return installedUrls.value.has(url) || completedDownloads.value.some(item => item.url === url)
}

function buttonLabel(mod) {
  const item = statusFor(mod.url)
  if (item && item.status !== 'Завершено.') return item.status || 'Скачивается'
  if (isInstalled(mod.url)) return 'Установлен'
  return 'Установить'
}

async function queueMods(mods) {
  const clean = mods.map(mod => ({
    url: mod.url,
    title: mod.title,
    thumbnailUrl: mod.imageUrl || mod.thumbnailUrl,
  })).filter(mod => mod.url && !installedUrls.value.has(mod.url))
  if (!clean.length) return
  clean.forEach(mod => upsertQueue({ url: mod.url, title: mod.title, thumbnailUrl: mod.thumbnailUrl, status: 'В очереди', progress: 0 }))
  await QueueMods(clean, outputPath.value)
  notify(`Добавлено в очередь: ${clean.length}`, 'success')
}

async function installMod(mod, event) {
  if (event) event.stopPropagation()
  await queueMods([mod])
}

function toggleCatalogMod(mod, event) {
  if (event) event.stopPropagation()
  const next = new Set(selectedCatalogUrls.value)
  if (next.has(mod.url)) next.delete(mod.url)
  else next.add(mod.url)
  selectedCatalogUrls.value = next
}

async function installSelected() {
  await queueMods(selectedCatalogMods.value)
  selectedCatalogUrls.value = new Set()
}

async function loadMoreCatalog(reset = false) {
  if (loadingCatalog.value || (!reset && !hasMore.value)) return
  loadingCatalog.value = true
  catalogError.value = ''
  try {
    const result = await SearchCatalog(query.value, reset ? 320 : nextPage.value, query.value.trim() ? 28 : 16)
    const incoming = Array.isArray(result.mods) ? result.mods : []
    const byUrl = new Map((reset ? [] : catalogMods.value).map(mod => [mod.url, mod]))
    incoming.forEach(mod => byUrl.set(mod.url, mod))
    catalogMods.value = Array.from(byUrl.values())
    nextPage.value = result.nextPage
    hasMore.value = Boolean(result.hasMore)
  } catch (error) {
    catalogError.value = 'Связь с сервером потеряна. Проверьте интернет или мы просто чиним трубы.'
  } finally {
    loadingCatalog.value = false
    await nextTick()
    ensureCatalogFilled()
  }
}

function ensureCatalogFilled() {
  window.clearTimeout(catalogFillTimer)
  if (!hasMore.value || loadingCatalog.value || activeTab.value !== 'discover') return
  catalogFillTimer = window.setTimeout(() => {
    if (!loadSentinel.value) return
    const rect = loadSentinel.value.getBoundingClientRect()
    if (rect.top <= window.innerHeight + 120) loadMoreCatalog(false)
  }, 80)
}

function onCatalogScroll() {
  if (activeTab.value !== 'discover') return
  ensureCatalogFilled()
}

function scheduleSearch() {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => loadMoreCatalog(true), 350)
}

async function openQuickView(mod) {
  quickView.value = { ...mod }
  quickViewLoading.value = true
  quickViewError.value = ''
  quickViewIndex.value = 0
  quickViewHoverX.value = null
  try {
    quickView.value = await GetModDetails(mod.url)
  } catch (error) {
    quickViewError.value = 'Не удалось загрузить детали мода.'
  } finally {
    quickViewLoading.value = false
  }
}

function closeQuickView() {
  quickView.value = null
  quickViewError.value = ''
  quickViewIndex.value = 0
  quickViewHoverX.value = null
}

function galleryImages(mod) {
  const images = Array.isArray(mod?.screenshots) ? mod.screenshots.filter(Boolean) : []
  if (images.length) return images
  if (mod?.imageUrl) return [mod.imageUrl]
  return []
}

function activeQuickImage() {
  const images = galleryImages(quickView.value)
  if (!images.length) return ''
  const index = Math.min(quickViewIndex.value, images.length - 1)
  return images[index]
}

function selectQuickImage(index) {
  quickViewIndex.value = index
}

function onQuickHeroMove(event) {
  const images = galleryImages(quickView.value)
  if (images.length < 2) return
  const rect = event.currentTarget.getBoundingClientRect()
  const x = event.clientX - rect.left
  const clamped = Math.max(0, Math.min(rect.width, x))
  const ratio = rect.width === 0 ? 0 : clamped / rect.width
  const index = Math.min(images.length - 1, Math.floor(ratio * images.length))
  quickViewHoverX.value = ratio
  quickViewIndex.value = index
}

function onQuickHeroLeave() {
  quickViewHoverX.value = null
}

async function selectFolder() {
  const result = await SelectFolder()
  if (result) {
    outputPath.value = result
    await saveSettings()
    notify('Папка установки обновлена', 'success')
    await refreshLibrary()
  }
}

async function refreshLibrary() {
  installedMods.value = await ListInstalledMods(outputPath.value)
}

async function loadSettings() {
  const settings = await GetSettings()
  if (settings.outputPath) outputPath.value = settings.outputPath
  if (settings.gamePath) {
    gamePath.value = settings.gamePath
    gamePathValid.value = await ValidateGamePath(gamePath.value)
  }
}

async function saveSettings() {
  await SaveSettings({
    outputPath: outputPath.value,
    gamePath: gamePath.value,
  })
}

function scheduleSaveSettings() {
  window.clearTimeout(settingsTimer)
  settingsTimer = window.setTimeout(() => saveSettings(), 350)
}

async function refreshInstalledRecords() {
  const records = await ListInstalledModRecords()
  installedUrls.value = new Set((records || []).map(record => record.url).filter(Boolean))
}

async function disableInstalled(mod) {
  await DisableMod(mod.path)
  notify('Мод выключен', 'success')
  await refreshLibrary()
}

async function enableInstalled(mod) {
  await EnableMod(mod.path)
  notify('Мод включён', 'success')
  await refreshLibrary()
}

async function deleteInstalled(mod) {
  await DeleteInstalledMod(mod.path)
  notify('Мод удалён', 'success')
  await refreshLibrary()
}

async function detectGamePath() {
  const detected = await DetectGamePath()
  gamePath.value = detected
  gamePathValid.value = Boolean(detected)
  await saveSettings()
  notify(detected ? 'Путь к игре найден' : 'Не удалось найти игру автоматически', detected ? 'success' : 'warning')
}

async function validateGamePath() {
  gamePathValid.value = await ValidateGamePath(gamePath.value)
  scheduleSaveSettings()
}

function pause(item) {
  PauseMod(item.url)
  upsertQueue({ url: item.url, status: 'Пауза' })
}

function resume(item) {
  ResumeMod(item.url)
  upsertQueue({ url: item.url, status: 'Продолжаем...' })
}

function cancel(item) {
  CancelMod(item.url)
  upsertQueue({ url: item.url, status: 'Отменено' })
}

function removeItem(item) {
  RemoveQueueItem(item.url)
  queue.value = queue.value.filter(q => q.url !== item.url)
}

function clearCompleted() {
  ClearCompleted()
  queue.value = queue.value.filter(item => item.status !== 'Завершено.' && !item.done)
}

EventsOn('queue:item', data => upsertQueue(data))
EventsOn('queue:done', data => {
  upsertQueue({ url: data.url, status: 'Завершено.', progress: 100, done: true })
  installedUrls.value = new Set([...installedUrls.value, data.url])
  notify(`Мод ${statusFor(data.url)?.title || data.url} успешно установлен!`, 'success')
  refreshLibrary()
  refreshInstalledRecords()
})
EventsOn('queue:error', data => notify(data.error || 'Ошибка загрузки', 'error'))
EventsOn('queue:cancelled', data => upsertQueue({ url: data.url, status: 'Отменено' }))

onMounted(async () => {
  await loadSettings()
  await refreshInstalledRecords()
  loadMoreCatalog(true)
  refreshLibrary()
  catalogObserver = new IntersectionObserver(entries => {
    if (entries.some(entry => entry.isIntersecting)) loadMoreCatalog(false)
  }, { rootMargin: '280px' })
  if (loadSentinel.value) catalogObserver.observe(loadSentinel.value)
  window.addEventListener('scroll', onCatalogScroll, { passive: true })
  contentRoot.value?.addEventListener('scroll', onCatalogScroll, { passive: true })
  window.addEventListener('keydown', event => {
    if (event.key === 'Escape') closeQuickView()
  })
})

onUnmounted(() => {
  window.clearTimeout(searchTimer)
  window.clearTimeout(settingsTimer)
  window.clearTimeout(catalogFillTimer)
  if (catalogObserver) catalogObserver.disconnect()
  window.removeEventListener('scroll', onCatalogScroll)
  contentRoot.value?.removeEventListener('scroll', onCatalogScroll)
})
</script>

<template>
  <main class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark">B</span>
        <div>
          <strong>BeamNGWOWD</strong>
          <small>Менеджер модов</small>
        </div>
      </div>
      <nav>
        <button v-for="tab in tabs" :key="tab.id" :class="{ active: activeTab === tab.id }" @click="activeTab = tab.id">
          <span>{{ tab.icon }}</span>{{ tab.label }}
        </button>
      </nav>
    </aside>

    <section class="content" ref="contentRoot">
      <header class="page-header">
        <div>
          <p class="eyebrow">BeamNGWOWD</p>
          <h1>{{ tabs.find(tab => tab.id === activeTab)?.label }}</h1>
        </div>
        <p class="header-note">Активных загрузок: {{ activeDownloads.length }}</p>
      </header>

      <section v-if="activeTab === 'discover'" class="page">
        <div class="toolbar">
          <input v-model="query" class="search" placeholder="Поиск Mercedes, BMW, Toyota..." @input="scheduleSearch" />
          <button class="accent" :disabled="selectedCatalogMods.length === 0" @click="installSelected">В очередь: {{ selectedCatalogMods.length }}</button>
        </div>

        <div v-if="catalogError" class="empty error">
          <h3>{{ catalogError }}</h3>
          <button class="accent" @click="loadMoreCatalog(true)">Повторить</button>
        </div>

        <div class="mod-grid">
          <article v-for="mod in catalogMods" :key="mod.url" class="mod-card" @click="openQuickView(mod)">
            <div class="cover">
              <img :src="mod.imageUrl" :alt="mod.title" loading="lazy" />
              <div class="cover-gradient"></div>
              <h3 class="cover-title">{{ mod.title }}</h3>
              <label class="check" @click.stop>
                <input type="checkbox" :checked="selectedCatalogUrls.has(mod.url)" @change="toggleCatalogMod(mod, $event)" />
                <span></span>
              </label>
            </div>
            <div class="card-body">
              <div class="meta simple">
                <span>{{ mod.author || 'WorldOfMods' }}</span>
                <span>{{ mod.downloads || '0' }} загрузок</span>
                <span>{{ mod.published || `страница ${mod.page}` }}</span>
              </div>
              <button class="accent card-install" :disabled="isInstalled(mod.url)" @click="installMod(mod, $event)">
                <span v-if="statusFor(mod.url) && statusFor(mod.url).status !== 'Завершено.'">
                  {{ Math.round(statusFor(mod.url).progress || 0) }}%
                </span>
                <span v-else>{{ buttonLabel(mod) }}</span>
              </button>
            </div>
          </article>

          <article v-if="loadingCatalog" v-for="n in 8" :key="n" class="mod-card skeleton">
            <div class="cover"></div>
            <div class="card-body"><i></i><b></b><b></b></div>
          </article>
        </div>

        <div class="load-more">
          <span>{{ loadingCatalog ? 'Загрузка...' : hasMore ? 'Листай ниже, моды подгрузятся автоматически' : 'Это всё' }}</span>
        </div>
        <div ref="loadSentinel" class="sentinel"></div>
      </section>

      <section v-if="activeTab === 'downloads'" class="page">
        <div class="downloads-head">
          <h2>Очередь загрузок</h2>
          <button class="tonal" @click="clearCompleted">Очистить завершённые</button>
        </div>
        <article v-for="item in queue" :key="item.url" class="download-row">
          <img :src="item.thumbnailUrl" alt="" />
          <div>
            <strong>{{ item.title || item.url }}</strong>
            <span>{{ item.status || 'В очереди' }}</span>
            <div class="progress"><i :style="{ width: `${Math.min(100, item.progress || 0)}%` }"></i></div>
            <small>{{ Math.round(item.progress || 0) }}% · {{ bytes(item.downloadedBytes) }} / {{ bytes(item.totalBytes) }} · {{ bytes(item.speedBytesPerSec) }}/с · Осталось: {{ eta(item.etaSeconds) }}</small>
          </div>
          <div class="row-actions">
            <button v-if="item.status !== 'Пауза' && item.status !== 'Завершено.' && item.status !== 'Отменено'" class="ghost" @click="pause(item)">Пауза</button>
            <button v-if="item.status === 'Пауза'" class="ghost" @click="resume(item)">Продолжить</button>
            <button v-if="item.status !== 'Завершено.' && item.status !== 'Отменено'" class="ghost danger" @click="cancel(item)">Отменить</button>
            <button v-if="item.status === 'Завершено.' || item.status === 'Отменено' || item.status === 'Ошибка'" class="ghost" @click="removeItem(item)">Убрать</button>
          </div>
        </article>
        <div v-if="queue.length === 0" class="empty">Очередь пустая. Открой каталог и добавь нужные моды.</div>
      </section>

      <section v-if="activeTab === 'library'" class="page">
        <div class="downloads-head">
          <h2>Мои моды</h2>
          <button class="tonal" @click="refreshLibrary">Обновить</button>
        </div>
        <article v-for="mod in installedMods" :key="mod.path" class="library-row">
          <div>
            <strong>{{ mod.name }}</strong>
            <span>{{ bytes(mod.size) }} · {{ mod.disabled ? 'выключен' : 'включен' }}</span>
          </div>
          <button class="ghost" @click="mod.disabled ? enableInstalled(mod) : disableInstalled(mod)">{{ mod.disabled ? 'Включить' : 'Выключить' }}</button>
          <button class="ghost danger" @click="deleteInstalled(mod)">Удалить</button>
        </article>
        <div v-if="installedMods.length === 0" class="empty">Пока нет `processed-*.zip` в выбранной папке.</div>
      </section>

      <section v-if="activeTab === 'settings'" class="page settings">
        <div class="settings-card">
          <h2>Путь установки</h2>
          <p>Сюда BeamNGWOWD кладёт обработанные архивы.</p>
          <div class="path-row">
            <input v-model="outputPath" @input="scheduleSaveSettings" />
            <button class="accent" @click="selectFolder">Обзор</button>
          </div>
        </div>
        <div class="settings-card">
          <h2>Путь к игре</h2>
          <p>Нужен для будущей интеграции с корневой папкой BeamNG.</p>
          <div class="path-row">
            <input v-model="gamePath" @input="validateGamePath" placeholder="C:\Program Files (x86)\Steam\steamapps\common\BeamNG.drive" />
            <button class="tonal" @click="detectGamePath">Найти автоматически</button>
          </div>
          <span :class="['validation', gamePathValid ? 'ok' : 'warn']">
            {{ gamePathValid ? 'Похоже, это BeamNG.drive' : 'Кажется, это не та папка. Убедитесь, что выбрали корневую папку игры' }}
          </span>
        </div>
      </section>
    </section>

    <div v-if="quickView" class="modal-backdrop" @click="closeQuickView">
      <article class="quick-modal" @click.stop>
        <button class="close" @click="closeQuickView">×</button>
        <div class="quick-hero" @mousemove="onQuickHeroMove" @mouseleave="onQuickHeroLeave">
          <img :src="activeQuickImage()" alt="" />
          <div class="quick-hero-gradient"></div>
          <h2 class="quick-hero-title">{{ quickView.title }}</h2>
          <div class="quick-indicator" v-if="galleryImages(quickView).length > 1">
            <i
              v-for="(shot, idx) in galleryImages(quickView)"
              :key="`${shot}-${idx}`"
              :class="{ active: idx === quickViewIndex }"
              @click="selectQuickImage(idx)"
            ></i>
          </div>
        </div>
        <div class="quick-content">
          <p class="eyebrow">Карточка мода</p>
          <p v-if="quickViewLoading">Загружаю детали...</p>
          <p v-else-if="quickViewError">{{ quickViewError }}</p>
          <p v-else>{{ quickView.description }}</p>
          <div class="meta modal-meta">
              <span>{{ quickView.author || 'WorldOfMods' }}</span>
              <span>{{ quickView.published || '—' }}</span>
              <span>{{ quickView.downloads || '0' }} загрузок</span>
          </div>
          <div class="screenshots">
            <button
              v-for="(shot, idx) in galleryImages(quickView)"
              :key="`${shot}-${idx}`"
              :class="['shot-thumb', { active: idx === quickViewIndex }]"
              @click="selectQuickImage(idx)"
            >
              <img :src="shot" alt="" />
            </button>
          </div>
          <button class="accent wide" @click="installMod(quickView, $event)">{{ buttonLabel(quickView) }}</button>
        </div>
      </article>
    </div>

    <div class="toasts">
      <div v-for="toast in toasts" :key="toast.id" :class="['toast', toast.type]">{{ toast.message }}</div>
    </div>
  </main>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 230px minmax(0, 1fr);
  font-family: "Manrope", "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  background: radial-gradient(circle at 20% 0%, rgba(255, 116, 38, 0.15), transparent 34rem), #0c0d10;
  color: #f4f5f7;
}

.sidebar {
  position: sticky;
  top: 0;
  height: 100vh;
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 20px;
  box-sizing: border-box;
  border-right: 1px solid #20232a;
  background: #111318;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand-mark {
  display: grid;
  width: 42px;
  height: 42px;
  place-items: center;
  border-radius: 8px;
  background: #ff7426;
  color: #14100c;
  font-weight: 900;
}

.brand strong,
.brand small,
.download-row strong,
.download-row span,
.library-row strong,
.library-row span {
  display: block;
}

.brand small,
.meta,
.download-row span,
.download-row small,
.library-row span,
.settings-card p,
.validation {
  color: #8e96a3;
}

nav {
  display: grid;
  gap: 8px;
}

nav button,
button,
select,
input {
  font: inherit;
}

nav button {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  border: 0;
  border-radius: 8px;
  padding: 0 12px;
  background: transparent;
  color: #b5bcc8;
  cursor: pointer;
  text-align: left;
}

nav button.active,
nav button:hover {
  background: #1b1f27;
  color: #fff;
}

.content {
  padding: 20px;
}

.page-header,
.toolbar,
.downloads-head,
.path-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.eyebrow {
  margin: 0 0 6px;
  color: #ff9b61;
  font-size: 0.74rem;
  font-weight: 800;
  text-transform: uppercase;
}

h1,
h2,
h3,
p {
  margin-top: 0;
}

h1 {
  margin-bottom: 0;
  font-size: 2.2rem;
}

.header-note {
  margin: 0;
  color: #8e96a3;
}

.page {
  margin-top: 16px;
}

.search,
.path-row input,
.settings-card input,
select {
  min-height: 46px;
  border: 1px solid #2a303a;
  border-radius: 8px;
  box-sizing: border-box;
  outline: none;
  background: #111318;
  color: #f4f5f7;
}

.search {
  flex: 1;
  padding: 0 16px;
  font-size: 1.05rem;
}

.toolbar {
  flex-wrap: wrap;
  justify-content: flex-start;
}

.toolbar .accent {
  margin-left: auto;
}

select {
  padding: 0 12px;
}

button {
  min-height: 42px;
  border: 0;
  border-radius: 8px;
  padding: 0 14px;
  font-weight: 800;
  cursor: pointer;
  transition: transform 0.16s, box-shadow 0.16s, background-color 0.16s;
}

button:hover:not(:disabled) {
  transform: translateY(-1px) scale(1.01);
}

button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.accent {
  background: #ff7426;
  color: #160c06;
  box-shadow: 0 0 22px rgba(255, 116, 38, 0.22);
}

.tonal,
.ghost {
  background: #202630;
  color: #f2f4f7;
}

.ghost {
  border: 1px solid #323945;
  background: transparent;
}

.danger {
  color: #ff9184;
}

.mod-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 14px;
  margin-top: 14px;
}

.mod-card {
  overflow: hidden;
  border: 1px solid #232832;
  border-radius: 12px;
  background: #14171d;
  cursor: pointer;
  transition: transform 0.18s, border-color 0.18s, box-shadow 0.18s;
}

.mod-card:hover {
  transform: translateY(-2px);
  border-color: rgba(255, 116, 38, 0.35);
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.24);
}

.cover {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  margin: 10px 10px 0;
  border-radius: 10px;
  background: #222833;
}

.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-gradient {
  position: absolute;
  inset: auto 0 0;
  height: 56%;
  background: linear-gradient(to top, rgba(8, 10, 14, 0.92), rgba(8, 10, 14, 0.34), transparent);
  pointer-events: none;
}

.cover-title {
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: 10px;
  margin: 0;
  font-size: 0.95rem;
  line-height: 1.25;
  color: #f3f5f7;
  text-shadow: 0 2px 10px rgba(0, 0, 0, 0.65);
  display: -webkit-box;
  overflow: hidden;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.check {
  position: absolute;
  top: 10px;
  right: 10px;
}

.check input {
  display: none;
}

.check span {
  display: block;
  width: 24px;
  height: 24px;
  border: 2px solid #fff;
  border-radius: 7px;
  background: rgba(0, 0, 0, 0.4);
}

.check input:checked + span {
  border-color: #ff7426;
  background: #ff7426;
  box-shadow: inset 0 0 0 5px #14171d;
}

.card-body {
  padding: 10px 12px 12px;
}

.card-body h3 {
  display: -webkit-box;
  min-height: 2.5em;
  overflow: hidden;
  font-size: 0.97rem;
  line-height: 1.3;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 10px;
  font-size: 0.76rem;
}

.meta.simple {
  gap: 4px 10px;
}

.meta.simple span:last-child {
  display: none;
}

.card-install {
  width: 100%;
  margin-top: 10px;
  background: #f3f5f7;
  color: #111318;
  box-shadow: none;
}

.card-install:hover:not(:disabled) {
  background: #ffffff;
}

.meta span {
  padding: 0;
  border-radius: 0;
  background: transparent;
}

.progress {
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: #2a303a;
}

.progress i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #ff7426, #ffd166);
}

.download-row,
.library-row,
.settings-card,
.empty {
  display: grid;
  gap: 12px;
  margin-top: 12px;
  padding: 16px;
  border: 1px solid #232832;
  border-radius: 12px;
  background: #151820;
}

.download-row {
  grid-template-columns: 80px minmax(0, 1fr) auto;
  align-items: center;
}

.download-row img {
  width: 80px;
  height: 50px;
  border-radius: 8px;
  object-fit: cover;
  background: #222833;
}

.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: end;
}

.library-row {
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
}

.settings {
  display: grid;
  gap: 14px;
  max-width: 760px;
}

.path-row input {
  flex: 1;
  padding: 0 14px;
}

.validation.ok {
  color: #78d99b;
}

.validation.warn {
  color: #ffd166;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 20;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(0, 0, 0, 0.68);
}

.quick-modal {
  position: relative;
  width: min(1320px, 100%);
  max-height: 92vh;
  display: grid;
  grid-template-columns: minmax(0, 1.8fr) minmax(360px, 0.9fr);
  overflow: auto;
  border: 1px solid #343a46;
  border-radius: 12px;
  background: #151820;
}

.quick-modal::-webkit-scrollbar,
.quick-content::-webkit-scrollbar {
  width: 10px;
}

.quick-modal::-webkit-scrollbar-track,
.quick-content::-webkit-scrollbar-track {
  background: #141820;
}

.quick-modal::-webkit-scrollbar-thumb,
.quick-content::-webkit-scrollbar-thumb {
  border: 2px solid #141820;
  border-radius: 999px;
  background: linear-gradient(180deg, #ff914d, #ff7426);
}

.quick-modal,
.quick-content {
  scrollbar-color: #ff7f35 #141820;
  scrollbar-width: thin;
}

.quick-hero img,
.screenshots img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.quick-hero {
  position: relative;
  min-height: 560px;
  background: #111318;
}

.quick-hero-gradient {
  position: absolute;
  inset: auto 0 0;
  height: 42%;
  background: linear-gradient(to top, rgba(8, 10, 14, 0.92), rgba(8, 10, 14, 0.2), transparent);
  pointer-events: none;
}

.quick-hero-title {
  position: absolute;
  left: 20px;
  right: 20px;
  bottom: 44px;
  margin: 0;
  font-size: 1.5rem;
  line-height: 1.2;
  text-shadow: 0 2px 12px rgba(0, 0, 0, 0.6);
}

.quick-indicator {
  position: absolute;
  left: 20px;
  right: 20px;
  bottom: 18px;
  display: flex;
  gap: 7px;
}

.quick-indicator i {
  flex: 1;
  height: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.25);
  cursor: pointer;
  transition: background-color 0.2s ease;
}

.quick-indicator i.active {
  background: #ff8d4a;
}

.quick-content {
  max-height: 92vh;
  overflow: auto;
  padding: 20px;
}

.close {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 2;
  width: 38px;
  padding: 0;
  background: rgba(0, 0, 0, 0.55);
  color: white;
}

.screenshots {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  margin: 16px 0;
}

.shot-thumb {
  overflow: hidden;
  border: 1px solid #2c3340;
  border-radius: 8px;
  padding: 0;
  background: #171b22;
}

.shot-thumb.active {
  border-color: #ff8740;
}

.screenshots img {
  aspect-ratio: 16 / 10;
  border-radius: 7px;
  object-fit: cover;
}

.wide {
  width: 100%;
}

.toasts {
  position: fixed;
  right: 22px;
  bottom: 22px;
  z-index: 30;
  display: grid;
  gap: 10px;
}

.toast {
  min-width: 260px;
  padding: 12px 14px;
  border: 1px solid #343a46;
  border-radius: 8px;
  background: #171a21;
  box-shadow: 0 18px 38px rgba(0, 0, 0, 0.35);
}

.toast.success {
  border-color: rgba(120, 217, 155, 0.5);
}

.toast.error {
  border-color: rgba(255, 145, 132, 0.5);
}

.skeleton .cover,
.skeleton i,
.skeleton b {
  display: block;
  border-radius: 8px;
  background: linear-gradient(90deg, #1d222b, #2a303a, #1d222b);
  animation: pulse 1.1s infinite linear;
}

.skeleton i,
.skeleton b {
  height: 16px;
}

.load-more {
  display: flex;
  justify-content: center;
  padding: 22px 0;
}

.sentinel {
  height: 1px;
}

@keyframes pulse {
  0% { opacity: 0.55; }
  50% { opacity: 1; }
  100% { opacity: 0.55; }
}

@media (max-width: 920px) {
  .app-shell {
    grid-template-columns: 1fr;
  }

  .sidebar {
    position: static;
    height: auto;
  }

  .toolbar .accent {
    margin-left: 0;
  }

  .quick-modal,
  .download-row {
    grid-template-columns: 1fr;
  }

  .quick-hero {
    min-height: 320px;
  }

  .quick-hero-title {
    font-size: 1.1rem;
    bottom: 36px;
  }

  .quick-indicator {
    bottom: 14px;
  }
}
</style>
