import { useState, useRef } from 'react';
import { Header } from '@/components/layout';
import { Modal, DataTable } from '@/components/ui';
import {
    useCampaigns, useProyectos, useCreateCampaign, useDeleteCampaign,
    useUploadCampaignCSV, useCampaignAction, useCampaignStats,
    useCampaignSchedules, useUpdateCampaignSchedules,
    useCampaignDispositions, useRecycleCampaign
} from '@/hooks/useApi';
import { Plus, Trash2, Play, Pause, Square, Upload, Clock, BarChart3, Recycle } from 'lucide-react';
import type { Campaign, CampaignSchedule } from '@/types';

const DAYS = ['Domingo', 'Lunes', 'Martes', 'Miércoles', 'Jueves', 'Viernes', 'Sábado'];
const STATE_COLORS: Record<string, string> = {
    draft: 'bg-gray-600',
    active: 'bg-green-600',
    paused: 'bg-yellow-600',
    completed: 'bg-blue-600',
    stopped: 'bg-red-600'
};

export function CampaignsPage() {
    const { data: campaigns, isLoading } = useCampaigns();
    const { data: proyectos } = useProyectos();
    const createMutation = useCreateCampaign();
    const deleteMutation = useDeleteCampaign();
    const uploadMutation = useUploadCampaignCSV();
    const actionMutation = useCampaignAction();

    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [uploadCampaignId, setUploadCampaignId] = useState<number | null>(null);
    const [statsCampaignId, setStatsCampaignId] = useState<number | null>(null);
    const [scheduleCampaignId, setScheduleCampaignId] = useState<number | null>(null);
    const [recycleCampaignId, setRecycleCampaignId] = useState<number | null>(null);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const columns = [
        { key: 'id', header: 'ID' },
        { key: 'nombre', header: 'Nombre', render: (c: Campaign) => <strong>{c.nombre}</strong> },
        {
            key: 'proyecto_id',
            header: 'Proyecto',
            render: (c: Campaign) => {
                const p = proyectos?.find(p => p.id === c.proyecto_id);
                return <span className="text-gray-300">{p?.nombre || c.proyecto_id}</span>;
            }
        },
        {
            key: 'estado',
            header: 'Estado',
            render: (c: Campaign) => (
                <span className={`px-2 py-1 rounded text-xs font-semibold ${STATE_COLORS[c.estado] || 'bg-gray-700'}`}>
                    {c.estado.toUpperCase()}
                </span>
            )
        },
        { key: 'total_contactos', header: 'Contactos' },
        {
            key: 'progress',
            header: 'Progreso',
            render: (c: Campaign) => {
                const pct = c.total_contactos > 0
                    ? Math.round(c.contactos_procesados / c.total_contactos * 100)
                    : 0;
                return (
                    <div className="w-full bg-gray-700 rounded h-2">
                        <div
                            className="bg-[hsl(var(--primary))] h-2 rounded transition-all"
                            style={{ width: `${pct}%` }}
                        />
                    </div>
                );
            }
        },
        {
            key: 'actions',
            header: 'Acciones',
            render: (c: Campaign) => (
                <div className="flex gap-1 flex-wrap">
                    {c.estado === 'draft' && (
                        <button
                            onClick={() => setUploadCampaignId(c.id)}
                            className="btn btn-secondary py-1 px-2 text-xs"
                            title="Subir CSV"
                        >
                            <Upload size={14} />
                        </button>
                    )}
                    <button
                        onClick={() => setScheduleCampaignId(c.id)}
                        className="btn btn-secondary py-1 px-2 text-xs"
                        title="Horarios"
                    >
                        <Clock size={14} />
                    </button>
                    <button
                        onClick={() => setStatsCampaignId(c.id)}
                        className="btn btn-secondary py-1 px-2 text-xs"
                        title="Estadísticas"
                    >
                        <BarChart3 size={14} />
                    </button>
                    {(c.estado === 'draft' || c.estado === 'paused') && c.total_contactos > 0 && (
                        <button
                            onClick={() => handleAction(c.id, 'start')}
                            className="btn btn-primary py-1 px-2 text-xs"
                            title="Iniciar"
                            disabled={actionMutation.isPending}
                        >
                            <Play size={14} />
                        </button>
                    )}
                    {c.estado === 'active' && (
                        <>
                            <button
                                onClick={() => handleAction(c.id, 'pause')}
                                className="btn btn-secondary py-1 px-2 text-xs"
                                title="Pausar"
                                disabled={actionMutation.isPending}
                            >
                                <Pause size={14} />
                            </button>
                            <button
                                onClick={() => handleAction(c.id, 'stop')}
                                className="btn btn-danger py-1 px-2 text-xs"
                                title="Detener"
                                disabled={actionMutation.isPending}
                            >
                                <Square size={14} />
                            </button>
                        </>
                    )}
                    {(c.estado === 'draft' || c.estado === 'completed' || c.estado === 'stopped') && (
                        <button
                            onClick={() => handleDelete(c.id)}
                            className="btn btn-danger py-1 px-2 text-xs"
                            title="Eliminar"
                            disabled={deleteMutation.isPending}
                        >
                            <Trash2 size={14} />
                        </button>
                    )}
                    {(c.estado === 'completed' || c.estado === 'stopped') && c.contactos_procesados > 0 && (
                        <button
                            onClick={() => setRecycleCampaignId(c.id)}
                            className="btn btn-secondary py-1 px-2 text-xs"
                            title="Reciclar Contactos"
                        >
                            <Recycle size={14} />
                        </button>
                    )}
                </div>
            ),
        },
    ];

    const handleAction = async (campaignId: number, action: 'start' | 'pause' | 'stop') => {
        await actionMutation.mutateAsync({ campaign_id: campaignId, action });
    };

    const handleDelete = async (id: number) => {
        if (!confirm('¿Eliminar campaña?')) return;
        await deleteMutation.mutateAsync(id);
    };

    const handleCreate = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        await createMutation.mutateAsync({
            nombre: form.get('nombre') as string,
            proyecto_id: Number(form.get('proyecto_id')),
        });
        setIsCreateOpen(false);
    };

    const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file || !uploadCampaignId) return;

        await uploadMutation.mutateAsync({ campaignId: uploadCampaignId, file });
        setUploadCampaignId(null);
        if (fileInputRef.current) fileInputRef.current.value = '';
    };

    return (
        <>
            <Header title="Campañas Masivas" />
            <div className="p-6">
                <div className="mb-4">
                    <button onClick={() => setIsCreateOpen(true)} className="btn btn-primary flex items-center gap-2">
                        <Plus size={18} /> Nueva Campaña
                    </button>
                </div>

                <div className="card p-0">
                    <DataTable data={campaigns || []} columns={columns} isLoading={isLoading} emptyMessage="No hay campañas" />
                </div>
            </div>

            {/* Create Modal */}
            <Modal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} title="Nueva Campaña">
                <form onSubmit={handleCreate} className="space-y-4">
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Nombre *</label>
                        <input name="nombre" className="input w-full" required />
                    </div>
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Proyecto *</label>
                        <select name="proyecto_id" className="input w-full" required>
                            <option value="">Seleccionar...</option>
                            {proyectos?.map(p => (
                                <option key={p.id} value={p.id}>{p.nombre}</option>
                            ))}
                        </select>
                    </div>
                    <div className="flex justify-end gap-2 pt-4">
                        <button type="button" onClick={() => setIsCreateOpen(false)} className="btn btn-secondary">Cancelar</button>
                        <button type="submit" className="btn btn-primary" disabled={createMutation.isPending}>
                            {createMutation.isPending ? 'Creando...' : 'Crear Campaña'}
                        </button>
                    </div>
                </form>
            </Modal>

            {/* Upload CSV Modal */}
            <Modal isOpen={!!uploadCampaignId} onClose={() => setUploadCampaignId(null)} title="Subir Contactos CSV">
                <div className="space-y-4">
                    <p className="text-gray-300">Sube un archivo CSV con números de teléfono. Formato: un número por línea o separado por comas/punto y coma.</p>
                    <input
                        ref={fileInputRef}
                        type="file"
                        accept=".csv,.txt"
                        onChange={handleUpload}
                        className="input w-full"
                    />
                    {uploadMutation.isPending && <p className="text-[hsl(var(--primary))]">Subiendo...</p>}
                </div>
            </Modal>

            {/* Stats Modal */}
            {statsCampaignId && (
                <StatsModal campaignId={statsCampaignId} onClose={() => setStatsCampaignId(null)} />
            )}

            {/* Schedule Modal */}
            {scheduleCampaignId && (
                <ScheduleModal campaignId={scheduleCampaignId} onClose={() => setScheduleCampaignId(null)} />
            )}

            {/* Recycle Modal */}
            {recycleCampaignId && (
                <RecycleModal campaignId={recycleCampaignId} onClose={() => setRecycleCampaignId(null)} />
            )}
        </>
    );
}

function StatsModal({ campaignId, onClose }: { campaignId: number; onClose: () => void }) {
    const { data: stats, isLoading } = useCampaignStats(campaignId);

    return (
        <Modal isOpen onClose={onClose} title="Estadísticas de Campaña" size="lg">
            {isLoading ? (
                <p>Cargando...</p>
            ) : stats ? (
                <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                        <div className="bg-gray-800 p-4 rounded">
                            <div className="text-2xl font-bold text-[hsl(var(--primary))]">{stats.campaign.total_contactos}</div>
                            <div className="text-sm text-gray-400">Total Contactos</div>
                        </div>
                        <div className="bg-gray-800 p-4 rounded">
                            <div className="text-2xl font-bold text-green-500">{stats.counts.completed || 0}</div>
                            <div className="text-sm text-gray-400">Exitosos</div>
                        </div>
                        <div className="bg-gray-800 p-4 rounded">
                            <div className="text-2xl font-bold text-red-500">{stats.counts.failed || 0}</div>
                            <div className="text-sm text-gray-400">Fallidos</div>
                        </div>
                        <div className="bg-gray-800 p-4 rounded">
                            <div className="text-2xl font-bold text-yellow-500">{stats.counts.pending || 0}</div>
                            <div className="text-sm text-gray-400">Pendientes</div>
                        </div>
                    </div>
                    <div className={`p-3 rounded text-center ${stats.in_schedule ? 'bg-green-900' : 'bg-gray-800'}`}>
                        {stats.in_schedule ? '✅ Dentro del horario activo' : '⏸️ Fuera del horario programado'}
                    </div>
                </div>
            ) : (
                <p>No hay datos</p>
            )}
        </Modal>
    );
}

function ScheduleModal({ campaignId, onClose }: { campaignId: number; onClose: () => void }) {
    const { data: schedules, isLoading } = useCampaignSchedules(campaignId);
    const updateMutation = useUpdateCampaignSchedules();

    const [localSchedules, setLocalSchedules] = useState<CampaignSchedule[]>([]);

    // Initialize from data
    useState(() => {
        if (schedules) {
            setLocalSchedules(schedules);
        } else {
            // Default: Mon-Fri 9-18
            setLocalSchedules(
                [1, 2, 3, 4, 5].map(d => ({
                    dia_semana: d,
                    hora_inicio: '09:00:00',
                    hora_fin: '18:00:00',
                    activo: true
                }))
            );
        }
    });

    const toggleDay = (day: number) => {
        const existing = localSchedules.find(s => s.dia_semana === day);
        if (existing) {
            setLocalSchedules(prev => prev.filter(s => s.dia_semana !== day));
        } else {
            setLocalSchedules(prev => [...prev, { dia_semana: day, hora_inicio: '09:00:00', hora_fin: '18:00:00', activo: true }]);
        }
    };

    const handleSave = async () => {
        await updateMutation.mutateAsync({ campaignId, schedules: localSchedules });
        onClose();
    };

    return (
        <Modal isOpen onClose={onClose} title="Horario de Campaña" size="lg">
            {isLoading ? (
                <p>Cargando...</p>
            ) : (
                <div className="space-y-4">
                    <p className="text-gray-400 text-sm">Selecciona los días y horas en que la campaña estará activa:</p>
                    <div className="grid gap-2">
                        {DAYS.map((day, index) => {
                            const schedule = localSchedules.find(s => s.dia_semana === index);
                            return (
                                <div key={index} className="flex items-center gap-4 p-3 bg-gray-800 rounded">
                                    <label className="flex items-center gap-2 min-w-[120px]">
                                        <input
                                            type="checkbox"
                                            checked={!!schedule}
                                            onChange={() => toggleDay(index)}
                                            className="w-4 h-4"
                                        />
                                        {day}
                                    </label>
                                    {schedule && (
                                        <>
                                            <input
                                                type="time"
                                                value={schedule.hora_inicio.substring(0, 5)}
                                                onChange={(e) => {
                                                    setLocalSchedules(prev => prev.map(s =>
                                                        s.dia_semana === index
                                                            ? { ...s, hora_inicio: e.target.value + ':00' }
                                                            : s
                                                    ));
                                                }}
                                                className="input"
                                            />
                                            <span>a</span>
                                            <input
                                                type="time"
                                                value={schedule.hora_fin.substring(0, 5)}
                                                onChange={(e) => {
                                                    setLocalSchedules(prev => prev.map(s =>
                                                        s.dia_semana === index
                                                            ? { ...s, hora_fin: e.target.value + ':00' }
                                                            : s
                                                    ));
                                                }}
                                                className="input"
                                            />
                                        </>
                                    )}
                                </div>
                            );
                        })}
                    </div>
                    <div className="flex justify-end gap-2 pt-4">
                        <button onClick={onClose} className="btn btn-secondary">Cancelar</button>
                        <button
                            onClick={handleSave}
                            className="btn btn-primary"
                            disabled={updateMutation.isPending}
                        >
                            {updateMutation.isPending ? 'Guardando...' : '💾 Guardar Horario'}
                        </button>
                    </div>
                </div>
            )}
        </Modal>
    );
}

// Mapping of disposition codes to human-readable labels
const DISPOSITION_LABELS: Record<string, string> = {
    'PENDING': 'Pendiente',
    'A': 'Contestada',
    'AM': 'Máquina Contestadora',
    'NA': 'No Contesta',
    'B': 'Ocupado',
    'N': 'Inválido/No Existe'
};

function RecycleModal({ campaignId, onClose }: { campaignId: number; onClose: () => void }) {
    const { data: dispositions, isLoading } = useCampaignDispositions(campaignId);
    const recycleMutation = useRecycleCampaign();
    const [selectedDispositions, setSelectedDispositions] = useState<string[]>([]);
    const [newName, setNewName] = useState('');

    const toggleDisposition = (resultado: string) => {
        setSelectedDispositions(prev =>
            prev.includes(resultado)
                ? prev.filter(d => d !== resultado)
                : [...prev, resultado]
        );
    };

    const selectedCount = dispositions
        ?.filter(d => selectedDispositions.includes(d.resultado))
        .reduce((sum, d) => sum + d.count, 0) || 0;

    const handleRecycle = async () => {
        if (!newName.trim() || selectedDispositions.length === 0) return;

        try {
            await recycleMutation.mutateAsync({
                campaign_id: campaignId,
                nombre: newName.trim(),
                dispositions: selectedDispositions
            });
            onClose();
        } catch (error) {
            console.error('Error recycling campaign:', error);
        }
    };

    return (
        <Modal isOpen onClose={onClose} title="♻️ Reciclar Contactos" size="md">
            {isLoading ? (
                <p>Cargando disposiciones...</p>
            ) : (
                <div className="space-y-4">
                    <p className="text-gray-300 text-sm">
                        Selecciona las disposiciones de llamada a reciclar. Se creará una nueva campaña con los contactos seleccionados.
                    </p>

                    <div className="space-y-2 max-h-64 overflow-y-auto">
                        {dispositions?.map(d => (
                            <label
                                key={d.resultado}
                                className={`flex items-center justify-between p-3 rounded cursor-pointer transition-colors ${selectedDispositions.includes(d.resultado)
                                    ? 'bg-[hsl(var(--primary)/0.2)] border border-[hsl(var(--primary))]'
                                    : 'bg-gray-800 hover:bg-gray-700'
                                    }`}
                            >
                                <div className="flex items-center gap-3">
                                    <input
                                        type="checkbox"
                                        checked={selectedDispositions.includes(d.resultado)}
                                        onChange={() => toggleDisposition(d.resultado)}
                                        className="w-4 h-4"
                                    />
                                    <span className="font-medium">
                                        {DISPOSITION_LABELS[d.resultado] || d.resultado}
                                    </span>
                                </div>
                                <span className="bg-gray-700 px-2 py-1 rounded text-sm font-semibold">
                                    {d.count.toLocaleString()} registros
                                </span>
                            </label>
                        ))}
                    </div>

                    {selectedCount > 0 && (
                        <div className="bg-[hsl(var(--primary)/0.1)] p-3 rounded text-center">
                            <span className="text-[hsl(var(--primary))] font-semibold">
                                {selectedCount.toLocaleString()} contactos seleccionados
                            </span>
                        </div>
                    )}

                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Nombre nueva campaña *</label>
                        <input
                            type="text"
                            value={newName}
                            onChange={(e) => setNewName(e.target.value)}
                            placeholder="Ej: Reciclada - No Contesta"
                            className="input w-full"
                        />
                    </div>

                    <div className="flex justify-end gap-2 pt-4">
                        <button onClick={onClose} className="btn btn-secondary">Cancelar</button>
                        <button
                            onClick={handleRecycle}
                            className="btn btn-primary"
                            disabled={recycleMutation.isPending || !newName.trim() || selectedDispositions.length === 0}
                        >
                            {recycleMutation.isPending ? 'Creando...' : `♻️ Reciclar (${selectedCount.toLocaleString()})`}
                        </button>
                    </div>
                </div>
            )}
        </Modal>
    );
}
