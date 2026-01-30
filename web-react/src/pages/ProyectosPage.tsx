import { useState } from 'react';
import { Header } from '@/components/layout';
import { Modal, DataTable, BlacklistSection } from '@/components/ui';
import { useProyectos, useCreateProyecto, useUpdateProyecto, useDeleteProyecto } from '@/hooks/useApi';
import { Plus, Pencil, Trash2 } from 'lucide-react';
import type { Proyecto } from '@/types';

export function ProyectosPage() {
    const { data: proyectos, isLoading } = useProyectos();
    const createMutation = useCreateProyecto();
    const updateMutation = useUpdateProyecto();
    const deleteMutation = useDeleteProyecto();

    const [isCreateOpen, setIsCreateOpen] = useState(false);
    const [editingProject, setEditingProject] = useState<Proyecto | null>(null);

    const columns = [
        { key: 'id', header: 'ID' },
        { key: 'nombre', header: 'Nombre', render: (p: Proyecto) => <strong>{p.nombre}</strong> },
        { key: 'caller_id', header: 'Caller ID' },
        { key: 'troncal_salida', header: 'Troncal', render: (p: Proyecto) => <span className="badge">{p.troncal_salida}</span> },
        { key: 'amd_active', header: 'AMD', render: (p: Proyecto) => p.amd_active ? '✅' : '❌' },
        { key: 'smart_cid_active', header: 'Smart CID', render: (p: Proyecto) => p.smart_cid_active ? '✅' : '❌' },
        {
            key: 'actions',
            header: 'Acciones',
            render: (p: Proyecto) => (
                <div className="flex gap-2">
                    <button onClick={() => setEditingProject(p)} className="btn btn-primary py-1 px-3 text-sm">
                        <Pencil size={14} />
                    </button>
                    <button
                        onClick={() => handleDelete(p.id)}
                        className="btn btn-danger py-1 px-3 text-sm"
                        disabled={deleteMutation.isPending}
                    >
                        <Trash2 size={14} />
                    </button>
                </div>
            ),
        },
    ];

    const handleDelete = async (id: number) => {
        if (!confirm('¿Eliminar proyecto?')) return;
        await deleteMutation.mutateAsync(id);
    };

    const handleCreate = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        await createMutation.mutateAsync({
            id: Number(form.get('id')),
            nombre: form.get('nombre') as string,
            caller_id: form.get('caller_id') as string,
            troncal_salida: form.get('troncal_salida') as string,
            audio: form.get('audio') as string,
            amd_active: form.get('amd_active') === 'on',
            smart_cid_active: form.get('smart_cid_active') === 'on',
        });
        setIsCreateOpen(false);
    };

    const handleUpdate = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        await updateMutation.mutateAsync({
            id: Number(form.get('id')),
            nombre: form.get('nombre') as string,
            caller_id: form.get('caller_id') as string,
            audio: form.get('audio') as string,
            dtmf_esperado: form.get('dtmf_esperado') as string,
            numero_desborde: form.get('numero_desborde') as string,
            troncal_salida: form.get('troncal_salida') as string,
            prefijo_salida: form.get('prefijo_salida') as string,
            ips_autorizadas: form.get('ips_autorizadas') as string,
            max_retries: Number(form.get('max_retries')),
            retry_time: Number(form.get('retry_time')),
            amd_active: form.get('amd_active') === 'on',
            smart_cid_active: form.get('smart_cid_active') === 'on',
            timezone: form.get('timezone') as string,
        });
        setEditingProject(null);
    };

    return (
        <>
            <Header title="Gestión de Proyectos" />
            <div className="p-6">
                <div className="mb-4">
                    <button onClick={() => setIsCreateOpen(true)} className="btn btn-primary flex items-center gap-2">
                        <Plus size={18} /> Nuevo Proyecto
                    </button>
                </div>

                <div className="card p-0">
                    <DataTable data={proyectos || []} columns={columns} isLoading={isLoading} emptyMessage="No hay proyectos" />
                </div>
            </div>

            {/* Create Modal */}
            <Modal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} title="Nuevo Proyecto">
                <form onSubmit={handleCreate} className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm text-gray-300 mb-1">ID *</label>
                            <input name="id" type="number" className="input" required />
                        </div>
                        <div>
                            <label className="block text-sm text-gray-300 mb-1">Nombre *</label>
                            <input name="nombre" className="input" required />
                        </div>
                        <div>
                            <label className="block text-sm text-gray-300 mb-1">Caller ID</label>
                            <input name="caller_id" className="input" />
                        </div>
                        <div>
                            <label className="block text-sm text-gray-300 mb-1">Troncal</label>
                            <input name="troncal_salida" className="input" />
                        </div>
                        <div className="col-span-2">
                            <label className="block text-sm text-gray-300 mb-1">Audio</label>
                            <input name="audio" className="input" />
                        </div>
                    </div>
                    <div className="flex gap-4">
                        <label className="flex items-center gap-2 text-gray-300">
                            <input type="checkbox" name="amd_active" className="w-4 h-4" /> AMD
                        </label>
                        <label className="flex items-center gap-2 text-gray-300">
                            <input type="checkbox" name="smart_cid_active" className="w-4 h-4" /> Smart CID
                        </label>
                    </div>
                    <div className="flex justify-end gap-2 pt-4">
                        <button type="button" onClick={() => setIsCreateOpen(false)} className="btn btn-secondary">Cancelar</button>
                        <button type="submit" className="btn btn-primary" disabled={createMutation.isPending}>
                            {createMutation.isPending ? 'Guardando...' : 'Guardar'}
                        </button>
                    </div>
                </form>
            </Modal>

            {/* Edit Modal */}
            <Modal isOpen={!!editingProject} onClose={() => setEditingProject(null)} title="Editar Proyecto" size="lg">
                {editingProject && (
                    <form onSubmit={handleUpdate} className="space-y-4">
                        <input type="hidden" name="id" value={editingProject.id} />

                        <h4 className="text-[hsl(var(--primary))] font-medium">Información Básica</h4>
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Nombre *</label>
                                <input name="nombre" defaultValue={editingProject.nombre} className="input" required />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Caller ID</label>
                                <input name="caller_id" defaultValue={editingProject.caller_id} className="input" />
                            </div>
                        </div>

                        <h4 className="text-[hsl(var(--primary))] font-medium">Configuración</h4>
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Audio</label>
                                <input name="audio" defaultValue={editingProject.audio} className="input" />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">DTMF Esperado</label>
                                <input name="dtmf_esperado" defaultValue={editingProject.dtmf_esperado} className="input" />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Troncal</label>
                                <input name="troncal_salida" defaultValue={editingProject.troncal_salida} className="input" />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Prefijo</label>
                                <input name="prefijo_salida" defaultValue={editingProject.prefijo_salida} className="input" />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Desborde</label>
                                <input name="numero_desborde" defaultValue={editingProject.numero_desborde} className="input" />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">IPs Autorizadas</label>
                                <input name="ips_autorizadas" defaultValue={editingProject.ips_autorizadas} className="input" />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Reintentos</label>
                                <input name="max_retries" type="number" defaultValue={editingProject.max_retries} className="input" />
                            </div>
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Tiempo Reintento (s)</label>
                                <input name="retry_time" type="number" defaultValue={editingProject.retry_time} className="input" />
                            </div>
                        </div>

                        <h4 className="text-[hsl(var(--primary))] font-medium">Funciones</h4>
                        <div className="flex gap-4 flex-wrap">
                            <label className="flex items-center gap-2 text-gray-300">
                                <input type="checkbox" name="amd_active" defaultChecked={editingProject.amd_active} className="w-4 h-4" /> AMD
                            </label>
                            <label className="flex items-center gap-2 text-gray-300">
                                <input type="checkbox" name="smart_cid_active" defaultChecked={editingProject.smart_cid_active} className="w-4 h-4" /> Smart CID
                            </label>
                            <div className="flex items-center gap-2">
                                <label className="text-gray-300">Zona Horaria:</label>
                                <select name="timezone" defaultValue={editingProject.timezone || 'America/Bogota'} className="input py-1 px-2">
                                    <option value="America/Bogota">Colombia/Perú/Ecuador (GMT-5)</option>
                                    <option value="America/Mexico_City">México (GMT-6)</option>
                                    <option value="America/Buenos_Aires">Argentina (GMT-3)</option>
                                    <option value="America/Santiago">Chile (GMT-3/-4)</option>
                                    <option value="America/Lima">Perú (GMT-5)</option>
                                    <option value="America/Caracas">Venezuela (GMT-4)</option>
                                    <option value="America/New_York">USA Este (GMT-5)</option>
                                    <option value="America/Los_Angeles">USA Oeste (GMT-8)</option>
                                    <option value="Europe/Madrid">España (GMT+1)</option>
                                </select>
                            </div>
                        </div>

                        {/* Blacklist Section */}
                        <BlacklistSection proyectoId={editingProject.id} />

                        <div className="flex justify-end gap-2 pt-4 border-t border-[hsl(var(--border))]">
                            <button type="button" onClick={() => setEditingProject(null)} className="btn btn-secondary">Cancelar</button>
                            <button type="submit" className="btn btn-primary" disabled={updateMutation.isPending}>
                                {updateMutation.isPending ? 'Guardando...' : '💾 Guardar Cambios'}
                            </button>
                        </div>
                    </form>
                )}
            </Modal>
        </>
    );
}
