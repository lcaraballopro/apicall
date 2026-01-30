import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import type { Proyecto, Troncal, CallLog, User, AudioFile, LoginResponse, BlacklistEntry } from '@/types';

// Auth
export function useLogin() {
    return useMutation({
        mutationFn: (credentials: { username: string; password: string }) =>
            api.post<LoginResponse>('/login', credentials),
    });
}

// Proyectos
export function useProyectos() {
    return useQuery({
        queryKey: ['proyectos'],
        queryFn: () => api.get<Proyecto[]>('/proyectos'),
    });
}

export function useCreateProyecto() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: Partial<Proyecto>) => api.post<Proyecto>('/proyectos', data),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['proyectos'] }),
    });
}

export function useUpdateProyecto() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: Partial<Proyecto>) => api.put<Proyecto>('/proyectos', data),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['proyectos'] }),
    });
}

export function useDeleteProyecto() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: number) => api.delete(`/proyectos/delete?id=${id}`),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['proyectos'] }),
    });
}

// Troncales
export function useTroncales() {
    return useQuery({
        queryKey: ['troncales'],
        queryFn: () => api.get<Troncal[]>('/troncales'),
    });
}

export function useCreateTroncal() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: Partial<Troncal>) => api.post<Troncal>('/troncales', data),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['troncales'] }),
    });
}

export function useDeleteTroncal() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: number) => api.delete(`/troncales/delete?id=${id}`),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['troncales'] }),
    });
}

// Call Logs
export function useCallLogs(filters?: { proyecto_id?: number; campaign_id?: number | string; from_date?: string; to_date?: string; limit?: number; refetchInterval?: number }) {
    const params = new URLSearchParams();
    if (filters?.proyecto_id) params.set('proyecto_id', String(filters.proyecto_id));
    if (filters?.campaign_id) params.set('campaign_id', String(filters.campaign_id));
    if (filters?.from_date) params.set('from_date', filters.from_date);
    if (filters?.to_date) params.set('to_date', filters.to_date);
    params.set('limit', String(filters?.limit || 5000));

    return useQuery({
        queryKey: ['logs', filters],
        queryFn: () => api.get<CallLog[]>(`/logs?${params.toString()}`),
        enabled: false, // Manual trigger
        refetchInterval: filters?.refetchInterval,
    });
}

// Real-time logs for dashboard (auto-fetches)
export function useRealtimeLogs(limit: number = 1000, refetchInterval: number = 3000) {
    return useQuery({
        queryKey: ['logs-realtime', limit],
        queryFn: () => api.get<CallLog[]>(`/logs?limit=${limit}`),
        refetchInterval,
        staleTime: 1000,
    });
}

// Test Call
export function useTestCall() {
    return useMutation({
        mutationFn: (data: { proyecto_id: number; telefono: string }) =>
            api.post('/call', data),
    });
}

// Users
export function useUsers() {
    return useQuery({
        queryKey: ['users'],
        queryFn: () => api.get<User[]>('/users'),
    });
}

export function useCreateUser() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: { username: string; password: string; role: string; full_name: string }) =>
            api.post('/users', data),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
    });
}

export function useDeleteUser() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: number) => api.get(`/users/delete?id=${id}`),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['users'] }),
    });
}

// Audios
export function useAudios() {
    return useQuery({
        queryKey: ['audios'],
        queryFn: () => api.get<AudioFile[]>('/audios'),
    });
}

export function useUploadAudio() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (formData: FormData) => api.upload('/audios/upload', formData),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['audios'] }),
    });
}

export function useDeleteAudio() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (name: string) => api.delete(`/audios/delete?name=${encodeURIComponent(name)}`),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['audios'] }),
    });
}

// Blacklist
export function useBlacklist(proyectoId: number) {
    return useQuery({
        queryKey: ['blacklist', proyectoId],
        queryFn: () => api.get<{ entries: BlacklistEntry[] | null; total: number }>(`/blacklist?proyecto_id=${proyectoId}&limit=50`),
        enabled: proyectoId > 0,
    });
}

export function useAddToBlacklist() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: { proyecto_id: number; telefono: string; razon?: string }) =>
            api.post('/blacklist', data),
        onSuccess: (_, variables) => queryClient.invalidateQueries({ queryKey: ['blacklist', variables.proyecto_id] }),
    });
}

export function useUploadBlacklist() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async ({ proyectoId, file }: { proyectoId: number; file: File }) => {
            const formData = new FormData();
            formData.append('file', file);
            formData.append('proyecto_id', String(proyectoId));
            return api.upload('/blacklist/upload', formData);
        },
        onSuccess: (_, variables) => queryClient.invalidateQueries({ queryKey: ['blacklist', variables.proyectoId] }),
    });
}

export function useDeleteFromBlacklist() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: ({ id }: { id: number; proyectoId: number }) =>
            api.delete(`/blacklist/delete?id=${id}`),
        onSuccess: (_, variables) => queryClient.invalidateQueries({ queryKey: ['blacklist', variables.proyectoId] }),
    });
}

export function useClearBlacklist() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (proyectoId: number) => api.delete(`/blacklist/clear?proyecto_id=${proyectoId}`),
        onSuccess: (_, proyectoId) => queryClient.invalidateQueries({ queryKey: ['blacklist', proyectoId] }),
    });
}

// Campaigns
export function useCampaigns(proyectoId?: number) {
    const params = proyectoId ? `?proyecto_id=${proyectoId}` : '';
    return useQuery({
        queryKey: ['campaigns', proyectoId],
        queryFn: () => api.get<import('@/types').Campaign[]>(`/campaigns${params}`),
        refetchInterval: 3000, // Real-time updates every 3 seconds
    });
}

export function useCreateCampaign() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: { nombre: string; proyecto_id: number }) =>
            api.post<import('@/types').Campaign>('/campaigns', data),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['campaigns'] }),
    });
}

export function useUpdateCampaign() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: Partial<import('@/types').Campaign>) =>
            api.put<import('@/types').Campaign>('/campaigns', data),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['campaigns'] }),
    });
}

export function useDeleteCampaign() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: number) => api.delete(`/campaigns/delete?id=${id}`),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['campaigns'] }),
    });
}

export function useUploadCampaignCSV() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async ({ campaignId, file }: { campaignId: number; file: File }) => {
            const formData = new FormData();
            formData.append('file', file);
            return api.upload(`/campaigns/upload?campaign_id=${campaignId}`, formData);
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['campaigns'] }),
    });
}

export function useCampaignAction() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: { campaign_id: number; action: 'start' | 'pause' | 'stop' }) =>
            api.post('/campaigns/action', data),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['campaigns'] }),
    });
}

export function useCampaignStats(campaignId: number) {
    return useQuery({
        queryKey: ['campaign-stats', campaignId],
        queryFn: () => api.get<import('@/types').CampaignStats>(`/campaigns/stats?campaign_id=${campaignId}`),
        enabled: campaignId > 0,
        refetchInterval: 5000, // Refresh every 5 seconds
    });
}

export function useCampaignSchedules(campaignId: number) {
    return useQuery({
        queryKey: ['campaign-schedules', campaignId],
        queryFn: () => api.get<import('@/types').CampaignSchedule[]>(`/campaigns/schedules?campaign_id=${campaignId}`),
        enabled: campaignId > 0,
    });
}

export function useUpdateCampaignSchedules() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: ({ campaignId, schedules }: { campaignId: number; schedules: import('@/types').CampaignSchedule[] }) =>
            api.post(`/campaigns/schedules?campaign_id=${campaignId}`, schedules),
        onSuccess: (_, { campaignId }) => queryClient.invalidateQueries({ queryKey: ['campaign-schedules', campaignId] }),
    });
}

// Campaign Recycling
export function useCampaignDispositions(campaignId: number | null) {
    return useQuery({
        queryKey: ['campaign-dispositions', campaignId],
        queryFn: () => api.get<import('@/types').DispositionCount[]>(`/campaigns/dispositions?campaign_id=${campaignId}`),
        enabled: campaignId !== null && campaignId > 0,
    });
}

export function useRecycleCampaign() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: { campaign_id: number; nombre: string; dispositions: string[] }) =>
            api.post<import('@/types').RecycleResult>('/campaigns/recycle', data),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['campaigns'] }),
    });
}
