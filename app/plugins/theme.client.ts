const STORAGE_KEY = 'bchexplorer.theme'

export default defineNuxtPlugin(() => {
  const { isDark, setTheme } = useTheme()
  try {
    const stored = localStorage.getItem(STORAGE_KEY) as 'light' | 'dark' | 'system' | null
    if (stored === 'light' || stored === 'dark' || stored === 'system') setTheme(stored)
  } catch {}

  function apply() {
    document.documentElement.classList.toggle('dark', isDark.value)
  }
  apply()
  watch(isDark, apply)
  const mq = window.matchMedia('(prefers-color-scheme: dark)')
  mq.addEventListener('change', apply)

  window.addEventListener('theme-toggled', ((e: CustomEvent<{ dark: boolean }>) => {
    setTheme(e.detail.dark ? 'dark' : 'light')
  }) as EventListener)
})
