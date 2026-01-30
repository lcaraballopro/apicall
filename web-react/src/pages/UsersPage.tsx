import { useState } from 'react';
import { Header } from '@/components/layout';
import { Modal, DataTable } from '@/components/ui';
import { useUsers, useCreateUser, useDeleteUser } from '@/hooks/useApi';
import { Plus, Trash2 } from 'lucide-react';
import type { User } from '@/types';

export function UsersPage() {
    const { data: users, isLoading } = useUsers();
    const createMutation = useCreateUser();
    const deleteMutation = useDeleteUser();
    const [isOpen, setIsOpen] = useState(false);

    const columns = [
        { key: 'username', header: 'Usuario' },
        { key: 'full_name', header: 'Nombre' },
        { key: 'role', header: 'Rol', render: (u: User) => <span className="badge">{u.role}</span> },
        { key: 'active', header: 'Estado', render: (u: User) => u.active ? '🟢' : '🔴' },
        {
            key: 'actions',
            header: 'Acciones',
            render: (u: User) => (
                <button onClick={() => handleDelete(u.id)} className="btn btn-danger py-1 px-3 text-sm">
                    <Trash2 size={14} />
                </button>
            ),
        },
    ];

    const handleDelete = async (id: number) => {
        if (!confirm('¿Eliminar usuario?')) return;
        await deleteMutation.mutateAsync(id);
    };

    const handleCreate = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const form = new FormData(e.currentTarget);
        await createMutation.mutateAsync({
            username: form.get('username') as string,
            password: form.get('password') as string,
            full_name: form.get('full_name') as string,
            role: form.get('role') as string,
        });
        setIsOpen(false);
    };

    return (
        <>
            <Header title="Gestión de Usuarios" />
            <div className="p-6">
                <div className="mb-4">
                    <button onClick={() => setIsOpen(true)} className="btn btn-primary flex items-center gap-2">
                        <Plus size={18} /> Nuevo Usuario
                    </button>
                </div>
                <div className="card p-0">
                    <DataTable data={users || []} columns={columns} isLoading={isLoading} emptyMessage="No hay usuarios" />
                </div>
            </div>

            <Modal isOpen={isOpen} onClose={() => setIsOpen(false)} title="Nuevo Usuario">
                <form onSubmit={handleCreate} className="space-y-4">
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Usuario *</label>
                        <input name="username" className="input" required />
                    </div>
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Contraseña *</label>
                        <input name="password" type="password" className="input" required />
                    </div>
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Nombre Completo *</label>
                        <input name="full_name" className="input" required />
                    </div>
                    <div>
                        <label className="block text-sm text-gray-300 mb-1">Rol</label>
                        <select name="role" className="input">
                            <option value="admin">Administrador</option>
                            <option value="supervisor">Supervisor</option>
                            <option value="viewer">Solo Lectura</option>
                        </select>
                    </div>
                    <div className="flex justify-end gap-2 pt-4">
                        <button type="button" onClick={() => setIsOpen(false)} className="btn btn-secondary">Cancelar</button>
                        <button type="submit" className="btn btn-primary" disabled={createMutation.isPending}>Crear</button>
                    </div>
                </form>
            </Modal>
        </>
    );
}
