import { useState } from 'react';
import { Header } from '@/components/layout';
import { Modal, DataTable } from '@/components/ui';
import { useTroncales, useCreateTroncal, useDeleteTroncal } from '@/hooks/useApi';
import { Plus, Trash2 } from 'lucide-react';
import type { Troncal } from '@/types';

export function TroncalesPage() {
    const { data: troncales, isLoading } = useTroncales();
    const createMutation = useCreateTroncal();
    const deleteMutation = useDeleteTroncal();
    const [isOpen, setIsOpen] = useState(false);

    const columns = [
        { key: 'id', header: 'ID' },
        { key: 'nombre', header: 'Nombre', render: (t: Troncal) => <strong>{t.nombre}</strong> },
        { key: 'host', header: 'Host', render: (t: Troncal) => `${t.host}:${t.puerto}` },
        { key: 'usuario', header: 'Usuario', render: (t: Troncal) => t.usuario || '-' },
        { key: 'contexto', header: 'Contexto' },
        {
            key: 'actions',
            header: 'Acciones',
            render: (t: Troncal) => (
                <button onClick={() => handleDelete(t.id)} className="btn btn-danger py-1 px-3 text-sm">
                    <Trash2 size={14} />
                </button>
            ),
        },
    ];

    const handleDelete = async (id: number) => {
        if (!confirm('¿Eliminar troncal?')) return;
        await deleteMutation.mutateAsync(id);
    };

    const handleCreate = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        await createMutation.mutateAsync({
            nombre: form.get('nombre') as string,
            host: form.get('host') as string,
            puerto: Number(form.get('puerto')) || 5060,
            usuario: form.get('usuario') as string,
            password: form.get('password') as string,
            contexto: 'apicall_context',
            activo: true,
        });
        setIsOpen(false);
    };

    return (
        <>
            <Header title="Gestión de Troncales" />
            <div className="p-6">
                <div className="mb-4">
                    <button onClick={() => setIsOpen(true)} className="btn btn-primary flex items-center gap-2">
                        <Plus size={18} /> Nueva Troncal
                    </button>
                </div>
                <div className="card p-0">
                    <DataTable data={troncales || []} columns={columns} isLoading={isLoading} emptyMessage="No hay troncales" />
                </div>
            </div>

            <Modal isOpen={isOpen} onClose={() => setIsOpen(false)} title="Nueva Troncal">
                <form onSubmit={handleCreate} className="space-y-4">
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Nombre *</label>
                        <input name="nombre" className="input" required />
                    </div>
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Host / IP *</label>
                        <input name="host" className="input" required />
                    </div>
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Puerto</label>
                        <input name="puerto" type="number" defaultValue={5060} className="input" />
                    </div>
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Usuario</label>
                        <input name="usuario" className="input" />
                    </div>
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Contraseña</label>
                        <input name="password" type="password" className="input" />
                    </div>
                    <div className="flex justify-end gap-2 pt-4">
                        <button type="button" onClick={() => setIsOpen(false)} className="btn btn-secondary">Cancelar</button>
                        <button type="submit" className="btn btn-primary" disabled={createMutation.isPending}>Guardar</button>
                    </div>
                </form>
            </Modal>
        </>
    );
}
