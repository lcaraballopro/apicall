const API_BASE = '/api/v1';

// --- Auth Middleware ---
function getToken() {
    return localStorage.getItem('apicall_token');
}

function checkAuth() {
    const token = getToken();
    if (!token) {
        window.location.href = 'login.html';
        return false;
    }
    // Update User Profile UI
    const userStr = localStorage.getItem('apicall_user');
    if (userStr) {
        const user = JSON.parse(userStr);
        document.getElementById('user-name-display').innerText = user.fullName || user.username;
        document.getElementById('user-role-badge').innerText = user.role.toUpperCase();

        // Show/Hide Admin menus
        if (user.role !== 'admin') {
            document.querySelectorAll('.admin-only').forEach(el => el.style.display = 'none');
        }
    }
    return true;
}

function logout() {
    localStorage.removeItem('apicall_token');
    localStorage.removeItem('apicall_user');
    window.location.href = 'login.html';
}

// Wrapper for fetch to add Auth Header
async function apiFetch(endpoint, options = {}) {
    if (!options.headers) options.headers = {};
    options.headers['Authorization'] = `Bearer ${getToken()}`;

    const res = await fetch(`${API_BASE}${endpoint}`, options);

    if (res.status === 401) {
        logout(); // Token expired or invalid
        return null;
    }
    return res;
}

// --- Navigation ---
function showView(viewId) {
    document.querySelectorAll('.view').forEach(el => el.classList.remove('active'));
    document.getElementById(`view-${viewId}`).classList.add('active');

    document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
    // Simple matching
    const navs = document.querySelectorAll('.nav-item');
    navs.forEach(n => {
        if (n.getAttribute('onclick').includes(viewId)) n.classList.add('active');
    });

    const titles = {
        'dashboard': 'Dashboard',
        'proyectos': 'Gestión de Proyectos',
        'troncales': 'Gestión de Troncales',
        'testcall': 'Prueba de Llamadas',
        'users': 'Gestión de Usuarios',
        'reports': 'Reportes de Llamadas',
        'audios': 'Gestión de Audios'
    };
    document.getElementById('page-title').innerText = titles[viewId];

    if (viewId === 'dashboard') loadDashboard();
    if (viewId === 'proyectos') loadProyectos();
    if (viewId === 'troncales') loadTroncales();
    if (viewId === 'testcall') loadProyectosForSelect();
    if (viewId === 'users') loadUsers();
    if (viewId === 'reports') loadReportsInit();
    if (viewId === 'audios') loadAudios();
}

// --- Data Loading ---
async function loadDashboard() {
    const r1 = await apiFetch('/proyectos');
    const p = r1 ? await r1.json() : [];

    const r2 = await apiFetch('/troncales');
    const t = r2 ? await r2.json() : [];

    document.getElementById('dash-projects-count').innerText = p.length || 0;
    document.getElementById('dash-trunks-count').innerText = t.length || 0;
}

async function loadProyectos() {
    const res = await apiFetch('/proyectos');
    if (!res) return;
    const data = await res.json();
    const tbody = document.getElementById('table-proyectos-body');
    tbody.innerHTML = '';

    if (!data || data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" style="text-align:center">No hay proyectos</td></tr>';
        return;
    }

    data.forEach(p => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${p.id}</td>
            <td><strong>${p.nombre}</strong></td>
            <td>${p.caller_id}</td>
            <td><span class="badge">${p.troncal_salida}</span></td>
            <td>${p.amd_active ? '✅' : '❌'}</td>
            <td>${p.smart_cid_active ? '✅' : '❌'}</td>
            <td>
                <button class="btn btn-primary" style="padding: 5px 12px; margin-right: 5px;" onclick='openEditProyecto(${JSON.stringify(p)})'>Editar</button>
                <button class="btn btn-danger" onclick="deleteProyecto(${p.id})">Eliminar</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

async function loadTroncales() {
    const res = await apiFetch('/troncales');
    if (!res) return;
    const data = await res.json();
    const tbody = document.getElementById('table-troncales-body');
    tbody.innerHTML = '';

    if (!data || data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align:center">No hay troncales</td></tr>';
        return;
    }

    data.forEach(t => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${t.id}</td>
            <td><strong>${t.nombre}</strong></td>
            <td>${t.host}:${t.puerto}</td>
            <td>${t.usuario || '-'}</td>
            <td>${t.contexto}</td>
            <td>
                <button class="btn btn-danger" onclick="deleteTroncal(${t.id})">Eliminar</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

async function loadProyectosForSelect() {
    const res = await apiFetch('/proyectos');
    if (!res) return;
    const data = await res.json();
    const select = document.getElementById('call-proyecto-id');
    select.innerHTML = '<option value="">Seleccione...</option>';
    if (data) {
        data.forEach(p => {
            const opt = document.createElement('option');
            opt.value = p.id;
            opt.innerText = `${p.nombre} (#${p.id})`;
            select.appendChild(opt);
        });
    }
}

// --- Users Management ---
async function loadUsers() {
    const res = await apiFetch('/users');
    if (!res) return;
    const data = await res.json();
    const tbody = document.getElementById('table-users-body');
    tbody.innerHTML = '';

    data.forEach(u => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${u.username}</td>
            <td>${u.full_name}</td>
            <td><span class="badge">${u.role}</span></td>
            <td>${u.active ? '🟢' : '🔴'}</td>
            <td>
                <button class="btn btn-danger" onclick="deleteUser(${u.id})">Borrar</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

async function handleCreateUser(e) {
    e.preventDefault();
    const formData = new FormData(e.target);
    const data = Object.fromEntries(formData.entries());

    const res = await apiFetch('/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });

    if (res && res.ok) {
        closeModal('modal-user');
        e.target.reset();
        loadUsers();
    } else {
        alert('Error creando usuario');
    }
}

async function deleteUser(id) {
    if (!confirm('Eliminar usuario?')) return;
    await apiFetch(`/users/delete?id=${id}`);
    loadUsers();
}

// --- Actions ---

async function handleCreateProyecto(e) {
    e.preventDefault();
    const formData = new FormData(e.target);
    const data = {
        id: parseInt(formData.get('id')),
        nombre: formData.get('nombre'),
        caller_id: formData.get('caller_id'),
        troncal_salida: formData.get('troncal_salida'),
        prefijo_salida: formData.get('prefijo_salida'),
        audio: formData.get('audio'),
        num_desborde: formData.get('num_desborde'),
        amd_active: formData.get('amd_active') === 'on',
        smart_active: formData.get('smart_active') === 'on'
    };

    const res = await apiFetch('/proyectos', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });

    if (res && res.ok) {
        closeModal('modal-proyecto');
        e.target.reset();
        loadProyectos();
    } else {
        alert('Error creando proyecto');
    }
}

async function deleteProyecto(id) {
    if (!confirm('¿Seguro?')) return;
    await apiFetch(`/proyectos/delete?id=${id}`, { method: 'DELETE' });
    loadProyectos();
}

function openEditProyecto(proyecto) {
    // Populate form with project data
    document.getElementById('edit-proyecto-id').value = proyecto.id;
    document.getElementById('edit-proyecto-nombre').value = proyecto.nombre;
    document.getElementById('edit-proyecto-callerid').value = proyecto.caller_id || '';
    document.getElementById('edit-proyecto-audio').value = proyecto.audio || '';
    document.getElementById('edit-proyecto-dtmf').value = proyecto.dtmf_esperado || '';
    document.getElementById('edit-proyecto-desborde').value = proyecto.numero_desborde || '';
    document.getElementById('edit-proyecto-troncal').value = proyecto.troncal_salida || '';
    document.getElementById('edit-proyecto-prefijo').value = proyecto.prefijo_salida || '';
    document.getElementById('edit-proyecto-ips').value = proyecto.ips_autorizadas || '*';
    document.getElementById('edit-proyecto-retries').value = proyecto.max_retries || 0;
    document.getElementById('edit-proyecto-retry-time').value = proyecto.retry_time || 60;
    document.getElementById('edit-proyecto-amd').checked = proyecto.amd_active || false;
    document.getElementById('edit-proyecto-smart').checked = proyecto.smart_cid_active || false;

    // Load blacklist for this project
    loadBlacklist(proyecto.id);

    openModal('modal-edit-proyecto');
}

async function handleEditProyecto(e) {
    e.preventDefault();
    const formData = new FormData(e.target);
    const data = {
        id: parseInt(formData.get('id')),
        nombre: formData.get('nombre'),
        caller_id: formData.get('caller_id'),
        audio: formData.get('audio'),
        dtmf_esperado: formData.get('dtmf_esperado'),
        numero_desborde: formData.get('numero_desborde'),
        troncal_salida: formData.get('troncal_salida'),
        prefijo_salida: formData.get('prefijo_salida'),
        ips_autorizadas: formData.get('ips_autorizadas'),
        max_retries: parseInt(formData.get('max_retries')),
        retry_time: parseInt(formData.get('retry_time')),
        amd_active: formData.get('amd_active') === 'on',
        smart_cid_active: formData.get('smart_cid_active') === 'on'
    };

    const res = await apiFetch('/proyectos', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });

    if (res && res.ok) {
        closeModal('modal-edit-proyecto');
        loadProyectos();
        alert('✅ Proyecto actualizado correctamente');
    } else {
        alert('❌ Error actualizando proyecto');
    }
}


async function handleCreateTroncal(e) {
    e.preventDefault();
    const formData = new FormData(e.target);
    const data = {
        nombre: formData.get('nombre'),
        host: formData.get('host'),
        puerto: parseInt(formData.get('puerto')),
        usuario: formData.get('usuario'),
        password: formData.get('password'),
        contexto: 'apicall_context',
        activo: true
    };

    const res = await apiFetch('/troncales', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
    });

    if (res && res.ok) {
        closeModal('modal-troncal');
        e.target.reset();
        loadTroncales();
    } else {
        alert('Error creando troncal');
    }
}

async function deleteTroncal(id) {
    if (!confirm('¿Seguro?')) return;
    await apiFetch(`/troncales/delete?id=${id}`, { method: 'DELETE' });
    loadTroncales();
}

async function handleTestCall(e) {
    e.preventDefault();
    const pid = document.getElementById('call-proyecto-id').value;
    const num = document.getElementById('call-number').value;

    const res = await apiFetch('/call', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ proyecto_id: parseInt(pid), telefono: num })
    });

    const resultDiv = document.getElementById('call-result');
    resultDiv.style.display = 'block';

    if (res && res.ok) {
        resultDiv.innerText = "✅ Llamada encolada";
        resultDiv.style.color = "var(--success)";
    } else {
        resultDiv.innerText = "❌ Error";
        resultDiv.style.color = "var(--danger)";
    }
}

// --- Reports Management ---
async function loadReportsInit() {
    // Populate project filter
    const res = await apiFetch('/proyectos');
    if (!res) return;
    const data = await res.json();
    const select = document.getElementById('report-proyecto-filter');
    select.innerHTML = '<option value="">Todos los Proyectos</option>';
    if (data) {
        data.forEach(p => {
            const opt = document.createElement('option');
            opt.value = p.id;
            opt.innerText = `${p.nombre} (#${p.id})`;
            select.appendChild(opt);
        });
    }

    // Set default dates (last 7 days)
    const today = new Date();
    const weekAgo = new Date(today.getTime() - 7 * 24 * 60 * 60 * 1000);
    document.getElementById('report-date-to').valueAsDate = today;
    document.getElementById('report-date-from').valueAsDate = weekAgo;
}

async function loadReports() {
    const proyectoID = document.getElementById('report-proyecto-filter').value;
    const fromDate = document.getElementById('report-date-from').value;
    const toDate = document.getElementById('report-date-to').value;

    // Build URL with optional filters
    let url = '/logs?limit=5000';
    if (proyectoID) {
        url += `&proyecto_id=${proyectoID}`;
    }
    if (fromDate) {
        url += `&from_date=${fromDate}`;
    }
    if (toDate) {
        url += `&to_date=${toDate}`;
    }

    try {
        const res = await apiFetch(url);
        if (!res) return;

        const data = await res.json();
        const tbody = document.getElementById('table-reports-body');
        const statsDiv = document.getElementById('report-stats');
        tbody.innerHTML = '';

        if (!data || data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="8" style="text-align:center">Sin registros para los filtros seleccionados</td></tr>';
            statsDiv.innerHTML = '0 registros encontrados';
            return;
        }

        // Calculate statistics
        const total = data.length;
        const answered = data.filter(log => log.disposition === 'ANSWERED').length;
        const interacted = data.filter(log => log.interacciono).length;
        const avgDuration = Math.round(data.reduce((sum, log) => sum + (log.duracion || 0), 0) / total);

        statsDiv.innerHTML = `${total} registros | ${answered} respondidas | ${interacted} con interacción | Duración promedio: ${avgDuration}s`;

        // Get project names for display
        const proyectosRes = await apiFetch('/proyectos');
        const proyectos = proyectosRes ? await proyectosRes.json() : [];
        const proyectoMap = {};
        proyectos.forEach(p => proyectoMap[p.id] = p.nombre);

        data.forEach(log => {
            const tr = document.createElement('tr');
            const projectName = proyectoMap[log.proyecto_id] || `Proyecto #${log.proyecto_id}`;
            const statusBadge = getStatusBadge(log.status);
            const dispositionBadge = getDispositionBadge(log.disposition);

            tr.innerHTML = `
                <td>${log.id}</td>
                <td><strong>${projectName}</strong></td>
                <td>${log.telefono}</td>
                <td>${statusBadge}</td>
                <td>${dispositionBadge}</td>
                <td>${log.interacciono ? '✅ Sí' : '❌ No'}</td>
                <td>${log.duracion || 0}s</td>
                <td>${new Date(log.created_at).toLocaleString()}</td>
            `;
            tbody.appendChild(tr);
        });
    } catch (error) {
        console.error('Error loading reports:', error);
        const tbody = document.getElementById('table-reports-body');
        tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;color:red">Error cargando datos</td></tr>';
    }
}

// Helper functions for status badges
function getStatusBadge(status) {
    const statusConfig = {
        'PENDING': { color: 'orange', text: 'Pendiente' },
        'CONNECTED': { color: 'blue', text: 'Conectado' },
        'ANSWERED': { color: 'green', text: 'Respondido' },
        'FAILED': { color: 'red', text: 'Falló' },
        'BUSY': { color: 'red', text: 'Ocupado' },
        'NOANSWER': { color: 'orange', text: 'No Respondió' }
    };

    const config = statusConfig[status] || { color: 'gray', text: status };
    return `<span class="badge" style="background-color: var(--${config.color}-color)">${config.text}</span>`;
}

function getDispositionBadge(disposition) {
    if (!disposition) return '-';

    const dispConfig = {
        'ANSWERED': { color: 'green', text: 'Respondida' },
        'BUSY': { color: 'red', text: 'Ocupado' },
        'NO ANSWER': { color: 'orange', text: 'No Respondió' },
        'CANCELLED': { color: 'gray', text: 'Cancelada' },
        'FAILED': { color: 'red', text: 'Falló' }
    };

    const config = dispConfig[disposition] || { color: 'gray', text: disposition };
    return `<span class="badge" style="background-color: var(--${config.color}-color)">${config.text}</span>`;
}

// --- Audio Management ---
async function loadAudios() {
    const res = await apiFetch('/audios');
    if (!res) return;
    const data = await res.json();
    const tbody = document.getElementById('table-audios-body');
    tbody.innerHTML = '';

    if (!data || data.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align:center">No hay audios</td></tr>';
        return;
    }

    data.forEach(audio => {
        const tr = document.createElement('tr');
        const sizeKB = (audio.size / 1024).toFixed(2);
        tr.innerHTML = `
            <td><strong>${audio.name}</strong></td>
            <td>${sizeKB} KB</td>
            <td>${new Date(audio.date).toLocaleDateString()}</td>
            <td>
                <button class="btn btn-danger" onclick="deleteAudio('${audio.name}')">Eliminar</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

async function handleAudioUpload(event) {
    const file = event.target.files[0];
    if (!file) return;

    const formData = new FormData();
    formData.append('audio', file);

    try {
        const token = getToken();
        const uploadRes = await fetch('/api/v1/audios/upload', {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${token}` },
            body: formData
        });

        if (uploadRes && uploadRes.ok) {
            const result = await uploadRes.json();
            alert(`✅ Audio subido: ${result.filename}`);
            loadAudios();
            event.target.value = ''; // Reset input
        } else {
            alert('❌ Error subiendo audio');
        }
    } catch (error) {
        console.error(error);
        alert('❌ Error de conexión');
    }
}

async function deleteAudio(name) {
    if (!confirm(`¿Eliminar ${name}?`)) return;
    await apiFetch(`/audios/delete?name=${encodeURIComponent(name)}`, { method: 'DELETE' });
    loadAudios();
}

// --- Blacklist Management ---
async function loadBlacklist(proyectoID) {
    const res = await apiFetch(`/blacklist?proyecto_id=${proyectoID}&limit=50`);
    if (!res) return;

    const data = await res.json();
    const tbody = document.getElementById('table-blacklist-body');
    document.getElementById('blacklist-count').innerText = data.total || 0;

    tbody.innerHTML = '';

    if (!data.entries || data.entries.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="text-align:center">No hay números bloqueados</td></tr>';
        return;
    }

    data.entries.forEach(entry => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td>${entry.telefono}</td>
            <td>${entry.razon || '-'}</td>
            <td>${new Date(entry.created_at).toLocaleDateString()}</td>
            <td>
                <button class="btn btn-danger" style="padding:2px 8px;font-size:0.75rem" 
                    onclick="deleteFromBlacklist(${entry.id})">🗑</button>
            </td>
        `;
        tbody.appendChild(tr);
    });
}

async function handleBlacklistCSVUpload(event) {
    const file = event.target.files[0];
    if (!file) return;

    const proyectoID = document.getElementById('edit-proyecto-id').value;
    const formData = new FormData();
    formData.append('file', file);
    formData.append('proyecto_id', proyectoID);

    try {
        const token = getToken();
        const res = await fetch('/api/v1/blacklist/upload', {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${token}` },
            body: formData
        });

        if (res && res.ok) {
            const result = await res.json();
            alert(`✅ Importados ${result.imported} de ${result.total} números`);
            loadBlacklist(proyectoID);
            event.target.value = ''; // Reset input
        } else {
            alert('❌ Error importando CSV');
        }
    } catch (error) {
        console.error(error);
        alert('❌ Error de conexión');
    }
}

async function addToBlacklist() {
    const proyectoID = document.getElementById('edit-proyecto-id').value;
    const telefono = document.getElementById('blacklist-telefono').value.trim();

    if (!telefono) {
        alert('Ingrese un número de teléfono');
        return;
    }

    const res = await apiFetch('/blacklist', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ proyecto_id: parseInt(proyectoID), telefono: telefono })
    });

    if (res && res.ok) {
        document.getElementById('blacklist-telefono').value = '';
        loadBlacklist(proyectoID);
    } else {
        alert('❌ Error agregando número');
    }
}

async function deleteFromBlacklist(id) {
    const proyectoID = document.getElementById('edit-proyecto-id').value;
    await apiFetch(`/blacklist/delete?id=${id}`, { method: 'DELETE' });
    loadBlacklist(proyectoID);
}

async function clearBlacklist() {
    if (!confirm('¿Eliminar TODOS los números de la lista negra?')) return;

    const proyectoID = document.getElementById('edit-proyecto-id').value;
    const res = await apiFetch(`/blacklist/clear?proyecto_id=${proyectoID}`, { method: 'DELETE' });

    if (res && res.ok) {
        alert('✅ Lista negra limpiada');
        loadBlacklist(proyectoID);
    } else {
        alert('❌ Error limpiando lista');
    }
}

// --- Modals ---
function openModal(id) { document.getElementById(id).classList.add('active'); }
function closeModal(id) { document.getElementById(id).classList.remove('active'); }

// --- Init ---
document.addEventListener('DOMContentLoaded', () => {
    if (checkAuth()) {
        showView('dashboard');
    }
});
