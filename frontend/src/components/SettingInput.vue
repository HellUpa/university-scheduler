<!-- src/components/SettingInput.vue -->
<script setup lang="ts">
// defineModel автоматически связывает v-model из родителя с этим инпутом
const model = defineModel<number>()

// Определяем параметры, которые можно передать компоненту
defineProps<{
  label: string
  tooltip?: string // ? означает, что параметр необязательный
  step?: string
}>()
</script>

<template>
  <div>
    <div class="flex items-center gap-1 mb-1">
      <label class="block text-sm font-medium text-gray-700">{{ label }}</label>
      
      <!-- Тултип рендерится только если передан текст tooltip -->
      <div v-if="tooltip" class="group relative flex items-center justify-center cursor-help">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-4 h-4 text-gray-400 hover:text-indigo-500">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 5.25h.008v.008H12v-.008Z" />
        </svg>
        <div class="hidden group-hover:block absolute bottom-full left-1/2 -translate-x-1/2 mb-2 w-48 p-2 bg-gray-800 text-xs text-white rounded shadow-lg z-50 text-center">
          {{ tooltip }}
        </div>
      </div>
    </div>
    
    <!-- v-model.number гарантирует, что в Go полетит число, а не строка -->
    <input 
      type="number" 
      :step="step || '1'" 
      v-model.number="model" 
      class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 bg-white transition-colors"
    >
  </div>
</template>