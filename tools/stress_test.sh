#!/bin/bash
# stress_test.sh - Simula tráfico concurrente
# Uso: ./stress_test.sh [numero_llamadas]

COUNT=${1:-10}
URL="http://localhost:8080/api/v1/call"
PROJECT_ID=937
PHONE_BASE=3000000

echo "=== Iniciando Stress Test: $COUNT llamadas simultáneas ==="
start_time=$(date +%s%N)

for i in $(seq 1 $COUNT); do
    PHONE="${PHONE_BASE}${i}"
    (
        curl -s -X POST "$URL" \
            -H "Content-Type: application/json" \
            -d "{\"proyecto_id\": $PROJECT_ID, \"telefono\": \"$PHONE\"}" > /dev/null
        # echo "Request $i sent"
    ) &
done

wait

end_time=$(date +%s%N)
duration=$(( (end_time - start_time) / 1000000 ))

echo "=== Fin del Test ==="
echo "Total enviados: $COUNT"
echo "Tiempo total: ${duration}ms"
echo "Rate estimado: $(( COUNT * 1000 / duration )) req/sec"
