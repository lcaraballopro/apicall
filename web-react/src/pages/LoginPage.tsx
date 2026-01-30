import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useLogin } from '@/hooks/useApi';
import { useAuthStore } from '@/stores/authStore';

export function LoginPage() {
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');

    const navigate = useNavigate();
    const login = useAuthStore((s) => s.login);
    const loginMutation = useLogin();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');

        try {
            const data = await loginMutation.mutateAsync({ username, password });
            login(data.token, data.user);
            navigate('/');
        } catch {
            setError('Credenciales inválidas');
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-[hsl(var(--background))]">
            <div className="w-full max-w-md p-8">
                {/* Logo */}
                <div className="text-center mb-8">
                    <img src="/logo.png" alt="Chock Telecom" className="h-20 mx-auto mb-4" />
                    <p className="text-gray-400 mt-2">Sistema de Gestión de Llamadas</p>
                </div>

                {/* Login Card */}
                <div className="card">
                    <h2 className="text-xl font-semibold text-white mb-6">Iniciar Sesión</h2>

                    <form onSubmit={handleSubmit} className="space-y-4">
                        <div>
                            <label className="block text-sm font-medium text-gray-300 mb-1">
                                Usuario
                            </label>
                            <input
                                type="text"
                                value={username}
                                onChange={(e) => setUsername(e.target.value)}
                                className="input"
                                placeholder="admin"
                                required
                            />
                        </div>

                        <div>
                            <label className="block text-sm font-medium text-gray-300 mb-1">
                                Contraseña
                            </label>
                            <input
                                type="password"
                                value={password}
                                onChange={(e) => setPassword(e.target.value)}
                                className="input"
                                placeholder="••••••••"
                                required
                            />
                        </div>

                        {error && (
                            <div className="text-red-500 text-sm bg-red-500/10 p-3 rounded-lg">
                                {error}
                            </div>
                        )}

                        <button
                            type="submit"
                            disabled={loginMutation.isPending}
                            className="btn btn-primary w-full py-3 text-lg"
                        >
                            {loginMutation.isPending ? 'Ingresando...' : 'Ingresar'}
                        </button>
                    </form>
                </div>
            </div>
        </div>
    );
}
