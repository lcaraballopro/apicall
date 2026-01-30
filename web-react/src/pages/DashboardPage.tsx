import { useState, useMemo, useEffect } from 'react';
import { Header } from '@/components/layout';
import { useProyectos, useRealtimeLogs } from '@/hooks/useApi';
import { useDashboardRealtime } from '@/hooks/useWebSocket';
import { useAuthStore } from '@/stores/authStore';
import {
    Phone, Users, Search, Play, CheckCircle,
    AlertTriangle, TrendingUp, PhoneCall, PhoneOff, BarChart3, RefreshCw, Wifi, WifiOff
} from 'lucide-react';
import { Link } from 'react-router-dom';

export function DashboardPage() {
    const { user } = useAuthStore();
    const { data: proyectos, isLoading: loadingProyectos } = useProyectos();
    const { data: allLogs, dataUpdatedAt, isRefetching } = useRealtimeLogs(2000, 5000); // Reduced polling since we have WebSocket
    const { isConnected, callEvents } = useDashboardRealtime();
    const [searchTerm, setSearchTerm] = useState('');
    const [statusFilter, setStatusFilter] = useState('all');
    const [currentTime, setCurrentTime] = useState(new Date());

    // Update clock every second
    useEffect(() => {
        const timer = setInterval(() => setCurrentTime(new Date()), 1000);
        return () => clearInterval(timer);
    }, []);

    // Get current date info (uses currentTime for live updates)
    const greeting = currentTime.getHours() < 12 ? 'Buenos días' : currentTime.getHours() < 18 ? 'Buenas tardes' : 'Buenas noches';
    const dateStr = currentTime.toLocaleDateString('es-ES', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
    const timeStr = currentTime.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    const lastUpdate = dataUpdatedAt ? new Date(dataUpdatedAt).toLocaleTimeString() : '--:--:--';

    // Calculate stats per project
    const projectStats = useMemo(() => {
        if (!allLogs || !proyectos) return {};
        const stats: Record<number, { total: number; answered: number; interacted: number; today: number }> = {};

        const todayStart = new Date();
        todayStart.setHours(0, 0, 0, 0);

        allLogs.forEach(log => {
            if (!stats[log.proyecto_id]) {
                stats[log.proyecto_id] = { total: 0, answered: 0, interacted: 0, today: 0 };
            }
            stats[log.proyecto_id].total++;
            if (log.status === 'ANSWERED' || log.disposition === 'ANSWERED') {
                stats[log.proyecto_id].answered++;
            }
            if (log.interacciono) {
                stats[log.proyecto_id].interacted++;
            }
            if (new Date(log.created_at) >= todayStart) {
                stats[log.proyecto_id].today++;
            }
        });
        return stats;
    }, [allLogs, proyectos]);

    // Filter projects
    const filteredProyectos = useMemo(() => {
        if (!proyectos) return [];
        return proyectos.filter(p => {
            const matchSearch = p.nombre.toLowerCase().includes(searchTerm.toLowerCase());
            const stats = projectStats[p.id];
            const hasActivity = stats && stats.total > 0;

            if (statusFilter === 'active' && !hasActivity) return false;
            if (statusFilter === 'inactive' && hasActivity) return false;

            return matchSearch;
        });
    }, [proyectos, searchTerm, statusFilter, projectStats]);

    // Global stats
    const totalCalls = allLogs?.length || 0;
    const totalAnswered = allLogs?.filter(l => l.status === 'ANSWERED' || l.disposition === 'ANSWERED').length || 0;
    const totalInteracted = allLogs?.filter(l => l.interacciono).length || 0;

    const getProjectStatus = (projectId: number) => {
        const stats = projectStats[projectId];
        if (!stats || stats.total === 0) return { label: 'Sin actividad', color: 'bg-gray-500', textColor: 'text-gray-300' };
        if (stats.today > 0) return { label: 'Activo', color: 'bg-green-500', textColor: 'text-green-400' };
        return { label: 'Pausado', color: 'bg-orange-500', textColor: 'text-orange-400' };
    };

    return (
        <>
            <Header title="Campañas" />
            <div className="p-6">
                {/* Header with greeting */}
                <div className="flex flex-col md:flex-row md:items-center md:justify-between mb-6">
                    <div>
                        <p className="text-gray-400 text-sm">Gestione y supervise sus campañas de marcado</p>
                        <div className="flex items-center gap-4 mt-1">
                            {/* WebSocket Connection Status */}
                            <div className={`flex items-center gap-1 ${isConnected ? 'text-green-400' : 'text-red-400'}`}>
                                {isConnected ? <Wifi size={12} /> : <WifiOff size={12} />}
                                <span className="text-xs">
                                    {isConnected ? 'Tiempo real' : 'Reconectando...'}
                                </span>
                            </div>
                            {/* Polling Status */}
                            <div className="flex items-center gap-1">
                                <RefreshCw size={12} className={`text-blue-400 ${isRefetching ? 'animate-spin' : ''}`} />
                                <span className="text-xs text-gray-500">
                                    {lastUpdate}
                                </span>
                            </div>
                            {/* Recent Events Counter */}
                            {callEvents.length > 0 && (
                                <span className="text-xs text-emerald-400 animate-pulse">
                                    ⚡ {callEvents.length} eventos
                                </span>
                            )}
                        </div>
                    </div>
                    <div className="text-right mt-2 md:mt-0">
                        <p className="text-white font-medium">👋 {greeting}, {user?.fullName || 'Usuario'}</p>
                        <p className="text-gray-400 text-sm">📅 {dateStr}</p>
                        <p className="text-gray-500 text-xs">🕐 {timeStr}</p>
                    </div>
                </div>

                {/* Global Stats */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
                    <div className="card bg-gradient-to-br from-blue-500/20 to-blue-600/10 border-blue-500/30">
                        <div className="flex items-center gap-3">
                            <div className="p-3 bg-blue-500/20 rounded-xl">
                                <BarChart3 size={28} className="text-blue-400" />
                            </div>
                            <div>
                                <p className="text-3xl font-bold text-white">{proyectos?.length || 0}</p>
                                <p className="text-xs text-gray-400">Proyectos</p>
                            </div>
                        </div>
                    </div>
                    <div className="card bg-gradient-to-br from-purple-500/20 to-purple-600/10 border-purple-500/30">
                        <div className="flex items-center gap-3">
                            <div className="p-3 bg-purple-500/20 rounded-xl">
                                <Phone size={28} className="text-purple-400" />
                            </div>
                            <div>
                                <p className="text-3xl font-bold text-white">{totalCalls.toLocaleString()}</p>
                                <p className="text-xs text-gray-400">Total Llamadas</p>
                            </div>
                        </div>
                    </div>
                    <div className="card bg-gradient-to-br from-green-500/20 to-green-600/10 border-green-500/30">
                        <div className="flex items-center gap-3">
                            <div className="p-3 bg-green-500/20 rounded-xl">
                                <CheckCircle size={28} className="text-green-400" />
                            </div>
                            <div>
                                <p className="text-3xl font-bold text-white">{totalAnswered.toLocaleString()}</p>
                                <p className="text-xs text-gray-400">Contestadas</p>
                            </div>
                        </div>
                    </div>
                    <div className="card bg-gradient-to-br from-emerald-500/20 to-emerald-600/10 border-emerald-500/30">
                        <div className="flex items-center gap-3">
                            <div className="p-3 bg-emerald-500/20 rounded-xl">
                                <TrendingUp size={28} className="text-emerald-400" />
                            </div>
                            <div>
                                <p className="text-3xl font-bold text-white">{totalInteracted.toLocaleString()}</p>
                                <p className="text-xs text-gray-400">Interacciones</p>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Search and Filter Bar */}
                <div className="flex flex-col md:flex-row gap-4 mb-6">
                    <div className="relative flex-1">
                        <Search size={18} className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400" />
                        <input
                            type="text"
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            placeholder="Buscar campañas..."
                            className="input pl-10 w-full"
                        />
                    </div>
                    <select
                        value={statusFilter}
                        onChange={(e) => setStatusFilter(e.target.value)}
                        className="input w-full md:w-48"
                    >
                        <option value="all">📋 Todos</option>
                        <option value="active">🟢 Activos</option>
                        <option value="inactive">🟠 Sin actividad</option>
                    </select>
                </div>

                {/* Project Cards Grid */}
                {loadingProyectos ? (
                    <div className="text-center py-12 text-gray-400">
                        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4"></div>
                        Cargando proyectos...
                    </div>
                ) : (
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
                        {filteredProyectos.map((proyecto) => {
                            const stats = projectStats[proyecto.id] || { total: 0, answered: 0, interacted: 0, today: 0 };
                            const status = getProjectStatus(proyecto.id);
                            const successRate = stats.total > 0 ? (stats.answered / stats.total) * 100 : 0;

                            return (
                                <div
                                    key={proyecto.id}
                                    className="card hover:border-blue-500/50 transition-all hover:shadow-lg hover:shadow-blue-500/10 cursor-pointer group"
                                >
                                    {/* Header */}
                                    <div className="flex items-start justify-between mb-3">
                                        <div className="flex-1 min-w-0">
                                            <h3 className="text-white font-semibold truncate group-hover:text-blue-400 transition-colors">
                                                {proyecto.nombre}
                                            </h3>
                                            <p className="text-xs text-gray-500 truncate">ID: {proyecto.id}</p>
                                        </div>
                                        <span className={`px-2 py-1 rounded-full text-xs font-medium ${status.color} text-white ml-2 whitespace-nowrap`}>
                                            {status.label}
                                        </span>
                                    </div>

                                    {/* Stats Row */}
                                    <div className="flex items-center gap-4 text-sm mb-3">
                                        <div className="flex items-center gap-1 text-gray-400">
                                            <Phone size={14} className="text-blue-400" />
                                            <span className="text-white font-medium">{stats.total.toLocaleString()}</span>
                                        </div>
                                        <div className="flex items-center gap-1 text-gray-400">
                                            <Users size={14} className="text-green-400" />
                                            <span className="text-white">{stats.answered}</span>
                                            <span className="text-gray-500">·</span>
                                            <span className="text-emerald-400">{stats.interacted}</span>
                                        </div>
                                    </div>

                                    {/* Progress Bar */}
                                    {stats.total > 0 && (
                                        <div className="space-y-2">
                                            <div className="flex justify-between text-xs">
                                                <span className="text-gray-400">Tasa de éxito</span>
                                                <span className="text-green-400">{successRate.toFixed(1)}%</span>
                                            </div>
                                            <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                                                <div
                                                    className="h-full bg-gradient-to-r from-green-500 to-emerald-400 rounded-full transition-all duration-500"
                                                    style={{ width: `${successRate}%` }}
                                                />
                                            </div>
                                            {stats.today > 0 && (
                                                <p className="text-xs text-blue-400 flex items-center gap-1">
                                                    <PhoneCall size={12} />
                                                    {stats.today} llamadas hoy
                                                </p>
                                            )}
                                        </div>
                                    )}

                                    {stats.total === 0 && (
                                        <div className="text-center py-2 text-gray-500 text-sm">
                                            <PhoneOff size={20} className="mx-auto mb-1 opacity-50" />
                                            Sin llamadas registradas
                                        </div>
                                    )}
                                </div>
                            );
                        })}
                    </div>
                )}

                {/* Empty State */}
                {!loadingProyectos && filteredProyectos.length === 0 && (
                    <div className="text-center py-12">
                        <AlertTriangle size={48} className="mx-auto text-gray-600 mb-4" />
                        <p className="text-gray-400">No se encontraron proyectos</p>
                        {searchTerm && (
                            <button
                                onClick={() => setSearchTerm('')}
                                className="mt-2 text-blue-400 hover:text-blue-300 text-sm"
                            >
                                Limpiar búsqueda
                            </button>
                        )}
                    </div>
                )}

                {/* Quick Actions */}
                <div className="mt-8 grid grid-cols-2 md:grid-cols-4 gap-4">
                    <Link to="/proyectos" className="card hover:border-blue-500/50 text-center py-4 transition-all hover:scale-105">
                        <BarChart3 size={24} className="mx-auto text-blue-400 mb-2" />
                        <span className="text-sm text-gray-300">Gestionar Proyectos</span>
                    </Link>

                    <Link to="/reports" className="card hover:border-purple-500/50 text-center py-4 transition-all hover:scale-105">
                        <TrendingUp size={24} className="mx-auto text-purple-400 mb-2" />
                        <span className="text-sm text-gray-300">Ver Reportes</span>
                    </Link>
                    <Link to="/audios" className="card hover:border-orange-500/50 text-center py-4 transition-all hover:scale-105">
                        <Play size={24} className="mx-auto text-orange-400 mb-2" />
                        <span className="text-sm text-gray-300">Gestionar Audios</span>
                    </Link>
                </div>
            </div>
        </>
    );
}
