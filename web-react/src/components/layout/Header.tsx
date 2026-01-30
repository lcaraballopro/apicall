import { useAuthStore } from '@/stores/authStore';
import { useNavigate } from 'react-router-dom';

interface HeaderProps {
    title: string;
}

export function Header({ title }: HeaderProps) {
    const { user, logout } = useAuthStore();
    const navigate = useNavigate();

    const handleLogout = () => {
        logout();
        navigate('/login');
    };

    return (
        <header className="h-16 border-b border-[hsl(var(--border))] px-6 flex items-center justify-between bg-[hsl(var(--card))]">
            <h2 className="text-xl font-semibold text-white">{title}</h2>

            <div className="flex items-center gap-4">
                <div className="text-right">
                    <div className="font-semibold text-white">{user?.fullName || user?.username}</div>
                    <span className="badge text-xs">{user?.role?.toUpperCase()}</span>
                </div>
                <button
                    onClick={handleLogout}
                    className="btn btn-secondary text-sm"
                >
                    Salir
                </button>
            </div>
        </header>
    );
}
