import { create } from 'zustand';
import { persist } from 'zustand/middleware';

type Theme = 'dark' | 'light';

interface ThemeState {
    theme: Theme;
    setTheme: (theme: Theme) => void;
    toggleTheme: () => void;
}

export const useThemeStore = create<ThemeState>()(
    persist(
        (set, get) => ({
            theme: 'dark',
            setTheme: (theme) => {
                set({ theme });
                applyTheme(theme);
            },
            toggleTheme: () => {
                const newTheme = get().theme === 'dark' ? 'light' : 'dark';
                set({ theme: newTheme });
                applyTheme(newTheme);
            },
        }),
        {
            name: 'apicall-theme',
            onRehydrateStorage: () => (state) => {
                // Apply theme on rehydration
                if (state) {
                    applyTheme(state.theme);
                }
            },
        }
    )
);

function applyTheme(theme: Theme) {
    const root = document.documentElement;
    root.classList.remove('dark', 'light');
    root.classList.add(theme);
}
