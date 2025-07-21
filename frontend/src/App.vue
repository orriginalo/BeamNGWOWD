<script setup>
import { ref } from 'vue'
import { ProcessMods, SelectFolder } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'

const outputPath = ref('./downloaded')
const workStarted = ref(false)

const urls = ref(`https://www.worldofmods.com/beamng/cars/497-range-rover-sport.html
https://www.worldofmods.com/beamng/cars/105850-toyota-camry-xv70-2021.html`)

function selectFolder() {
  SelectFolder().then(result => {
    outputPath.value = result
  })
}

const mods = ref([])

function startDownload() {
  workStarted.value = true
  ProcessMods(urls.value.split("\n").filter(url => url.length > 0), outputPath.value)
}

EventsOn('progress', (data) => {
  try {
    const existing = mods.value.find(m => m.url === data.url)
    if (existing) {
      existing.progress = data.progress
    } 
    else mods.value.push({ url: data.url, progress: data.progress })
  } catch (error) {
    console.log(error)
  }
})

EventsOn('status', (data) => {
  const existing = mods.value.find(m => m.url === data.url)
  if (existing) {
    existing.status = data.status
  } 
  else mods.value.push({ url: data.url, status: data.status })
})

EventsOn('title', (data) => {
  const existing = mods.value.find(m => m.url === data.url)
  if (existing) {
    existing.title = data.title
  } 
  else mods.value.push({ url: data.url, title: data.title })
})

</script>


<template>
  <div class="app-container">
    <div class="layout">
      <!-- Left Panel -->
      <div class="left-panel">
        <h1>BeamNGWOMD</h1>
        <span>BeamNG World Of Mods Downloader</span>
        <button 
          class="button"
          :disabled="outputPathSelected"
          @click="selectFolder"
          >
          Выбрать папку
        </button>
        <input
          v-model="outputPath"
          placeholder="Введите путь к выходной папке"
          class="input"
        />
        <label class="label">Введите сюда URL модов</label>
        <textarea
          v-model="urls"
          rows="6"
          class="textarea"
        ></textarea>
        <button
          class="button"
          :disabled="workStarted"
          @click="startDownload"
        >
          Начать загрузку
        </button>
      </div>

      <!-- Right Panel -->
      <div class="right-panel">
        <div class="progress-section">
          <h2>Прогресс</h2>
          <div class="progress-area">
            <div
              v-for="mod in mods"
              :key="mod.url"
              class="progress-item"
            >
              <div class="progress-title">{{ mod.title }}</div>
              <div class="progress-status">{{ mod.status }}</div>
              <div class="progress-bar">
                <div
                  class="progress-fill"
                  :style="{ width: mod.progress + '%' }"
                >
                  <span class="progress-text">{{ Math.round(mod.progress) }}%</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
body {
  margin: 0;
  font-family: sans-serif;
  background-color: #1e1e1e;
  color: white;
}

.app-container {
  padding: 20px;
  min-height: 100vh;
  background-color: #1e1e1e;
  box-sizing: border-box;
  height: 100%;       /* добавь это */
  display: flex;
  flex-direction: column;
}

.layout {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 20px;
  height: 100%; 
}

.left-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.right-panel {
  flex: 2;
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: 100%;
  text-align: start;
}

.input, .textarea {
  background-color: #2a2a2a;
  border: 1px solid #444;
  padding: 8px;
  border-radius: 4px;
  color: white;
}

.textarea {
    height: 100%;
}

.button {
  background-color: #444; /* синий */
  color: white;
  padding: 10px 16px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: background-color 0.3s;
}

.button:hover {
  background-color: #666; /* чуть темнее при наведении */
}

/* Стили для disabled */
.button:disabled {
  background-color: #1a1a1a00; /* серая */
  color: #6b7280; /* тусклый текст */
  cursor: not-allowed; /* курсор "запрещено" */
  opacity: 0.8;
}

.label {
  font-size: 0.9em;
  margin-bottom: 4px;
}

.progress-section h2,
.logs-section h2 {
  margin-bottom: 8px;
}

.progress-area {
  /* background-color: #2a2a2a; */
  padding: 10px;
  border-radius: 4px;
  height: 100%;
  overflow-y: auto;
}

.progress-item {
  margin-bottom: 12px;
  padding: 12px 16px;
  border-radius: 8px;
  background-color: #292929; /* чуть светлее фон */
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
  /* убрал border-bottom */
}

.progress-status {
  font-size: 0.85em;
  color: #9ca3af; /* светло-серый, менее навязчивый */
  margin-bottom: 6px;
}


.progress-title {
  font-size: 1em;
  margin-bottom: 4px;
}

.progress-bar {
  position: relative;
  width: 100%;
  height: 24px;
  background-color: #e0e0e0;
  border-radius: 6px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background-color: #4caf50;
  transition: width 0.3s;
}

.progress-text {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: black; /* или white, если хочешь */
  pointer-events: none;
  font-size: 1.2rem;
}

.logs {
  background-color: #2a2a2a;
  padding: 10px;
  height: 140px;
  overflow-y: auto;
  white-space: pre-wrap;
  border-radius: 4px;
  border: 1px solid #444;
}
</style>
