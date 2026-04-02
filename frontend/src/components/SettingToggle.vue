<script setup lang="ts">
// Используем boolean вместо number
const model = defineModel<boolean>()

defineProps<{
  label: string
  tooltip?: string
}>()
</script>

<template>
  <div class="relative">
    <!-- Шапка с лейблом и тултипом (один в один как в твоем компоненте) -->
    <div class="flex items-center justify-between mb-2">
      <label class="block text-sm font-medium text-gray-700 cursor-pointer" @click="model = !model">
        {{ label }}
      </label>
      
      <!-- Иконка с тултипом -->
      <div v-if="tooltip" class="group relative flex items-center cursor-help">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="w-4 h-4 text-gray-400 group-hover:text-indigo-500 transition-colors">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 5.25h.008v.008H12v-.008Z" />
        </svg>
        
        <!-- Сам тултип -->
        <div class="opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 absolute top-full right-0 mt-2 w-56 p-3 bg-gray-800 text-xs text-white rounded-lg shadow-xl z-[100] text-left leading-relaxed pointer-events-none">
          <div class="absolute bottom-full right-1.5 -mb-[1px] border-8 border-transparent border-b-gray-800"></div>
          {{ tooltip }}
        </div>
      </div>
    </div>
    
    <!-- Красивый Toggle вместо обычного чекбокса -->
    <div class="flex items-center">
      <button 
        type="button" 
        role="switch" 
        :aria-checked="model" 
        @click="model = !model"
        :class="model ? 'bg-indigo-600' : 'bg-gray-200'"
        class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
      >
        <span class="sr-only">Use setting</span>
        <span 
          aria-hidden="true" 
          :class="model ? 'translate-x-5' : 'translate-x-0'"
          class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
        ></span>
      </button>
      
      <!-- Текст статуса рядом с тумблером (опционально, можешь убрать) -->
      <span class="ml-3 text-sm" :class="model ? 'text-indigo-600 font-medium' : 'text-gray-500'">
        {{ model ? 'Включено' : 'Выключено' }}
      </span>
    </div>
  </div>
</template>