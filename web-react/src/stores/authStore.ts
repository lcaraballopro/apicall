import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface User {
    username: string;
    role: string;
    fullName: string;
}

interface AuthState {
    token: string | null;
    user: User | null;
    isAuthenticated: boolean;
    login: (token: string, user: User) => void;
    logout: () => void;
}

export const useAuthStore = create<AuthState>()(
    persist(
        (set) => ({
            token: null,
            user: null,
            isAuthenticated: false,
            login: (token, user) => {
                localStorage.setItem('apicall_token', token);
                localStorage.setItem('apicall_user', JSON.stringify(user));
                set({ token, user, isAuthenticated: true });
            },
            logout: () => {
                localStorage.removeItem('apicall_token');
                localStorage.removeItem('apicall_user');
                set({ token: null, user: null, isAuthenticated: false });
            },
        }),
        {
            name: 'apicall-auth',
        }
    )
);
