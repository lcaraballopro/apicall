# Scripts de Apicall

## stress_test.sh - Prueba de Estrés

Script para probar la capacidad del sistema generando llamadas masivas.

### Uso Básico

```bash
# Test rápido - 100 llamadas Colombia
./scripts/stress_test.sh

# Test con 500 llamadas México
./scripts/stress_test.sh -n 500 -c mexico

# Test extremo 1000 llamadas con limpieza previa
./scripts/stress_test.sh -n 1000 --clean

# Ver todas las opciones
./scripts/stress_test.sh --help
```

### Opciones

| Opción | Descripción | Default |
|--------|-------------|---------|
| `-n, --calls` | Número de llamadas | 100 |
| `-p, --proyecto` | ID del proyecto | 937 |
| `-c, --country` | País (mexico/colombia) | colombia |
| `-u, --url` | URL de la API | http://localhost:8080/api/v1/call |
| `-w, --wait` | Segundos de espera post-test | 5 |
| `--clean` | Limpiar cola antes del test | false |

### Ejemplos de Output

```
╔═══════════════════════════════════════════╗
║   APICALL - PRUEBA DE ESTRÉS             ║
╚═══════════════════════════════════════════╝

═══ CONFIGURACIÓN DEL TEST ═══
  Llamadas:    400
  Proyecto:    937
  País:        colombia
  API URL:     http://localhost:8080/api/v1/call

🚀 INICIANDO TEST: 17:10:11.800117258
  → 50/400 llamadas enviadas...
  → 100/400 llamadas enviadas...
  → 150/400 llamadas enviadas...
✅ ENCOLADO COMPLETO: 17:10:13.751419008
   Tiempo total: 1.95 segundos

═══ MÉTRICAS POST-ENCOLADO ═══
Total Encoladas: 400
Caller IDs Únicos: 400
Números Únicos: 400
Archivos en spool: 377

═══ PROGRESO DE PROCESAMIENTO ═══
  Antes:       377 archivos
  Ahora:       0 archivos
  Procesados:  377 en 5s
  Rate real:   75.40 CPS
```

### Notas

- Requiere `bc` para cálculos decimales: `apt-get install bc`
- Requiere acceso a MySQL/MariaDB
- Genera números aleatorios válidos por país
- Muestra progreso en tiempo real
- Calcula throughput y CPS real
