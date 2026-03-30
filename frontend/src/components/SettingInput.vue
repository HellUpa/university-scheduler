<script setup lang="ts">
const model = defineModel<number>()

defineProps<{
  label: string
  tooltip?: string
  step?: string
}>()
</script>

<template>
  <div class="relative">
    <div class="flex items-center justify-between mb-1">
      <label class="block text-sm font-medium text-gray-700">{{ label }}</label>
      
      <!-- Иконка с тултипом -->
      <div v-if="tooltip" class="group relative flex items-center cursor-help">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-4 h-4 text-gray-400 group-hover:text-indigo-500 transition-colors">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 5.25h.008v.008H12v-.008Z" />
        </svg>
        
        <!-- Сам тултип -->
        <div class="opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 absolute top-full right-0 mt-2 w-56 p-3 bg-gray-800 text-xs text-white rounded-lg shadow-xl z-[100] text-left leading-relaxed pointer-events-none">
          <!-- Треугольник -->
          <div class="absolute bottom-full right-1.5 -mb-[1px] border-8 border-transparent border-b-gray-800"></div>
          {{ tooltip }}
        </div>
      </div>
    </div>
    
    <input 
      type="number" 
      :step="step || '1'" 
      v-model.number="model" 
      class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 bg-white transition-shadow shadow-sm"
    >
  </div>
</template>