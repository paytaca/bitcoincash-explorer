export type ThemeMode = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'bchexplorer.theme'

function getStoredTheme(): ThemeMode {
  try {
    const s = localStorage.getItem(STORAGE_KEY)
    if (s === 'light' || s === 'dark' || s === 'system') return s
  } catch {}
  return 'system'
}

export function useTheme() {
  const theme = useState<ThemeMode>('theme', () => getStoredTheme())

  const isDark = computed(() => {
    if (theme.value === 'dark') return true
    if (theme.value === 'light') return false
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  function setTheme(mode: ThemeMode) {
    theme.value = mode
    try {
      localStorage.setItem(STORAGE_KEY, mode)
    } catch {}
  }

  function toggleTheme() {
    const next = isDark.value ? 'light' : 'dark'
    setTheme(next)
  }

  return { theme, isDark, setTheme, toggleTheme }
}
