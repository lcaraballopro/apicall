import { useState, useMemo, useCallback } from 'react';
import { Header } from '@/components/layout';
import { DataTable } from '@/components/ui';
import { useProyectos, useCallLogs, useCampaigns } from '@/hooks/useApi';
import { Search, Phone, PhoneOff, PhoneCall, Clock, CheckCircle, XCircle, Bot, AlertTriangle, ChevronDown, ChevronUp, Filter, Download, PhoneForwarded, RotateCw, Timer, WifiOff } from 'lucide-react';
import type { CallLog } from '@/types';

const PAGE_SIZE = 50;

export function ReportsPage() {
    const { data: proyectos } = useProyectos();
    const [filters, setFilters] = useState({ proyecto_id: '', campaign_id: '', from_date: '', to_date: '' });

    // Fetch campaigns when a project is selected
    const { data: campaigns } = useCampaigns(filters.proyecto_id ? Number(filters.proyecto_id) : undefined);

    const [showAdvanced, setShowAdvanced] = useState(false);
    const [advancedFilters, setAdvancedFilters] = useState({ status: '', interacciono: '', search: '' });
    const [currentPage, setCurrentPage] = useState(1);

    const { data: logs, isLoading, refetch } = useCallLogs({
        proyecto_id: filters.proyecto_id ? Number(filters.proyecto_id) : undefined,
        campaign_id: filters.campaign_id ? Number(filters.campaign_id) : undefined,
        from_date: filters.from_date,
        to_date: filters.to_date,
        limit: 5000,
    });

    const proyectoMap = Object.fromEntries((proyectos || []).map((p) => [p.id, p.nombre]));
    const timezoneMap = Object.fromEntries((proyectos || []).map((p) => [p.id, p.timezone || 'America/Bogota']));
    const campaignMap = useMemo(() => Object.fromEntries((campaigns || []).map((c) => [c.id, c.nombre])), [campaigns]);

    // Format date using the selected project's timezone
    // Note: Server sends dates with 'Z' suffix but they are actually local server time (America/Bogota)
    // We need to interpret them correctly and then display in the project's timezone
    const formatDate = (dateStr: string, proyectoId?: number) => {
        try {
            // Remove the Z suffix to treat as local time, then add the server's timezone offset
            const cleanDateStr = dateStr.replace('Z', '');
            // Parse the date as if it were in the server's timezone (America/Bogota = UTC-5)
            const serverTz = 'America/Bogota';
            const targetTz = proyectoId ? timezoneMap[proyectoId] : serverTz;

            // Create date object - JavaScript will parse this as local time since there's no Z
            const date = new Date(cleanDateStr);

            // Format in the target timezone
            return date.toLocaleString('es-CO', {
                timeZone: targetTz || serverTz,
                year: 'numeric',
                month: '2-digit',
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
                hour12: true
            });
        } catch {
            return new Date(dateStr).toLocaleString();
        }
    };

    // Status config with icons, colors and FRIENDLY labels in Spanish
    // Standard Contact Center Disposition Codes + Legacy Support
    const statusConfig: Record<string, { icon: React.ElementType; color: string; bgColor: string; label: string }> = {
        // === STANDARD CONTACT CENTER DISPOSITIONS ===
        'A': { icon: CheckCircle, color: 'text-emerald-400', bgColor: 'from-emerald-500/20 to-emerald-600/10', label: 'Contactado' },
        'AM': { icon: Bot, color: 'text-violet-400', bgColor: 'from-violet-500/20 to-violet-600/10', label: 'Contestadora' },
        'B': { icon: Phone, color: 'text-rose-400', bgColor: 'from-rose-500/20 to-rose-600/10', label: 'Ocupado' },
        'NA': { icon: PhoneOff, color: 'text-amber-400', bgColor: 'from-amber-500/20 to-amber-600/10', label: 'No Contesta' },
        'N': { icon: Timer, color: 'text-orange-400', bgColor: 'from-orange-500/20 to-orange-600/10', label: 'Timeout DTMF' },
        'XFER': { icon: PhoneForwarded, color: 'text-teal-400', bgColor: 'from-teal-500/20 to-teal-600/10', label: 'Transferido' },
        'NI': { icon: XCircle, color: 'text-red-500', bgColor: 'from-red-600/20 to-red-700/10', label: 'Número Inválido' },
        'CONG': { icon: AlertTriangle, color: 'text-yellow-400', bgColor: 'from-yellow-500/20 to-yellow-600/10', label: 'Congestión' },
        'DNC': { icon: AlertTriangle, color: 'text-red-400', bgColor: 'from-red-500/20 to-red-600/10', label: 'No Llamar' },
        'FAIL': { icon: WifiOff, color: 'text-red-400', bgColor: 'from-red-500/20 to-red-600/10', label: 'Fallida' },

        // === STATUS (Ciclo de vida) ===
        'PENDING': { icon: Clock, color: 'text-slate-400', bgColor: 'from-slate-500/20 to-slate-600/10', label: 'Pendiente' },
        'DIALING': { icon: PhoneCall, color: 'text-blue-400', bgColor: 'from-blue-500/20 to-blue-600/10', label: 'Marcando' },
        'CONNECTED': { icon: RotateCw, color: 'text-sky-400', bgColor: 'from-sky-500/20 to-sky-600/10', label: 'Conectado' },
        'COMPLETED': { icon: CheckCircle, color: 'text-emerald-400', bgColor: 'from-emerald-500/20 to-emerald-600/10', label: 'Completado' },
        'FAILED': { icon: XCircle, color: 'text-red-500', bgColor: 'from-red-600/20 to-red-700/10', label: 'Fallida' },

        // === LEGACY SUPPORT (backward compatibility) ===
        'ANSWERED': { icon: CheckCircle, color: 'text-emerald-400', bgColor: 'from-emerald-500/20 to-emerald-600/10', label: 'Contactado' },
        'TRANSFERRED': { icon: PhoneForwarded, color: 'text-teal-400', bgColor: 'from-teal-500/20 to-teal-600/10', label: 'Transferido' },
        'AMD_MACHINE': { icon: Bot, color: 'text-violet-400', bgColor: 'from-violet-500/20 to-violet-600/10', label: 'Contestadora' },
        'BUSY': { icon: Phone, color: 'text-rose-400', bgColor: 'from-rose-500/20 to-rose-600/10', label: 'Ocupado' },
        'NOANSWER': { icon: PhoneOff, color: 'text-amber-400', bgColor: 'from-amber-500/20 to-amber-600/10', label: 'No Contesta' },
        'TIMEOUT': { icon: Timer, color: 'text-orange-400', bgColor: 'from-orange-500/20 to-orange-600/10', label: 'Sin Interés' },
        'CONGESTION': { icon: AlertTriangle, color: 'text-yellow-400', bgColor: 'from-yellow-500/20 to-yellow-600/10', label: 'Congestión' },
        'SPOOL_ERROR': { icon: WifiOff, color: 'text-red-400', bgColor: 'from-red-500/20 to-red-600/10', label: 'Fallida' },
        'INVALID-DTMF': { icon: AlertTriangle, color: 'text-amber-500', bgColor: 'from-amber-500/20 to-amber-600/10', label: 'Sin Interés' },
        'CHANNEL_LIMIT': { icon: AlertTriangle, color: 'text-yellow-400', bgColor: 'from-yellow-500/20 to-yellow-600/10', label: 'Límite Canales' },
        'INITIATED': { icon: RotateCw, color: 'text-sky-400', bgColor: 'from-sky-500/20 to-sky-600/10', label: 'Iniciando' },
        'INITIATED_LEGACY': { icon: RotateCw, color: 'text-slate-400', bgColor: 'from-slate-500/20 to-slate-600/10', label: 'Histórico' },
        'ORPHAN_TIMEOUT': { icon: Timer, color: 'text-orange-400', bgColor: 'from-orange-500/20 to-orange-600/10', label: 'Sin Respuesta' },
        'Marcando': { icon: PhoneCall, color: 'text-blue-400', bgColor: 'from-blue-500/20 to-blue-600/10', label: 'Marcando' },
        'Contestadora': { icon: Bot, color: 'text-violet-400', bgColor: 'from-violet-500/20 to-violet-600/10', label: 'Contestadora' },
    };

    // Calculate stats by status (from full dataset)
    const stats = useMemo(() => logs?.reduce((acc, log) => {
        const status = log.status || 'UNKNOWN';
        acc[status] = (acc[status] || 0) + 1;
        return acc;
    }, {} as Record<string, number>) || {}, [logs]);

    // Filter logs client-side with advanced filters
    const filteredLogs = useMemo(() => {
        if (!logs) return [];
        return logs.filter(log => {
            // Status filter
            if (advancedFilters.status && log.status !== advancedFilters.status) return false;
            // Interaction filter
            if (advancedFilters.interacciono === 'yes' && !log.interacciono) return false;
            if (advancedFilters.interacciono === 'no' && log.interacciono) return false;
            // Search filter
            if (advancedFilters.search) {
                const search = advancedFilters.search.toLowerCase();
                const matchPhone = log.telefono.toLowerCase().includes(search);
                const matchCaller = (log.caller_id_used || '').toLowerCase().includes(search);
                const matchDTMF = (log.dtmf_marcado || '').toLowerCase().includes(search);
                if (!matchPhone && !matchCaller && !matchDTMF) return false;
            }
            return true;
        });
    }, [logs, advancedFilters]);

    // Pagination
    const totalPages = Math.ceil(filteredLogs.length / PAGE_SIZE);
    const paginatedLogs = useMemo(() => {
        const start = (currentPage - 1) * PAGE_SIZE;
        return filteredLogs.slice(start, start + PAGE_SIZE);
    }, [filteredLogs, currentPage]);

    // Reset page when filters change
    const handleAdvancedFilterChange = (key: string, value: string) => {
        setAdvancedFilters({ ...advancedFilters, [key]: value });
        setCurrentPage(1);
    };

    const total = logs?.length || 0;
    const interacted = logs?.filter((l) => l.interacciono).length || 0;
    const filteredTotal = filteredLogs.length;

    // Export to Excel/CSV function
    const exportToExcel = useCallback(() => {
        if (!filteredLogs.length) return;

        const getStatusLabel = (status: string) => statusConfig[status]?.label || status;

        // CSV headers
        const headers = ['ID', 'Proyecto', 'Campaña', 'Teléfono', 'Caller ID', 'Estado', 'Resultado', 'DTMF', 'Interacción', 'Duración (s)', 'Fecha'];

        // CSV rows
        const rows = filteredLogs.map(log => [
            log.id,
            proyectoMap[log.proyecto_id] || `#${log.proyecto_id}`,
            log.campaign_id ? (campaignMap[log.campaign_id] || `#${log.campaign_id}`) : '-',
            log.telefono,
            log.caller_id_used || '',
            getStatusLabel(log.status),
            log.disposition || '',
            log.dtmf_marcado || '',
            log.interacciono ? 'Sí' : 'No',
            log.duracion,
            formatDate(log.created_at, log.proyecto_id)
        ]);

        // Build CSV content with BOM for Excel compatibility
        const BOM = '\uFEFF';
        const csvContent = BOM + [
            headers.join(','),
            ...rows.map(row => row.map(cell => `"${String(cell).replace(/"/g, '""')}"`).join(','))
        ].join('\n');

        // Create and download file
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        const dateStr = new Date().toISOString().split('T')[0];
        link.download = `reporte_llamadas_${dateStr}.csv`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
    }, [filteredLogs, proyectoMap, statusConfig]);

    const getStatusBadge = (status: string) => {
        const config = statusConfig[status] || { icon: Phone, color: 'text-gray-400', label: status };
        const IconComponent = config.icon || Phone;
        return (
            <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${config.color} bg-black/30 border border-current/20`}>
                <IconComponent size={12} />
                {config.label}
            </span>
        );
    };

    const columns = [
        { key: 'id', header: 'ID' },
        { key: 'proyecto', header: 'Proyecto', render: (l: CallLog) => <strong>{proyectoMap[l.proyecto_id] || `#${l.proyecto_id}`}</strong> },
        { key: 'campaign', header: 'Campaña', render: (l: CallLog) => l.campaign_id ? (campaignMap[l.campaign_id] || `#${l.campaign_id}`) : '-' },
        { key: 'telefono', header: 'Teléfono' },
        { key: 'caller_id_used', header: 'Caller ID', render: (l: CallLog) => l.caller_id_used || '-' },
        { key: 'status', header: 'Estado', render: (l: CallLog) => getStatusBadge(l.status) },
        { key: 'disposition', header: 'Resultado', render: (l: CallLog) => l.disposition || '-' },
        { key: 'dtmf_marcado', header: 'DTMF', render: (l: CallLog) => l.dtmf_marcado || '-' },
        { key: 'interacciono', header: 'Interacción', render: (l: CallLog) => l.interacciono ? '✅' : '❌' },
        { key: 'duracion', header: 'Duración', render: (l: CallLog) => `${l.duracion}s` },
        { key: 'created_at', header: 'Fecha', render: (l: CallLog) => formatDate(l.created_at, l.proyecto_id) },
    ];

    // Get unique statuses for filter dropdown
    const uniqueStatuses = Object.keys(stats).sort();

    return (
        <>
            <Header title="Reportes de Llamadas" />
            <div className="p-6">
                {/* Filters */}
                <div className="card mb-6">
                    <div className="flex items-center justify-between mb-4">
                        <h3 className="text-white font-medium">Filtros</h3>
                        <button
                            onClick={() => setShowAdvanced(!showAdvanced)}
                            className="text-sm text-blue-400 hover:text-blue-300 flex items-center gap-1"
                        >
                            <Filter size={14} />
                            Filtros Avanzados
                            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                        </button>
                    </div>

                    {/* Basic Filters */}
                    <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
                        <div>
                            <label className="block text-sm text-gray-300 mb-1">Proyecto</label>
                            <select
                                value={filters.proyecto_id}
                                onChange={(e) => setFilters({ ...filters, proyecto_id: e.target.value })}
                                className="input"
                            >
                                <option value="">Todos</option>
                                {proyectos?.map((p) => (
                                    <option key={p.id} value={p.id}>{p.nombre}</option>
                                ))}
                            </select>
                        </div>
                        <div>
                            <label className="block text-sm text-gray-300 mb-1">Campaña</label>
                            <select
                                value={filters.campaign_id}
                                onChange={(e) => setFilters({ ...filters, campaign_id: e.target.value })}
                                className="input"
                                disabled={!filters.proyecto_id}
                            >
                                <option value="">Todas</option>
                                {campaigns?.map((c) => (
                                    <option key={c.id} value={c.id}>{c.nombre}</option>
                                ))}
                            </select>
                        </div>
                        <div>
                            <label className="block text-sm text-gray-300 mb-1">Desde</label>
                            <input
                                type="date"
                                value={filters.from_date}
                                onChange={(e) => setFilters({ ...filters, from_date: e.target.value })}
                                className="input"
                            />
                        </div>
                        <div>
                            <label className="block text-sm text-gray-300 mb-1">Hasta</label>
                            <input
                                type="date"
                                value={filters.to_date}
                                onChange={(e) => setFilters({ ...filters, to_date: e.target.value })}
                                className="input"
                            />
                        </div>
                        <div className="flex items-end">
                            <button onClick={() => { refetch(); setCurrentPage(1); }} className="btn btn-primary flex items-center gap-2">
                                <Search size={18} /> Buscar
                            </button>
                        </div>
                    </div>

                    {/* Advanced Filters */}
                    {showAdvanced && (
                        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mt-4 pt-4 border-t border-gray-700">
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">🔍 Buscar (teléfono, CID, DTMF)</label>
                                <input
                                    type="text"
                                    value={advancedFilters.search}
                                    onChange={(e) => handleAdvancedFilterChange('search', e.target.value)}
                                    placeholder="Buscar..."
                                    className="input"
                                />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Estado</label>
                                <select
                                    value={advancedFilters.status}
                                    onChange={(e) => handleAdvancedFilterChange('status', e.target.value)}
                                    className="input"
                                >
                                    <option value="">Todos</option>
                                    {uniqueStatuses.map((s) => (
                                        <option key={s} value={s}>{statusConfig[s]?.label || s} ({stats[s]})</option>
                                    ))}
                                </select>
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Interacción</label>
                                <select
                                    value={advancedFilters.interacciono}
                                    onChange={(e) => handleAdvancedFilterChange('interacciono', e.target.value)}
                                    className="input"
                                >
                                    <option value="">Todos</option>
                                    <option value="yes">✅ Con interacción</option>
                                    <option value="no">❌ Sin interacción</option>
                                </select>
                            </div>
                            <div className="flex items-end">
                                <button
                                    onClick={() => { setAdvancedFilters({ status: '', interacciono: '', search: '' }); setCurrentPage(1); }}
                                    className="btn bg-gray-600 hover:bg-gray-500 text-white"
                                >
                                    Limpiar Filtros
                                </button>
                            </div>
                        </div>
                    )}
                </div>

                {/* Status Cards */}
                {logs && logs.length > 0 && (
                    <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4 mb-6">
                        {/* Total Card */}
                        <div className="card bg-gradient-to-br from-blue-500/20 to-blue-600/10 border-blue-500/30">
                            <div className="flex items-center gap-3">
                                <div className="p-2 bg-blue-500/20 rounded-lg">
                                    <Phone size={24} className="text-blue-400" />
                                </div>
                                <div>
                                    <p className="text-2xl font-bold text-white">{total}</p>
                                    <p className="text-xs text-gray-400">Total</p>
                                </div>
                            </div>
                        </div>

                        {/* Interaction Card */}
                        <div className="card bg-gradient-to-br from-emerald-500/20 to-emerald-600/10 border-emerald-500/30">
                            <div className="flex items-center gap-3">
                                <div className="p-2 bg-emerald-500/20 rounded-lg">
                                    <CheckCircle size={24} className="text-emerald-400" />
                                </div>
                                <div>
                                    <p className="text-2xl font-bold text-white">{interacted}</p>
                                    <p className="text-xs text-gray-400">Interacción</p>
                                </div>
                            </div>
                        </div>

                        {/* Status Cards */}
                        {Object.entries(stats)
                            .sort((a, b) => b[1] - a[1])
                            .slice(0, 4)
                            .map(([status, count]) => {
                                const config = statusConfig[status] || {
                                    icon: Phone,
                                    color: 'text-gray-400',
                                    bgColor: 'from-gray-500/20 to-gray-600/10',
                                    label: status
                                };
                                const IconComponent = config.icon;
                                return (
                                    <div
                                        key={status}
                                        className={`card bg-gradient-to-br ${config.bgColor} border-transparent cursor-pointer hover:scale-105 transition-transform`}
                                        onClick={() => handleAdvancedFilterChange('status', advancedFilters.status === status ? '' : status)}
                                    >
                                        <div className="flex items-center gap-3">
                                            <div className={`p-2 bg-black/20 rounded-lg`}>
                                                <IconComponent size={24} className={config.color} />
                                            </div>
                                            <div>
                                                <p className="text-2xl font-bold text-white">{count}</p>
                                                <p className="text-xs text-gray-400">{config.label}</p>
                                            </div>
                                        </div>
                                    </div>
                                );
                            })}
                    </div>
                )}

                {/* Summary line with Export button */}
                {logs && (
                    <div className="flex items-center justify-between text-sm text-gray-400 mb-4">
                        <div className="flex items-center gap-4">
                            <span>
                                {filteredTotal !== total ? (
                                    <span className="text-white font-medium">{filteredTotal.toLocaleString()}</span>
                                ) : (
                                    <span className="text-white font-medium">{total.toLocaleString()}</span>
                                )}
                                {filteredTotal !== total ? ` de ${total.toLocaleString()}` : ''} registros
                            </span>
                            <span className="text-gray-600">|</span>
                            <span><span className="text-emerald-400 font-medium">{interacted.toLocaleString()}</span> con interacción</span>
                            <span className="text-gray-600">|</span>
                            <span>Tasa: <span className="text-blue-400 font-medium">{((interacted / total) * 100 || 0).toFixed(1)}%</span></span>
                        </div>
                        <div className="flex items-center gap-3">
                            {/* Pagination info */}
                            {totalPages > 1 && (
                                <span>Página {currentPage} de {totalPages}</span>
                            )}
                            {/* Export button */}
                            <button
                                onClick={exportToExcel}
                                disabled={!filteredLogs.length}
                                className="btn bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-sm px-4 py-2 flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed shadow-lg shadow-emerald-500/20"
                            >
                                <Download size={16} />
                                Exportar Excel
                            </button>
                        </div>
                    </div>
                )}

                {/* Table */}
                <div className="card p-0">
                    <DataTable data={paginatedLogs} columns={columns} isLoading={isLoading} emptyMessage="Haga clic en Buscar para cargar datos" />
                </div>

                {/* Pagination Controls */}
                {totalPages > 1 && (
                    <div className="flex items-center justify-center gap-2 mt-4">
                        <button
                            onClick={() => setCurrentPage(1)}
                            disabled={currentPage === 1}
                            className="btn bg-gray-700 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed px-3 py-1 text-sm"
                        >
                            ⏮️
                        </button>
                        <button
                            onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                            disabled={currentPage === 1}
                            className="btn bg-gray-700 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed px-3 py-1 text-sm"
                        >
                            ◀️ Anterior
                        </button>

                        {/* Page numbers */}
                        <div className="flex gap-1">
                            {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                                let page;
                                if (totalPages <= 5) {
                                    page = i + 1;
                                } else if (currentPage <= 3) {
                                    page = i + 1;
                                } else if (currentPage >= totalPages - 2) {
                                    page = totalPages - 4 + i;
                                } else {
                                    page = currentPage - 2 + i;
                                }
                                return (
                                    <button
                                        key={page}
                                        onClick={() => setCurrentPage(page)}
                                        className={`px-3 py-1 text-sm rounded ${currentPage === page
                                            ? 'bg-blue-600 text-white'
                                            : 'bg-gray-700 hover:bg-gray-600 text-gray-300'
                                            }`}
                                    >
                                        {page}
                                    </button>
                                );
                            })}
                        </div>

                        <button
                            onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                            disabled={currentPage === totalPages}
                            className="btn bg-gray-700 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed px-3 py-1 text-sm"
                        >
                            Siguiente ▶️
                        </button>
                        <button
                            onClick={() => setCurrentPage(totalPages)}
                            disabled={currentPage === totalPages}
                            className="btn bg-gray-700 hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed px-3 py-1 text-sm"
                        >
                            ⏭️
                        </button>
                    </div>
                )}
            </div>
        </>
    );
}
