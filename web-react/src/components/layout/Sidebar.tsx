import { NavLink } from 'react-router-dom';
import {
    LayoutDashboard,
    FolderKanban,
    Network,

    FileText,
    Music,
    Users,
    Megaphone,
    Settings,
    Sun,
    Moon,
} from 'lucide-react';
import { useAuthStore } from '@/stores/authStore';
import { useThemeStore } from '@/stores/themeStore';
import { cn } from '@/lib/utils';

const navItems = [
    { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
    { to: '/proyectos', icon: FolderKanban, label: 'Proyectos' },
    { to: '/campanas', icon: Megaphone, label: 'Campañas' },
    { to: '/troncales', icon: Network, label: 'Troncales' },

    { to: '/reportes', icon: FileText, label: 'Reportes' },
    { to: '/audios', icon: Music, label: 'Audios', adminOnly: true },
    { to: '/usuarios', icon: Users, label: 'Usuarios', adminOnly: true },
    { to: '/configuracion', icon: Settings, label: 'Configuración', adminOnly: true },
];

export function Sidebar() {
    const user = useAuthStore((s) => s.user);
    const isAdmin = user?.role === 'admin';
    const { theme, toggleTheme } = useThemeStore();

    return (
        <aside className="w-64 h-screen bg-[hsl(var(--card))] border-r border-[hsl(var(--border))] flex flex-col">
            {/* Logo */}
            <div className="p-6 flex items-center gap-3">
                <img src="/logo.png" alt="Chock Telecom" className="h-10 w-auto" />
            </div>

            {/* Navigation */}
            <nav className="flex-1 px-3">
                {navItems.map((item) => {
                    if (item.adminOnly && !isAdmin) return null;
                    return (
                        <NavLink
                            key={item.to}
                            to={item.to}
                            className={({ isActive }) =>
                                cn(
                                    'flex items-center gap-3 px-4 py-3 rounded-lg mb-1 transition-all duration-200 text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] hover:bg-[hsl(var(--secondary))]',
                                    isActive && 'bg-[hsl(var(--primary))] text-white'
                                )
                            }
                        >
                            <item.icon size={20} />
                            <span className="font-medium">{item.label}</span>
                        </NavLink>
                    );
                })}
            </nav>

            {/* Footer with Theme Toggle and Server Status */}
            <div className="p-4 border-t border-[hsl(var(--border))] space-y-3">
                {/* Theme Toggle */}
                <button
                    onClick={toggleTheme}
                    className="w-full flex items-center justify-between px-3 py-2 rounded-lg bg-[hsl(var(--secondary))] hover:bg-[hsl(var(--accent))] transition-all duration-200"
                >
                    <span className="text-sm text-[hsl(var(--muted-foreground))]">
                        {theme === 'dark' ? 'Modo Oscuro' : 'Modo Claro'}
                    </span>
                    <div className="relative w-12 h-6 bg-[hsl(var(--background))] rounded-full p-1 transition-all">
                        <div
                            className={cn(
                                "absolute w-4 h-4 rounded-full bg-[hsl(var(--primary))] transition-all duration-300 flex items-center justify-center",
                                theme === 'dark' ? 'left-1' : 'left-7'
                            )}
                        >
                            {theme === 'dark' ? (
                                <Moon size={10} className="text-white" />
                            ) : (
                                <Sun size={10} className="text-white" />
                            )}
                        </div>
                    </div>
                </button>

                {/* Server Status */}
                <div className="flex items-center gap-2 text-sm text-[hsl(var(--muted-foreground))]">
                    <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
                    <span>Server Online</span>
                </div>
            </div>
        </aside>
    );
}
