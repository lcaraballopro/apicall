export interface Proyecto {
    id: number;
    nombre: string;
    caller_id: string;
    audio: string;
    dtmf_esperado: string;
    numero_desborde: string;
    troncal_salida: string;
    prefijo_salida: string;
    ips_autorizadas: string;
    max_retries: number;
    retry_time: number;
    amd_active: boolean;
    smart_cid_active: boolean;
    timezone: string;
    created_at: string;
    updated_at: string;
}

export interface Troncal {
    id: number;
    nombre: string;
    host: string;
    puerto: number;
    usuario: string;
    password: string;
    contexto: string;
    caller_id: string;
    activo: boolean;
}

export interface CallLog {
    id: number;
    proyecto_id: number;
    campaign_id: number | null;
    telefono: string;
    dtmf_marcado: string | null;
    interacciono: boolean;
    status: string;
    disposition: string | null;
    duracion: number;
    uniqueid: string;
    caller_id_used: string;
    created_at: string;
}

export interface User {
    id: number;
    username: string;
    role: string;
    full_name: string;
    active: boolean;
}

export interface AudioFile {
    name: string;
    size: number;
    date: string;
}

export interface LoginResponse {
    token: string;
    user: {
        username: string;
        role: string;
        fullName: string;
    };
}

export interface BlacklistEntry {
    id: number;
    proyecto_id: number;
    telefono: string;
    razon: string | null;
    created_at: string;
}

export interface BlacklistResponse {
    entries: BlacklistEntry[] | null;
    total: number;
}

export interface Campaign {
    id: number;
    nombre: string;
    proyecto_id: number;
    estado: 'draft' | 'active' | 'paused' | 'completed' | 'stopped';
    total_contactos: number;
    contactos_procesados: number;
    contactos_exitosos: number;
    contactos_fallidos: number;
    fecha_inicio: string | null;
    fecha_fin: string | null;
    created_at: string;
    updated_at: string;
}

export interface CampaignSchedule {
    id?: number;
    campaign_id?: number;
    dia_semana: number; // 0=Domingo, 1=Lunes, ..., 6=Sábado
    hora_inicio: string; // "HH:MM:SS"
    hora_fin: string;
    activo: boolean;
    created_at?: string;
}

export interface CampaignStats {
    campaign: Campaign;
    counts: Record<string, number>;
    in_schedule: boolean;
}

export interface DispositionCount {
    resultado: string;
    count: number;
}

export interface RecycleResult {
    success: boolean;
    new_campaign_id: number;
    contacts_copied: number;
    dispositions: string[];
}
