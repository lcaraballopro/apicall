import { useState, useEffect } from 'react';
import { Header } from '@/components/layout';
import { Settings, Save, RefreshCw } from 'lucide-react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';

interface Config {
    id: number;
    key: string;
    value: string;
    description: string;
}

export function ConfigPage() {
    const queryClient = useQueryClient();

    const { data: configs, isLoading, refetch } = useQuery<Config[]>({
        queryKey: ['configs'],
        queryFn: () => api.get<Config[]>('/config'),
    });

    const updateMutation = useMutation({
        mutationFn: ({ key, value }: { key: string; value: string }) =>
            api.put<{ success: boolean }>('/config', { key, value }),
        onSuccess: () => {
            alert('✅ Configuración actualizada');
            queryClient.invalidateQueries({ queryKey: ['configs'] });
        },
        onError: () => {
            alert('❌ Error actualizando configuración');
        },
    });

    const [editValues, setEditValues] = useState<Record<string, string>>({});
    const [savedKeys, setSavedKeys] = useState<Set<string>>(new Set());

    // Initialize edit values when configs load
    useEffect(() => {
        if (configs) {
            const values: Record<string, string> = {};
            configs.forEach(c => {
                values[c.key] = c.value;
            });
            setEditValues(values);
        }
    }, [configs]);

    const handleSave = (key: string) => {
        updateMutation.mutate({ key, value: editValues[key] });
        setSavedKeys(prev => new Set(prev).add(key));
        setTimeout(() => {
            setSavedKeys(prev => {
                const next = new Set(prev);
                next.delete(key);
                return next;
            });
        }, 2000);
    };

    // Define dialer-related configs for highlighting
    const dialerConfigs = ['contacts_per_cycle', 'max_channels', 'max_per_trunk', 'max_cps'];

    // Group configs
    const dialerGroup = configs?.filter(c => dialerConfigs.includes(c.key)) || [];
    const otherGroup = configs?.filter(c => !dialerConfigs.includes(c.key)) || [];

    const ConfigCard = ({ config }: { config: Config }) => {
        const isChanged = editValues[config.key] !== config.value;
        const wasSaved = savedKeys.has(config.key);

        return (
            <div className="card p-4 flex flex-col sm:flex-row sm:items-center gap-3">
                <div className="flex-1">
                    <div className="flex items-center gap-2">
                        <code className="text-cyan-400 font-mono text-sm">{config.key}</code>
                        {wasSaved && <span className="text-green-400 text-xs">✓ Guardado</span>}
                    </div>
                    <p className="text-gray-400 text-sm mt-1">{config.description || 'Sin descripción'}</p>
                </div>
                <div className="flex items-center gap-2">
                    <input
                        type="text"
                        className="input w-32 text-center font-mono"
                        value={editValues[config.key] || ''}
                        onChange={(e) => setEditValues(prev => ({ ...prev, [config.key]: e.target.value }))}
                    />
                    <button
                        onClick={() => handleSave(config.key)}
                        disabled={updateMutation.isPending || !isChanged}
                        className={`btn ${isChanged ? 'btn-primary' : 'btn-secondary'} px-3`}
                        title="Guardar"
                    >
                        <Save size={16} />
                    </button>
                </div>
            </div>
        );
    };

    if (isLoading) {
        return (
            <>
                <Header title="Configuración del Sistema" />
                <div className="p-6 flex items-center justify-center">
                    <RefreshCw className="animate-spin text-cyan-400" size={32} />
                </div>
            </>
        );
    }

    return (
        <>
            <Header title="Configuración del Sistema" />
            <div className="p-6 max-w-4xl">
                {/* Refresh button */}
                <div className="flex justify-end mb-4">
                    <button onClick={() => refetch()} className="btn btn-secondary flex items-center gap-2">
                        <RefreshCw size={16} /> Recargar
                    </button>
                </div>

                {/* Dialer Section */}
                <div className="mb-8">
                    <div className="flex items-center gap-2 mb-4">
                        <Settings className="text-cyan-400" size={24} />
                        <h2 className="text-xl font-semibold text-white">Parámetros del Dialer</h2>
                    </div>
                    <p className="text-gray-400 mb-4 text-sm">
                        Estos parámetros controlan la agresividad del marcador. Los cambios se aplican <strong className="text-green-400">inmediatamente</strong> sin reiniciar el servicio.
                    </p>
                    <div className="space-y-3">
                        {dialerGroup.map(config => (
                            <ConfigCard key={config.id} config={config} />
                        ))}
                    </div>
                </div>

                {/* Other Settings */}
                {otherGroup.length > 0 && (
                    <div>
                        <h2 className="text-xl font-semibold text-white mb-4">Otros Parámetros</h2>
                        <div className="space-y-3">
                            {otherGroup.map(config => (
                                <ConfigCard key={config.id} config={config} />
                            ))}
                        </div>
                    </div>
                )}
            </div>
        </>
    );
}
