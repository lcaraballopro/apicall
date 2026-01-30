import { useEffect, useRef, useState, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';

export type WebSocketEventType =
    | 'call_start'
    | 'call_update'
    | 'call_end'
    | 'stats_update'
    | 'project_stats';

export interface WebSocketMessage {
    type: WebSocketEventType;
    data: unknown;
    timestamp: string;
}

interface UseWebSocketOptions {
    onMessage?: (message: WebSocketMessage) => void;
    autoInvalidateQueries?: boolean;
    reconnectInterval?: number;
}

export function useWebSocket(options: UseWebSocketOptions = {}) {
    const {
        onMessage,
        autoInvalidateQueries = true,
        reconnectInterval = 3000
    } = options;

    const wsRef = useRef<WebSocket | null>(null);
    const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const queryClient = useQueryClient();

    const [isConnected, setIsConnected] = useState(false);
    const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null);
    const [connectionAttempts, setConnectionAttempts] = useState(0);

    const connect = useCallback(() => {
        // Clean up existing connection
        if (wsRef.current) {
            wsRef.current.close();
        }

        // Determine WebSocket URL based on current location
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        try {
            const ws = new WebSocket(wsUrl);
            wsRef.current = ws;

            ws.onopen = () => {
                console.log('[WebSocket] Connected');
                setIsConnected(true);
                setConnectionAttempts(0);
            };

            ws.onclose = (event) => {
                console.log('[WebSocket] Disconnected', event.code, event.reason);
                setIsConnected(false);
                wsRef.current = null;

                // Auto reconnect
                if (reconnectInterval > 0) {
                    reconnectTimeoutRef.current = setTimeout(() => {
                        setConnectionAttempts(prev => prev + 1);
                        connect();
                    }, reconnectInterval);
                }
            };

            ws.onerror = (error) => {
                console.error('[WebSocket] Error:', error);
            };

            ws.onmessage = (event) => {
                try {
                    // Handle multiple messages (newline separated)
                    const messages = event.data.split('\n').filter(Boolean);

                    messages.forEach((msgStr: string) => {
                        const message: WebSocketMessage = JSON.parse(msgStr);
                        setLastMessage(message);

                        // Call custom handler
                        if (onMessage) {
                            onMessage(message);
                        }

                        // Auto-invalidate relevant queries based on event type
                        if (autoInvalidateQueries) {
                            switch (message.type) {
                                case 'call_start':
                                case 'call_update':
                                case 'call_end':
                                    queryClient.invalidateQueries({ queryKey: ['logs-realtime'] });
                                    queryClient.invalidateQueries({ queryKey: ['logs'] });
                                    break;
                                case 'stats_update':
                                case 'project_stats':
                                    queryClient.invalidateQueries({ queryKey: ['logs-realtime'] });
                                    queryClient.invalidateQueries({ queryKey: ['proyectos'] });
                                    break;
                            }
                        }
                    });
                } catch (error) {
                    console.error('[WebSocket] Parse error:', error);
                }
            };
        } catch (error) {
            console.error('[WebSocket] Connection error:', error);
        }
    }, [onMessage, autoInvalidateQueries, queryClient, reconnectInterval]);

    // Connect on mount
    useEffect(() => {
        connect();

        return () => {
            if (reconnectTimeoutRef.current) {
                clearTimeout(reconnectTimeoutRef.current);
            }
            if (wsRef.current) {
                wsRef.current.close();
            }
        };
    }, [connect]);

    // Subscribe to a topic
    const subscribe = useCallback((topic: string) => {
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
            wsRef.current.send(JSON.stringify({ action: 'subscribe', topic }));
        }
    }, []);

    // Unsubscribe from a topic
    const unsubscribe = useCallback((topic: string) => {
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
            wsRef.current.send(JSON.stringify({ action: 'unsubscribe', topic }));
        }
    }, []);

    return {
        isConnected,
        lastMessage,
        connectionAttempts,
        subscribe,
        unsubscribe,
        reconnect: connect,
    };
}

// Hook specifically for dashboard real-time updates
export function useDashboardRealtime() {
    const [callEvents, setCallEvents] = useState<WebSocketMessage[]>([]);

    const handleMessage = useCallback((message: WebSocketMessage) => {
        // Keep last 50 events for display
        setCallEvents(prev => [message, ...prev].slice(0, 50));
    }, []);

    const ws = useWebSocket({
        onMessage: handleMessage,
        autoInvalidateQueries: true,
    });

    return {
        ...ws,
        callEvents,
        clearEvents: () => setCallEvents([]),
    };
}
