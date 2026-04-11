<script setup lang="ts">
import { ref, reactive, computed, shallowRef } from 'vue'
import { Chart, registerables } from 'chart.js'
import SettingInput from './components/SettingInput.vue'
import SettingToggle from './components/SettingToggle.vue'
import LanguageSwitcher from './components/LanguageSwitcher.vue'
import { useI18n } from 'vue-i18n'


Chart.register(...registerables)

const { t } = useI18n()
const API_BASE_URL = 'http://localhost:8080'
const WS_BASE_URL = 'ws://localhost:8080'

interface ScheduleItem {
  subject: string; instructor: string; room: string; day: string; time: string; groups: string[]
}

const getDefaultParams = () => ({
  main_options: {
    population_size: 150,
    generations: 500,
    mutation_rate: 0.001,
  },
  additional_options: {
    elitism: 0.05,
    tournament_size: 3,
    is_soft_mutation_enabled: false,
    soft_mutation_rate: 0.10,
    soft_mutation_attempts: 10,
    heat_stagnant_count: 20,
    heat_step_scale: 0.1,
    shock_stagnant_count: 80,
    shock_mutation_rate: 0.05,
    shock_min_recovery_count: 20,
    shock_recovery_scale: 0.05,
  }
})

// Инициализируем
const params = ref(getDefaultParams())

// Функция сброса
const resetParams = () => {
  params.value = getDefaultParams()
}

const scrollBarParams = {
  min: 50,
  max: 800,
  step: 10,
}

const calculateValue = (x: number) => {
  if (!x) return 0;
  // Формула: 0.5 * (x в степени 1.4)
  return Math.round(0.5 * Math.pow(x, 1.4));
};

// Функция автоматического расчета поколений (в 3 раза больше популяции)
const updateGenerations = () => {
    params.value.main_options.generations = calculateValue(params.value.main_options.population_size);
};

const isSettingsOpen = ref(false) // Состояние боковой панели
const expandedSections = reactive({
  all: false,
  main: true,      // Базовые открыты по умолчанию
  advanced: false, // Продвинутые скрыты
  rules: false     // Правила скрыты
})
const isAdvancedUnlocked = ref(false); // Флаг разблокировки экспертных настроек
const isGenerating = ref(false)
const loadingText = ref('Оптимизация расписания...')
const rawSchedule = ref<ScheduleItem[]>([])
const stats = reactive({ fitness: 0, hard_conflicts: 0, time: 0, algo: '', show: false })

const chartCanvas = ref<HTMLCanvasElement | null>(null)
const chartInstance = shallowRef<Chart | null>(null)

const initChart = () => {
  if (chartInstance.value) chartInstance.value.destroy()
  if (!chartCanvas.value) return
  chartInstance.value = new Chart(chartCanvas.value, {
    type: 'line',
    data: { labels: [], datasets: [{ label: 'Best Fitness', data: [], borderColor: '#4f46e5', backgroundColor: 'rgba(79, 70, 229, 0.1)', borderWidth: 2, fill: true, tension: 0.3, pointRadius: 0 }] },
    options: { responsive: true, maintainAspectRatio: false, animation: false, scales: { y: { beginAtZero: false, grid: { color: '#f1f5f9' } }, x: { display: false } }, plugins: { legend: { display: false } } }
  })
}

const generateGreedy = async () => {
  isGenerating.value = true; stats.show = false; rawSchedule.value = []
  loadingText.value = '⚡ Работает жадный алгоритм...'
  isSettingsOpen.value = false // Закрываем настройки при старте

  try {
    const response = await fetch(`${API_BASE_URL}/api/v1/schedule/generate/greedy`, {
      method: 'POST',
      // headers: { 'Content-Type': 'application/json' },
      // body: JSON.stringify(params)
    })
    const data = await response.json()
    stats.fitness = data.fitness_score; stats.hard_conflicts = data.hard_conflicts; stats.time = data.time_taken_ms; stats.algo = t('stats.algorithm_type.greedy'); stats.show = true
    rawSchedule.value = data.schedule
  } catch (e) { alert("Ошибка: " + e) }
  finally { isGenerating.value = false }
}

const generateGenetic = () => {
  isGenerating.value = true; stats.show = false; rawSchedule.value = []
  loadingText.value = '🧬 Инициализация эволюции...'
  isSettingsOpen.value = false

  setTimeout(() => initChart(), 0)

  const socket = new WebSocket(`${WS_BASE_URL}/api/v1/schedule/generate/genetic/ws`)

  socket.onopen = () => socket.send(JSON.stringify(params.value))
  socket.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    if (msg.type === 'progress') {
      loadingText.value = `🧬 Поколение ${msg.gen}: Фитнес ${msg.fitness.toFixed(4)}`
      stats.fitness = msg.fitness
      if (chartInstance.value && chartInstance.value.data.datasets[0]) {
        chartInstance.value.data.labels?.push(msg.gen)
        chartInstance.value.data.datasets[0].data.push(msg.fitness)
        chartInstance.value.update('none')
      }
    }
    if (msg.type === 'final') {
      stats.fitness = msg.fitness; stats.hard_conflicts = msg.hard_conflicts; stats.time = msg.time_taken_ms; stats.algo = t('stats.algorithm_type.genetic'); stats.show = true
      rawSchedule.value = msg.schedule; isGenerating.value = false
      socket.close()
    }
  }
  socket.onerror = (err) => { alert("WebSocket error"); console.error(err); isGenerating.value = false }
}

const daysOrder = ["monday", "tuesday", "wednesday", "thursday", "friday", "saturday"]

const scheduleMatrix = computed(() => {
  if (rawSchedule.value.length === 0) return null
  const groupsSet = new Set<string>(); const timesSet = new Set<string>()
  const matrix: Record<string, Record<string, Record<string, ScheduleItem[]>>> = {}

  rawSchedule.value.forEach(item => {
    timesSet.add(item.time)
    if (!matrix[item.day]) matrix[item.day] = {}
    if (!matrix[item.day][item.time]) matrix[item.day][item.time] = {}
    item.groups.forEach(g => {
      groupsSet.add(g)
      if (!matrix[item.day][item.time][g]) matrix[item.day][item.time][g] = []
      matrix[item.day][item.time][g].push(item)
    })
  })
  return { groups: Array.from(groupsSet).sort(), times: Array.from(timesSet).sort(), data: matrix }
})
</script>

<template>
  <div class="bg-gray-50 min-h-screen text-gray-800 font-sans pb-10 relative overflow-x-hidden">

    <!-- Главный контейнер -->
    <div class="mx-auto px-4 py-8 w-[96%] max-w-[1600px] transition-all duration-300" :class="{ 'pr-96': isSettingsOpen }">

      <!-- Header -->
      <header class="flex flex-col md:flex-row justify-between items-start md:items-center mb-6 bg-white p-6 rounded-xl shadow-sm border border-gray-100 gap-4">
        <div>
          <h1 class="text-3xl font-extrabold text-indigo-600 tracking-tight">{{ t('header.title') }}</h1>
          <p class="text-sm text-gray-500 mt-1">{{ t('header.subtitle') }}</p>
        </div>

        <div class="flex gap-3 items-center">
          <button @click="generateGreedy" :disabled="isGenerating" class="px-5 py-2.5 bg-gray-800 hover:bg-gray-900 text-white font-medium rounded-lg shadow-sm disabled:opacity-50">
            {{ t('header.btn_greedy') }}
          </button>
          <button @click="generateGenetic" :disabled="isGenerating" class="px-5 py-2.5 bg-indigo-600 hover:bg-indigo-700 text-white font-medium rounded-lg shadow-sm disabled:opacity-50">
            {{ t('header.btn_genetic') }}
          </button>

          <LanguageSwitcher />

          <!-- Кнопка Настроек (Шестеренка) -->
          <button @click="isSettingsOpen = true" class="p-2.5 bg-white border border-gray-200 hover:bg-gray-100 text-gray-600 rounded-lg shadow-sm transition-colors">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-6 h-6">
              <path stroke-linecap="round" stroke-linejoin="round" d="M10.343 3.94c.09-.542.56-.94 1.11-.94h1.093c.55 0 1.02.398 1.11.94l.149.894c.07.424.384.764.78.93.398.164.855.142 1.205-.108l.737-.527a1.125 1.125 0 0 1 1.45.12l.773.774c.39.389.44 1.002.12 1.45l-.527.737c-.25.35-.272.806-.107 1.204.165.397.505.71.93.78l.893.15c.543.09.94.559.94 1.109v1.094c0 .55-.397 1.02-.94 1.11l-.894.149c-.424.07-.764.383-.929.78-.165.398-.143.854.107 1.204l.527.738c.32.447.269 1.06-.12 1.45l-.774.773a1.125 1.125 0 0 1-1.449.12l-.738-.527c-.35-.25-.806-.272-1.203-.107-.398.165-.71.505-.78.929l-.15.894c-.09.542-.56.94-1.11.94h-1.094c-.55 0-1.019-.398-1.11-.94l-.148-.894c-.071-.424-.384-.764-.781-.93-.398-.164-.854-.142-1.204.108l-.738.527c-.447.32-1.06.269-1.45-.12l-.773-.774a1.125 1.125 0 0 1-.12-1.45l.527-.737c.25-.35.272-.806.108-1.204-.165-.397-.506-.71-.93-.78l-.894-.15c-.542-.09-.94-.56-.94-1.109v-1.094c0-.55.398-1.02.94-1.11l.894-.149c.424-.07.765-.383.93-.78.165-.398.143-.854-.108-1.204l-.526-.738a1.125 1.125 0 0 1 .12-1.45l.773-.773a1.125 1.125 0 0 1 1.45-.12l.737.527c.35.25.807.272 1.204.107.397-.165.71-.505.78-.929l.15-.894Z" />
              <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
            </svg>
          </button>
        </div>
      </header>

      <!-- Stats Panel -->
      <div v-if="stats.show" class="mb-6 grid grid-cols-1 md:grid-cols-3 gap-4">
          <div class="bg-white p-4 rounded-xl shadow-sm border border-gray-100 text-center transition-all">
            <!-- Состояние: ЕСТЬ КОНФЛИКТЫ -->
            <template v-if="stats.hard_conflicts > 0">
              <p class="text-sm font-semibold text-red-500 uppercase tracking-wider">
                {{ t('stats.conflicts_found') }}
              </p>
              <div class="mt-1">
                <p class="text-3xl font-extrabold text-red-600">
                  {{ stats.hard_conflicts }}
                </p>
              </div>
            </template>

            <!-- Состояние: ВСЕ ЧИСТО -->
            <template v-else>
              <p class="text-sm font-semibold text-gray-400 uppercase tracking-wider">
                {{ t('stats.fitness') }}
              </p>
              <p class="text-3xl font-extrabold mt-1 text-green-600">
                {{ (stats.fitness * 100).toFixed(2) }}%
              </p>
            </template>
          </div>
        <div class="bg-white p-4 rounded-xl shadow-sm border border-gray-100 text-center">
          <p class="text-sm font-semibold text-gray-400 uppercase tracking-wider">{{ t('stats.time') }}</p>
          <p class="text-3xl font-extrabold text-gray-800 mt-1">{{ stats.time }} {{ t('stats.time_unit') }}</p>
        </div>
        <div class="bg-white p-4 rounded-xl shadow-sm border border-gray-100 text-center">
          <p class="text-sm font-semibold text-gray-400 uppercase tracking-wider">{{ t('stats.algorithm') }}</p>
          <p class="text-3xl font-extrabold text-indigo-600 mt-1">{{ stats.algo }}</p>
        </div>
      </div>

      <!-- Loading & Chart -->
      <div v-show="isGenerating" class="flex flex-col items-center py-10 bg-white rounded-xl shadow-sm border border-gray-100 mb-6">
        <div class="loader ease-linear rounded-full border-4 border-gray-200 h-16 w-16 mb-6"></div>
        <p class="text-gray-500 font-medium animate-pulse text-lg mb-6">{{ loadingText }}</p>
        <div class="w-full max-w-4xl px-4 h-[200px]" :class="{ 'hidden': loadingText.includes('Жадный') }">
          <canvas ref="chartCanvas"></canvas>
        </div>
      </div>

      <!-- Matrix Table -->
      <div v-if="scheduleMatrix && !isGenerating" class="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden table-container overflow-x-auto">
         <table class="w-full text-left border-collapse min-w-[800px]">
          <thead class="bg-gray-100 text-gray-700 text-sm tracking-wider border-b-2 border-gray-300">
            <tr>
              <th class="p-4 w-32 border-r border-gray-200 bg-gray-100 sticky left-0 z-10">{{ t('matrix.time_slots') }}</th>
              <th v-for="group in scheduleMatrix.groups" :key="group" class="p-4 border-r border-gray-200 text-center font-bold text-indigo-900">{{ group }}</th>
            </tr>
          </thead>
          <tbody class="text-sm divide-y divide-gray-200">
            <template v-for="day in daysOrder" :key="day">
              <template v-if="scheduleMatrix.data[day]">
                <tr class="bg-indigo-50 border-y-2 border-indigo-100">
                  <td :colspan="scheduleMatrix.groups.length + 1" class="p-3 text-center font-bold text-indigo-700 uppercase tracking-widest sticky left-0">{{ t(`days.${day}`) }}</td>
                </tr>
                <tr v-for="time in scheduleMatrix.times" :key="time" class="hover:bg-gray-50 transition-colors">
                  <template v-if="scheduleMatrix.data[day][time]">
                    <td class="p-3 border-r border-gray-200 font-medium text-gray-600 bg-white sticky left-0 shadow-[2px_0_5px_-2px_rgba(0,0,0,0.1)] text-center whitespace-nowrap">{{ time }}</td>
                    <td v-for="group in scheduleMatrix.groups" :key="group" class="p-2 border-r border-gray-200 align-top w-48 min-w-[12rem]">
                      <template v-if="scheduleMatrix.data[day][time][group]">
                        <div v-if="scheduleMatrix.data[day][time][group].length > 1" class="text-xs font-bold text-red-600 mb-1 text-center bg-red-100 rounded">{{ t('matrix.collision') }}</div>
                        <div v-for="(item, idx) in scheduleMatrix.data[day][time][group]" :key="idx"
                             class="border rounded-md p-2 h-full shadow-sm flex flex-col justify-between mb-1"
                             :class="[ scheduleMatrix.data[day][time][group].length > 1 ? 'bg-red-50 border-red-300' : (item.groups.length > 1 ? 'bg-blue-50 border-blue-200' : 'bg-green-50 border-green-200') ]">
                          <div class="font-bold text-gray-800 text-[13px] leading-tight mb-1">{{ item.subject }}</div>
                          <div class="text-xs text-gray-600 mb-2">{{ item.instructor }}</div>
                          <div class="text-[11px] font-mono bg-white px-1.5 py-0.5 rounded text-gray-500 inline-block self-start border border-gray-100">📍 {{ item.room }}</div>
                        </div>
                      </template>
                      <template v-else><div class="w-full h-full bg-gray-50/50"></div></template>
                    </td>
                  </template>
                </tr>
              </template>
            </template>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Затемнение фона при открытом меню (на мобилках) -->
    <div v-if="isSettingsOpen" @click="isSettingsOpen = false" class="fixed inset-0 bg-black/20 z-40 lg:hidden transition-opacity"></div>

    <!-- Боковое меню (Sidebar) -->
    <div class="fixed top-0 right-0 h-full w-80 bg-white shadow-2xl z-50 transform transition-transform duration-300 ease-in-out border-l border-gray-200 flex flex-col"
         :class="isSettingsOpen ? 'translate-x-0' : 'translate-x-full'">

      <!-- Заголовок сайдбара -->
      <div class="p-5 border-b border-gray-100 flex justify-between items-center bg-gray-50">
        <h2 class="text-lg font-bold text-gray-800 flex items-center gap-2">
          {{ t('settings.title') }}
        </h2>
        <button @click="isSettingsOpen = false" class="text-gray-400 hover:text-red-500 transition-colors">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
        </button>
      </div>

      <!-- Контент сайдбара (скроллится если много настроек) -->
      <div class="p-5 overflow-y-auto flex-1 space-y-2">
        <div class="mb-8 p-5 bg-gray-50 rounded-xl border border-gray-200">
          <div class="flex justify-between items-end mb-4">
              <div>
                  <h3 class="text-md font-bold text-gray-800">{{ t('settings.scroll_title') }}</h3>
                  <p class="text-xs text-gray-500 mt-1">{{ t('settings.scroll_desc') }}</p>
              </div>
          </div>

          <div class="relative w-full">
              <input
                  type="range"
                  :min="scrollBarParams.min"
                  :max="scrollBarParams.max"
                  :step="scrollBarParams.step"
                  v-model.number="params.main_options.population_size"
                  @input="updateGenerations"
                  class="w-full h-2 bg-gray-300 rounded-lg appearance-none cursor-pointer accent-indigo-600 focus:outline-none focus:ring-2 focus:ring-indigo-500"
              >
              <!-- Подписи под ползунком -->
              <div class="flex justify-between text-xs text-gray-500 mt-2 font-medium">
                  <span>{{ t('settings.scroll_fast') }}</span>
                  <span>{{ t('settings.scroll_optimal') }}</span>
                  <span>{{ t('settings.scroll_max') }}</span>
              </div>
          </div>
        </div>

      <!-- ПРОДВИНУТЫЕ НАСТРОЙКИ -->
      <div class="border-t border-b border-gray-100 py-2">
          <button
        @click="expandedSections.all = !expandedSections.all"
        class="w-full flex justify-between items-center py-3 text-left focus:outline-none group"
    >
        <span class="text-sm font-bold uppercase tracking-wider transition-colors"
              :class="expandedSections.all ? 'text-indigo-600' : 'text-gray-500 group-hover:text-gray-800'">
          {{ t('settings.all_title') }}
        </span>
        <svg xmlns="http://www.w3.org/2000/svg"
             class="w-4 h-4 text-gray-400 transition-transform duration-300"
             :class="{ 'rotate-180': expandedSections.all }"
             fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
    </button>

    <!-- Содержимое аккордеона -->
    <div v-show="expandedSections.all" class="pt-2 pb-4">

        <!-- СОСТОЯНИЕ 1: Блок предупреждения (показываем, если НЕ разблокировано) -->
        <div v-if="!isAdvancedUnlocked" class="bg-amber-50 border border-amber-200 rounded-lg p-4 shadow-sm">
            <div class="flex items-start mb-4">
                <!-- Иконка Alert -->
                <svg class="w-5 h-5 text-amber-500 mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <div class="ml-3">
                    <h4 class="text-sm font-semibold text-amber-800">{{ t('settings.warn_title') }}</h4>
                    <p class="text-xs text-amber-700 mt-1 leading-relaxed">
                        {{ t('settings.warn_desc') }}
                    </p>
                </div>
            </div>

            <!-- Кнопки действий -->
            <div class="flex space-x-2 pl-8">
                <button
                    @click="isAdvancedUnlocked = true"
                    class="text-xs font-medium px-3 py-1.5 bg-amber-500 text-white rounded-md hover:bg-amber-600 transition-colors"
                >
                    {{ t('settings.warn_btn_ok') }}
                </button>
                <button
                    @click="expandedSections.all = false"
                    class="text-xs font-medium px-3 py-1.5 bg-amber-100 text-amber-800 rounded-md hover:bg-amber-200 transition-colors"
                >
                    {{ t('settings.warn_btn_cancel') }}
                </button>
            </div>
        </div>

        <div v-else class="space-y-5 transition-opacity duration-300 animate-fade-in">
            <!-- Секция 1: Базовые настройки -->
            <div class="pl-2 border-b border-gray-100 pb-2">
              <button
                @click="expandedSections.main = !expandedSections.main"
                class="w-full flex justify-between items-center py-3 text-left focus:outline-none group"
              >
                <span class="text-xs font-bold uppercase tracking-wider transition-colors"
                      :class="expandedSections.main ? 'text-indigo-600' : 'text-gray-500 group-hover:text-gray-800'">
                  {{ t('settings.main_title') }}
                </span>
                <!-- Иконка стрелочки (крутится при открытии) -->
                <svg xmlns="http://www.w3.org/2000/svg"
                     class="w-4 h-4 text-gray-400 transition-transform duration-300"
                     :class="{ 'rotate-180': expandedSections.main }"
                     fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
                </svg>
              </button>

              <!-- Контент скрывается/показывается -->
              <div v-show="expandedSections.main" class="space-y-4 pt-2 pb-4">
                <SettingInput
                  v-model="params.main_options.population_size"
                  :label="t('settings.pop_size')"
                  :tooltip="t('settings.pop_size_tip')"
                />

                <SettingInput
                  v-model="params.main_options.generations"
                  :label="t('settings.generations')"
                  :tooltip="t('settings.generations_tip')"
                />

                <SettingInput
                  v-model="params.main_options.mutation_rate"
                  :label="t('settings.mutation')"
                  :tooltip="t('settings.mutation_tip')"
                  step="0.001"
                />
              </div>
            </div>

            <!-- Секция 2: Продвинутые настройки -->
            <div class="pl-2 border-b border-gray-100 pb-2">
              <button
                @click="expandedSections.advanced = !expandedSections.advanced"
                class="w-full flex justify-between items-center py-3 text-left focus:outline-none group"
              >
                <span class="text-xs font-bold uppercase tracking-wider transition-colors"
                      :class="expandedSections.advanced ? 'text-indigo-600' : 'text-gray-500 group-hover:text-gray-800'">
                  {{ t('settings.adv_title') }}
                </span>
                <svg xmlns="http://www.w3.org/2000/svg"
                     class="w-4 h-4 text-gray-400 transition-transform duration-300"
                     :class="{ 'rotate-180': expandedSections.advanced }"
                     fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
                </svg>
              </button>

              <div v-show="expandedSections.advanced" class="space-y-4 pt-2 pb-4">
                <SettingInput
                  v-model="params.additional_options.elitism"
                  :label="t('settings.elitism')"
                  :tooltip="t('settings.elitism_tip')"
                  step="0.01"
                />

                <SettingInput
                  v-model="params.additional_options.tournament_size"
                  :label="t('settings.tournament')"
                  :tooltip="t('settings.tournament_tip')"
                />

                <SettingToggle
                  v-model="params.additional_options.is_soft_mutation_enabled"
                  :label="t('settings.soft_mutation')"
                  :tooltip="t('settings.soft_mutation_tip')"
                />

                <SettingInput
                  v-model="params.additional_options.soft_mutation_rate"
                  :label="t('settings.soft_mutation_rate')"
                  :tooltip="t('settings.soft_mutation_rate_tip')"
                  step="0.1"
                />

                <SettingInput
                  v-model="params.additional_options.soft_mutation_attempts"
                  :label="t('settings.soft_mutation_attempts')"
                  :tooltip="t('settings.soft_mutation_attempts_tip')"
                />

                <SettingInput
                  v-model="params.additional_options.heat_stagnant_count"
                  :label="t('settings.heat_stagnant_count')"
                  :tooltip="t('settings.heat_stagnant_count_tip')"
                />

                <SettingInput
                  v-model="params.additional_options.heat_step_scale"
                  :label="t('settings.heat_step_scale')"
                  :tooltip="t('settings.heat_step_scale_tip')"
                  step="0.01"
                />

                <SettingInput
                  v-model="params.additional_options.shock_stagnant_count"
                  :label="t('settings.shock_stagnant_count')"
                  :tooltip="t('settings.shock_stagnant_count_tip')"
                />

                <SettingInput
                  v-model="params.additional_options.shock_mutation_rate"
                  :label="t('settings.shock_mutation_rate')"
                  :tooltip="t('settings.shock_mutation_rate_tip')"
                  step="0.01"
                />

                <SettingInput
                  v-model="params.additional_options.shock_min_recovery_count"
                  :label="t('settings.shock_min_recovery_count')"
                  :tooltip="t('settings.shock_min_recovery_count_tip')"
                />

                <SettingInput
                  v-model="params.additional_options.shock_recovery_scale"
                  :label="t('settings.shock_recovery_scale')"
                  :tooltip="t('settings.shock_recovery_scale_tip')"
                  step="0.01"
                />

              </div>
            </div>

            <!-- Секция 3: Правила (Заглушка) -->
            <div class="pl-2 border-b border-gray-100 pb-2">
              <button
                @click="expandedSections.rules = !expandedSections.rules"
                class="w-full flex justify-between items-center py-3 text-left focus:outline-none group"
              >
                <span class="text-xs font-bold uppercase tracking-wider transition-colors"
                      :class="expandedSections.rules ? 'text-indigo-600' : 'text-gray-500 group-hover:text-gray-800'">
                  {{ t('settings.rules_title') }}
                </span>
                <svg xmlns="http://www.w3.org/2000/svg"
                     class="w-4 h-4 text-gray-400 transition-transform duration-300"
                     :class="{ 'rotate-180': expandedSections.rules }"
                     fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
                </svg>
              </button>

            <div v-show="expandedSections.rules" class="pt-2 pb-4">
            <div class="p-4 bg-gray-50 border border-dashed border-gray-300 rounded-lg text-center text-sm text-gray-400">
              {{ t('settings.rules_dev') }}
            </div>
          </div>

            </div>

            <!-- Сброс настроек  -->
            <div class="px-5 pb-2 flex gap-2">
              <button @click="resetParams" class="flex-1 bg-gray-600 hover:bg-gray-700 text-white py-2 rounded-lg text-sm font-semibold transition-colors">
                {{ t('settings.btn_reset') }}
              </button>
            </div>
        </div>
    </div>
        <!-- Подвал сайдбара (кнопки быстрого старта прямо оттуда) -->
        <div class="p-5 flex gap-2">
          <button @click="generateGenetic" class="flex-1 bg-indigo-600 hover:bg-indigo-700 text-white py-2 rounded-lg text-sm font-semibold transition-colors">
            {{ t('settings.btn_start') }}
          </button>
        </div>

      </div>
    </div>
  </div>
  </div>
</template>
