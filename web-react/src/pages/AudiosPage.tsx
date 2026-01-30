import { useRef, useState } from 'react';
import { Header } from '@/components/layout';
import { DataTable, Modal } from '@/components/ui';
import { useAudios, useUploadAudio, useDeleteAudio } from '@/hooks/useApi';
import { Upload, Trash2, Play, Pause, Volume2 } from 'lucide-react';
import type { AudioFile } from '@/types';

export function AudiosPage() {
    const { data: audios, isLoading } = useAudios();
    const uploadMutation = useUploadAudio();
    const deleteMutation = useDeleteAudio();
    const fileInputRef = useRef<HTMLInputElement>(null);
    const audioRef = useRef<HTMLAudioElement>(null);
    const [playingAudio, setPlayingAudio] = useState<string | null>(null);

    // Upload modal state
    const [showUploadModal, setShowUploadModal] = useState(false);
    const [selectedFile, setSelectedFile] = useState<File | null>(null);
    const [customName, setCustomName] = useState('');
    const [isUploading, setIsUploading] = useState(false);

    const handlePlay = (name: string) => {
        const token = localStorage.getItem('apicall_token');
        const audioUrl = `/api/v1/audios/stream?name=${encodeURIComponent(name)}`;

        if (playingAudio === name) {
            // Pause if same audio
            audioRef.current?.pause();
            setPlayingAudio(null);
        } else {
            // Play new audio
            if (audioRef.current) {
                audioRef.current.src = audioUrl;
                // Add auth header via fetch workaround for audio element
                fetch(audioUrl, {
                    headers: { 'Authorization': `Bearer ${token}` }
                })
                    .then(res => res.blob())
                    .then(blob => {
                        const url = URL.createObjectURL(blob);
                        if (audioRef.current) {
                            audioRef.current.src = url;
                            audioRef.current.play();
                            setPlayingAudio(name);
                        }
                    });
            }
        }
    };

    const handleAudioEnded = () => {
        setPlayingAudio(null);
    };

    const columns = [
        {
            key: 'name', header: 'Nombre', render: (a: AudioFile) => (
                <div className="flex items-center gap-2">
                    <Volume2 size={16} className="text-blue-400" />
                    <strong>{a.name}</strong>
                </div>
            )
        },
        { key: 'size', header: 'Tamaño', render: (a: AudioFile) => `${(a.size / 1024).toFixed(2)} KB` },
        { key: 'date', header: 'Fecha', render: (a: AudioFile) => new Date(a.date).toLocaleDateString() },
        {
            key: 'actions',
            header: 'Acciones',
            render: (a: AudioFile) => (
                <div className="flex gap-2">
                    <button
                        onClick={() => handlePlay(a.name)}
                        className={`btn py-1 px-3 text-sm ${playingAudio === a.name ? 'bg-green-600 hover:bg-green-500' : 'bg-blue-600 hover:bg-blue-500'}`}
                    >
                        {playingAudio === a.name ? <Pause size={14} /> : <Play size={14} />}
                    </button>
                    <button onClick={() => handleDelete(a.name)} className="btn btn-danger py-1 px-3 text-sm">
                        <Trash2 size={14} />
                    </button>
                </div>
            ),
        },
    ];

    const handleDelete = async (name: string) => {
        if (!confirm(`¿Eliminar ${name}?`)) return;
        // Stop playback if this audio is playing
        if (playingAudio === name && audioRef.current) {
            audioRef.current.pause();
            audioRef.current.src = '';
            setPlayingAudio(null);
        }
        try {
            await deleteMutation.mutateAsync(name);
        } catch {
            alert('❌ Error eliminando audio');
        }
    };

    const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) return;

        // Set file and auto-generate name from filename (without extension)
        setSelectedFile(file);
        const nameWithoutExt = file.name.replace(/\.[^.]+$/, '');
        // Sanitize: only alphanumeric, hyphen, underscore
        const sanitized = nameWithoutExt
            .toLowerCase()
            .replace(/[^a-z0-9_-]/g, '_')
            .replace(/_+/g, '_');
        setCustomName(sanitized);
        setShowUploadModal(true);

        // Reset input
        if (fileInputRef.current) fileInputRef.current.value = '';
    };

    const handleUploadSubmit = async () => {
        if (!selectedFile || !customName.trim()) return;

        // Validate name
        if (!/^[a-zA-Z0-9_-]+$/.test(customName)) {
            alert('❌ El nombre solo puede contener letras, números, guiones y guiones bajos');
            return;
        }

        const formData = new FormData();
        formData.append('audio', selectedFile);
        formData.append('name', customName.trim());

        setIsUploading(true);
        try {
            await uploadMutation.mutateAsync(formData);
            alert('✅ Audio subido y convertido correctamente');
            closeUploadModal();
        } catch {
            alert('❌ Error subiendo audio');
        } finally {
            setIsUploading(false);
        }
    };

    const closeUploadModal = () => {
        setShowUploadModal(false);
        setSelectedFile(null);
        setCustomName('');
    };

    return (
        <>
            <Header title="Gestión de Audios" />
            <div className="p-6">
                {/* Hidden audio element for playback */}
                <audio ref={audioRef} onEnded={handleAudioEnded} className="hidden" />

                <div className="mb-4">
                    <input
                        ref={fileInputRef}
                        type="file"
                        accept=".wav,.gsm,.ulaw,.alaw,.sln,.mp3,.ogg,.flac,.m4a"
                        onChange={handleFileSelect}
                        className="hidden"
                    />
                    <button onClick={() => fileInputRef.current?.click()} className="btn btn-primary flex items-center gap-2">
                        <Upload size={18} /> Subir Audio
                    </button>
                </div>
                <div className="card p-0">
                    <DataTable data={audios || []} columns={columns} isLoading={isLoading} emptyMessage="No hay audios" />
                </div>
            </div>

            {/* Upload Modal */}
            <Modal isOpen={showUploadModal} onClose={closeUploadModal} title="Subir Audio">
                <div className="space-y-4">
                    {selectedFile && (
                        <div className="p-3 bg-slate-700/50 rounded-lg">
                            <p className="text-sm text-slate-400">Archivo seleccionado:</p>
                            <p className="font-medium">{selectedFile.name}</p>
                            <p className="text-sm text-slate-400">
                                {(selectedFile.size / 1024).toFixed(2)} KB • {selectedFile.type || 'audio'}
                            </p>
                        </div>
                    )}

                    <div>
                        <label className="block text-sm font-medium mb-1">
                            Nombre del audio <span className="text-red-400">*</span>
                        </label>
                        <input
                            type="text"
                            value={customName}
                            onChange={(e) => setCustomName(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''))}
                            placeholder="mi_audio_ejemplo"
                            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                            disabled={isUploading}
                        />
                        <p className="text-xs text-slate-400 mt-1">
                            Solo letras minúsculas, números, guiones y guiones bajos. Se guardará como .wav
                        </p>
                    </div>

                    <div className="p-3 bg-blue-900/30 border border-blue-700/50 rounded-lg">
                        <p className="text-sm text-blue-300">
                            ℹ️ El audio será convertido automáticamente a formato compatible con Asterisk (8kHz, mono, 16-bit WAV)
                        </p>
                    </div>

                    <div className="flex justify-end gap-2 pt-2">
                        <button
                            onClick={closeUploadModal}
                            className="btn bg-slate-600 hover:bg-slate-500"
                            disabled={isUploading}
                        >
                            Cancelar
                        </button>
                        <button
                            onClick={handleUploadSubmit}
                            className="btn btn-primary"
                            disabled={!customName.trim() || isUploading}
                        >
                            {isUploading ? 'Subiendo...' : 'Subir Audio'}
                        </button>
                    </div>
                </div>
            </Modal>
        </>
    );
}

