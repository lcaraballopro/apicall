const API_BASE = '/api/v1';

interface FetchOptions extends RequestInit {
    body?: string | FormData;
}

class ApiClient {
    private getToken(): string | null {
        return localStorage.getItem('apicall_token');
    }

    async fetch<T>(endpoint: string, options: FetchOptions = {}): Promise<T> {
        const token = this.getToken();

        const headers: HeadersInit = {
            ...options.headers,
        };

        if (token) {
            (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
        }

        if (options.body && typeof options.body === 'string') {
            (headers as Record<string, string>)['Content-Type'] = 'application/json';
        }

        const res = await fetch(`${API_BASE}${endpoint}`, {
            ...options,
            headers,
        });

        if (res.status === 401) {
            localStorage.removeItem('apicall_token');
            localStorage.removeItem('apicall_user');
            window.location.href = '/login';
            throw new Error('Unauthorized');
        }

        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || `HTTP ${res.status}`);
        }

        const contentType = res.headers.get('content-type');
        if (contentType?.includes('application/json')) {
            return res.json();
        }

        return res.text() as unknown as T;
    }

    get<T>(endpoint: string): Promise<T> {
        return this.fetch<T>(endpoint, { method: 'GET' });
    }

    post<T>(endpoint: string, data?: unknown): Promise<T> {
        return this.fetch<T>(endpoint, {
            method: 'POST',
            body: data ? JSON.stringify(data) : undefined,
        });
    }

    put<T>(endpoint: string, data: unknown): Promise<T> {
        return this.fetch<T>(endpoint, {
            method: 'PUT',
            body: JSON.stringify(data),
        });
    }

    delete<T>(endpoint: string): Promise<T> {
        return this.fetch<T>(endpoint, { method: 'DELETE' });
    }

    async upload<T>(endpoint: string, formData: FormData): Promise<T> {
        const token = this.getToken();
        const headers: HeadersInit = {};

        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }

        const res = await fetch(`${API_BASE}${endpoint}`, {
            method: 'POST',
            headers,
            body: formData,
        });

        if (!res.ok) {
            throw new Error(`Upload failed: ${res.status}`);
        }

        return res.json();
    }
}

export const api = new ApiClient();
