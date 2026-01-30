import { useState } from 'react';
import { Header } from '@/components/layout';
import { useProyectos, useTestCall } from '@/hooks/useApi';
import { PhoneCall, CheckCircle, XCircle } from 'lucide-react';

export function TestCallPage() {
    const { data: proyectos } = useProyectos();
    const testCall = useTestCall();
    const [proyectoId, setProyectoId] = useState('');
    const [telefono, setTelefono] = useState('');
    const [result, setResult] = useState<{ success: boolean; message: string } | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setResult(null);

        try {
            await testCall.mutateAsync({
                proyecto_id: Number(proyectoId),
                telefono,
            });
            setResult({ success: true, message: '✅ Llamada encolada correctamente' });
        } catch {
            setResult({ success: false, message: '❌ Error generando llamada' });
        }
    };

    return (
        <>
            <Header title="Probar Llamada" />
            <div className="p-6">
                <div className="max-w-xl mx-auto">
                    <div className="card">
                        <div className="flex items-center gap-3 mb-6">
                            <div className="w-12 h-12 bg-gradient-to-br from-green-500 to-green-600 rounded-xl flex items-center justify-center">
                                <PhoneCall size={24} className="text-white" />
                            </div>
                            <div>
                                <h3 className="text-lg font-semibold text-white">Generar Llamada de Prueba</h3>
                                <p className="text-sm text-gray-400">Envía una llamada a un número de destino</p>
                            </div>
                        </div>

                        <form onSubmit={handleSubmit} className="space-y-4">
                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Proyecto</label>
                                <select
                                    value={proyectoId}
                                    onChange={(e) => setProyectoId(e.target.value)}
                                    className="input"
                                    required
                                >
                                    <option value="">Seleccione...</option>
                                    {proyectos?.map((p) => (
                                        <option key={p.id} value={p.id}>
                                            {p.nombre} (#{p.id})
                                        </option>
                                    ))}
                                </select>
                            </div>

                            <div>
                                <label className="block text-sm text-gray-300 mb-1">Número Destino</label>
                                <input
                                    type="text"
                                    value={telefono}
                                    onChange={(e) => setTelefono(e.target.value)}
                                    placeholder="Ej: 573001234567"
                                    className="input"
                                    required
                                />
                            </div>

                            <button
                                type="submit"
                                disabled={testCall.isPending}
                                className="btn btn-primary w-full py-3 flex items-center justify-center gap-2"
                            >
                                <PhoneCall size={18} />
                                {testCall.isPending ? 'Enviando...' : 'Llamar Ahora'}
                            </button>
                        </form>

                        {result && (
                            <div
                                className={`mt-4 p-4 rounded-lg flex items-center gap-3 ${result.success ? 'bg-green-500/10 text-green-400' : 'bg-red-500/10 text-red-400'
                                    }`}
                            >
                                {result.success ? <CheckCircle size={20} /> : <XCircle size={20} />}
                                {result.message}
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </>
    );
}
