import { useState, useRef } from 'react';
import { Trash2, Upload, X } from 'lucide-react';
import { useBlacklist, useAddToBlacklist, useUploadBlacklist, useDeleteFromBlacklist, useClearBlacklist } from '@/hooks/useApi';

interface BlacklistSectionProps {
    proyectoId: number;
}

export function BlacklistSection({ proyectoId }: BlacklistSectionProps) {
    const { data, isLoading } = useBlacklist(proyectoId);
    const addMutation = useAddToBlacklist();
    const uploadMutation = useUploadBlacklist();
    const deleteMutation = useDeleteFromBlacklist();
    const clearMutation = useClearBlacklist();

    const [telefono, setTelefono] = useState('');
    const fileInputRef = useRef<HTMLInputElement>(null);

    const entries = data?.entries || [];
    const total = data?.total || 0;

    const handleAdd = async () => {
        if (!telefono.trim()) return;
        await addMutation.mutateAsync({ proyecto_id: proyectoId, telefono: telefono.trim() });
        setTelefono('');
    };

    const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) return;

        try {
            const result = await uploadMutation.mutateAsync({ proyectoId, file });
            const data = result as { imported?: number; total?: number };
            alert(`✅ Importados ${data.imported || 0} de ${data.total || 0} números`);
        } catch {
            alert('❌ Error importando CSV');
        }

        if (fileInputRef.current) {
            fileInputRef.current.value = '';
        }
    };

    const handleDelete = async (id: number) => {
        await deleteMutation.mutateAsync({ id, proyectoId });
    };

    const handleClear = async () => {
        if (!confirm('¿Eliminar TODOS los números de la lista negra?')) return;
        await clearMutation.mutateAsync(proyectoId);
        alert('✅ Lista negra limpiada');
    };

    return (
        <div className="mt-4">
            <h4 className="text-red-400 font-medium flex items-center gap-2">
                🚫 Lista Negra (Blacklist)
            </h4>
            <p className="text-sm text-gray-400 mb-3">
                Números bloqueados que no recibirán llamadas.
            </p>

            {/* Add/Upload Row */}
            <div className="flex gap-2 mb-3">
                <input
                    type="text"
                    value={telefono}
                    onChange={(e) => setTelefono(e.target.value)}
                    placeholder="Teléfono"
                    className="input flex-1"
                    onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), handleAdd())}
                />
                <button
                    type="button"
                    onClick={handleAdd}
                    className="btn btn-secondary"
                    disabled={addMutation.isPending}
                >
                    + Agregar
                </button>
                <button
                    type="button"
                    onClick={() => fileInputRef.current?.click()}
                    className="btn btn-secondary flex items-center gap-1"
                    disabled={uploadMutation.isPending}
                >
                    <Upload size={14} /> CSV
                </button>
                <input
                    ref={fileInputRef}
                    type="file"
                    accept=".csv,.txt"
                    className="hidden"
                    onChange={handleUpload}
                />
            </div>

            {/* Stats */}
            <div className="flex items-center justify-between mb-2 text-sm text-gray-400">
                <span>Total: <strong>{total}</strong> números bloqueados</span>
                <button
                    type="button"
                    onClick={handleClear}
                    className="text-red-400 hover:text-red-300 flex items-center gap-1"
                    disabled={clearMutation.isPending || total === 0}
                >
                    <X size={14} /> Limpiar Lista
                </button>
            </div>

            {/* Table */}
            <div className="max-h-48 overflow-y-auto border border-[hsl(var(--border))] rounded-lg">
                <table className="w-full text-sm">
                    <thead className="sticky top-0 bg-[hsl(var(--card))]">
                        <tr className="border-b border-[hsl(var(--border))]">
                            <th className="text-left p-2 text-gray-300">Teléfono</th>
                            <th className="text-left p-2 text-gray-300">Razón</th>
                            <th className="text-left p-2 text-gray-300">Fecha</th>
                            <th className="p-2 w-12"></th>
                        </tr>
                    </thead>
                    <tbody>
                        {isLoading ? (
                            <tr><td colSpan={4} className="text-center p-4 text-gray-400">Cargando...</td></tr>
                        ) : entries.length === 0 ? (
                            <tr><td colSpan={4} className="text-center p-4 text-gray-400">No hay números bloqueados</td></tr>
                        ) : (
                            entries.map((entry) => (
                                <tr key={entry.id} className="border-b border-[hsl(var(--border))] hover:bg-[hsl(var(--border)/0.3)]">
                                    <td className="p-2 text-white">{entry.telefono}</td>
                                    <td className="p-2 text-gray-400">{entry.razon || '-'}</td>
                                    <td className="p-2 text-gray-400">{new Date(entry.created_at).toLocaleDateString()}</td>
                                    <td className="p-2">
                                        <button
                                            type="button"
                                            onClick={() => handleDelete(entry.id)}
                                            className="text-red-400 hover:text-red-300"
                                            disabled={deleteMutation.isPending}
                                        >
                                            <Trash2 size={14} />
                                        </button>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    );
}
