<!-- src/components/LanguageSwitcher.vue -->
<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()
const isOpen = ref(false)

// Массив доступных языков
const languages = [
  { code: 'ru', name: 'Русский', flag: 'https://flagcdn.com/w40/ru.png' },
  { code: 'kk', name: 'Қазақша', flag: 'https://flagcdn.com/w40/kz.png' },
  { code: 'en', name: 'English', flag: 'https://flagcdn.com/w40/us.png' }
]

// Находим текущий выбранный язык для отображения на кнопке
const currentLang = computed(() => languages.find(l => l.code === locale.value))

const selectLanguage = (code: string) => {
  locale.value = code
  isOpen.value = false
}
</script>

<template>
  <div class="relative">
    <!-- Кнопка открытия меню -->
    <button 
      @click="isOpen = !isOpen"
      class="flex items-center gap-2 px-3 py-2 bg-white border border-gray-200 hover:bg-gray-50 text-gray-700 text-sm font-medium rounded-lg shadow-sm transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500"
    >
      <img :src="currentLang?.flag" alt="flag" class="w-5 h-auto rounded-[2px] shadow-sm">
      <span class="hidden sm:block">{{ currentLang?.code.toUpperCase() }}</span>
      <!-- Стрелочка вниз -->
      <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <!-- Прозрачный оверлей для закрытия меню при клике вне его -->
    <div v-if="isOpen" @click="isOpen = false" class="fixed inset-0 z-40"></div>

    <!-- Выпадающий список -->
    <transition
      enter-active-class="transition ease-out duration-100"
      enter-from-class="transform opacity-0 scale-95"
      enter-to-class="transform opacity-100 scale-100"
      leave-active-class="transition ease-in duration-75"
      leave-from-class="transform opacity-100 scale-100"
      leave-to-class="transform opacity-0 scale-95"
    >
      <div v-if="isOpen" class="absolute right-0 mt-2 w-40 bg-white border border-gray-100 rounded-lg shadow-xl z-50 overflow-hidden">
        <div class="py-1">
          <button 
            v-for="lang in languages" 
            :key="lang.code"
            @click="selectLanguage(lang.code)"
            class="w-full flex items-center gap-3 px-4 py-2 text-sm text-left hover:bg-indigo-50 transition-colors"
            :class="{ 'bg-indigo-50 text-indigo-700 font-bold': locale === lang.code, 'text-gray-700': locale !== lang.code }"
          >
            <img :src="lang.flag" alt="flag" class="w-5 h-auto rounded-[2px] shadow-sm">
            {{ lang.name }}
          </button>
        </div>
      </div>
    </transition>
  </div>
</template>