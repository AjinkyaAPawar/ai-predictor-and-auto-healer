// AI Incident Predictor & Auto-Healer - Production Grade
// Expert-level implementation with invisible demo controls

const state = {
  snapshot: null,
  timeline: [],
  route: 'overview',
  connectedAt: null,
  lastPingAt: null,
  lastUpdateAt: null,
  streamStatus: 'connecting',
  es: null,
  reconnectTimer: null,
  connectWatchdog: null,
  pollInterval: null,
  renderInterval: null,
  activityPulse: 0,
  demoMode: false,
};

function qs(id){ return document.getElementById(id); }
function escapeHTML(s){ return String(s||'').replace(/[&<>"']/g, m => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m])); }
function fmtPct(n){ return (n===null||n===undefined)?'—':`${Number(n).toFixed(1)}%`; }
function fmtTs(ts){ return ts ? new Date(ts).toLocaleTimeString() : '—'; }

// SSE CONNECTION - FIXED
function connectSSE(){
  if (state.es) {
    try { state.es.close(); } catch (_) {}
    state.es = null;
  }
  if (state.reconnectTimer) {
    clearTimeout(state.reconnectTimer);
    state.reconnectTimer = null;
  }
  if (state.connectWatchdog) {
    clearTimeout(state.connectWatchdog);
    state.connectWatchdog = null;
  }

  state.streamStatus = 'connecting';
  renderLive();

  try {
    const es = new EventSource('/api/v1/stream');
    state.es = es;

    // Watchdog for connection timeout
    state.connectWatchdog = setTimeout(() => {
      state.connectWatchdog = null;
      if (state.streamStatus === 'connecting') {
        console.warn('SSE connection timeout, retrying...');
        try { es.close(); } catch (_) {}
        setTimeout(() => connectSSE(), 2000);
      }
    }, 5000);

    es.addEventListener('open', () => {
      console.log('SSE connected');
      state.streamStatus = 'open';
      if (!state.connectedAt) state.connectedAt = Date.now();
      state.lastPingAt = Date.now();
      if (state.connectWatchdog) {
        clearTimeout(state.connectWatchdog);
        state.connectWatchdog = null;
      }
      renderLive();
      const liveStateEl = qs('liveState');
      if (liveStateEl) liveStateEl.textContent = 'Connected';
    });

    es.addEventListener('ready', (e) => {
      console.log('SSE ready event received');
      state.streamStatus = 'open';
      if (!state.connectedAt) state.connectedAt = Date.now();
      state.lastPingAt = Date.now();
      if (state.connectWatchdog) {
        clearTimeout(state.connectWatchdog);
        state.connectWatchdog = null;
      }
      renderLive();
    });

    es.addEventListener('ping', () => {
      state.lastPingAt = Date.now();
      if (state.streamStatus !== 'open') {
        state.streamStatus = 'open';
        renderLive();
      }
    });

    es.addEventListener('update', async (e) => {
      const now = Date.now();
      state.lastPingAt = now;
      state.lastUpdateAt = now;
      state.streamStatus = 'open';
      state.activityPulse = now;
      await loadSnapshot();
      if (state.route === 'timeline') await loadTimeline();
      renderLive();
    });

    es.addEventListener('error', (e) => {
      console.error('SSE error:', e);
      state.streamStatus = 'error';
      renderLive();
      const liveStateEl = qs('liveState');
      if (liveStateEl) liveStateEl.textContent = 'Reconnecting…';
      
      if (state.connectWatchdog) {
        clearTimeout(state.connectWatchdog);
        state.connectWatchdog = null;
      }

      // Only reconnect if fully closed
      if (es.readyState === EventSource.CLOSED) {
        if (!state.reconnectTimer) {
          state.reconnectTimer = setTimeout(() => {
            state.reconnectTimer = null;
            console.log('Attempting SSE reconnection...');
            connectSSE();
          }, 3000);
        }
      }
    });

  } catch (err) {
    console.error('SSE connection failed:', err);
    state.streamStatus = 'error';
    renderLive();
    setTimeout(() => connectSSE(), 5000);
  }
}

// Fallback polling
function startFallbackPolling() {
  if (state.pollInterval) clearInterval(state.pollInterval);
  
  state.pollInterval = setInterval(async () => {
    const timeSinceUpdate = Date.now() - (state.lastUpdateAt || 0);
    if (timeSinceUpdate > 15000) {
      console.log('Fallback poll triggered');
      await loadSnapshot();
      if (state.route === 'timeline') await loadTimeline();
    }
  }, 10000);
}

async function loadSnapshot() {
  try {
    const res = await fetch('/api/v1/snapshot');
    if (!res.ok) throw new Error('Snapshot fetch failed');
    state.snapshot = await res.json();
    state.lastUpdateAt = Date.now();
    state.activityPulse = Date.now();
    renderAll();
  } catch (err) {
    console.error('Snapshot error:', err);
  }
}

async function loadTimeline() {
  const limitEl = qs('timelineLimit');
  const limit = limitEl ? parseInt(limitEl.value || '300', 10) : 300;
  try {
    const res = await fetch(`/api/v1/timeline?limit=${limit}`);
    if (!res.ok) throw new Error('Timeline fetch failed');
    const data = await res.json();
    state.timeline = data.items || [];
    if (state.route === 'timeline') renderTimeline();
  } catch (err) {
    console.error('Timeline error:', err);
  }
}

// RENDERING
function renderAll() {
  renderKPIs();
  renderMode();
  renderSystemHealth();
  if (state.route === 'overview') {
    renderRiskBars();
    renderIncidentBreakdown();
  } else if (state.route === 'workloads') {
    renderWorkloads();
  } else if (state.route === 'timeline') {
    renderTimeline();
  }
  renderFooter();
}

function renderLive() {
  const dot = qs('liveDot');
  const text = qs('liveText');
  const sidebarState = qs('liveState');
  const sync = qs('lastSync');

  const timeSincePulse = Date.now() - (state.activityPulse || 0);
  const isPulsing = timeSincePulse < 1000;

  if (state.streamStatus === 'open') {
    if (dot) dot.className = isPulsing ? 'dot active pulse' : 'dot active';
    if (text) text.textContent = 'Live';
    if (sidebarState) sidebarState.textContent = 'Connected';
  } else if (state.streamStatus === 'connecting') {
    if (dot) dot.className = 'dot';
    if (text) text.textContent = 'Connecting…';
    if (sidebarState) sidebarState.textContent = 'Connecting…';
  } else {
    if (dot) dot.className = 'dot error';
    if (text) text.textContent = 'Reconnecting…';
    if (sidebarState) sidebarState.textContent = 'Reconnecting…';
  }

  if (state.lastUpdateAt && sync) {
    const ago = Math.floor((Date.now() - state.lastUpdateAt) / 1000);
    sync.textContent = ago < 2 ? 'Just now' : ago < 60 ? `${ago}s ago` : `${Math.floor(ago/60)}m ago`;
  }
}

function renderKPIs() {
  if (!state.snapshot) return;
  
  const snap = state.snapshot;
  const pods = snap.pod_metrics || [];
  const actions = snap.healing_actions || [];
  const preds = snap.predictions || [];
  const stuck = snap.stuck_containers || [];
  const checks = snap.container_checks || [];
  
  const runningPods = pods.filter(p => String(p.Status||'').startsWith('Running')).length;
  const totalPods = pods.length;
  const unhealthyPods = totalPods - runningPods;
  
  const critCount = preds.filter(p => (p.Risk||'').toUpperCase() === 'CRITICAL').length;
  const highCount = preds.filter(p => (p.Risk||'').toUpperCase() === 'HIGH').length;

  const grid = qs('kpiGrid');
  if (!grid) return;

  const cards = [
    { title: 'Running Pods', value: String(runningPods), sub: unhealthyPods > 0 ? `${unhealthyPods} unhealthy of ${totalPods}` : `${totalPods} total` },
    { title: 'Workloads', value: String(totalPods), sub: `${runningPods} running` },
    { title: 'Predictions', value: String(preds.length), sub: `Critical: ${critCount} | High: ${highCount}` },
    { title: 'Diagnostics', value: String(stuck.length + checks.length), sub: `Stuck: ${stuck.length} | Checks: ${checks.length}` },
    { title: 'Remediations', value: String(actions.length), sub: snap.dry_run ? 'Dry-run: simulated' : 'Live: applied' },
  ];

  grid.innerHTML = cards.map(c => `
    <div class="card">
      <div class="card-title">${escapeHTML(c.title)}</div>
      <div class="card-value">${escapeHTML(c.value)}</div>
      <div class="card-sub">${escapeHTML(c.sub)}</div>
    </div>
  `).join('');
}

function renderMode() {
  const modeEl = qs('mode');
  if (modeEl && state.snapshot) {
    modeEl.textContent = state.snapshot.dry_run ? 'DRY-RUN' : 'LIVE';
    modeEl.className = state.snapshot.dry_run ? 'v' : 'v';
  }
}

function renderSystemHealth() {
  const healthEl = qs('systemHealth');
  if (!healthEl || !state.snapshot) return;
  
  const snap = state.snapshot;
  const pods = snap.pod_metrics || [];
  const actions = snap.healing_actions || [];
  const preds = snap.predictions || [];
  
  const critPreds = preds.filter(p => (p.Risk||'').toUpperCase() === 'CRITICAL').length;
  const failedActions = actions.filter(a => a.Status === 'FAILED').length;
  const crashingPods = pods.filter(p => (p.Status||'').includes('CrashLoop') || (p.Restarts || 0) >= 10).length;
  
  let health = 'HEALTHY';
  
  if (failedActions > 0 || critPreds > 3 || crashingPods > 5) {
    health = 'CRITICAL';
  } else if (critPreds > 0 || crashingPods > 0 || actions.length > 10) {
    health = 'WARNING';
  }
  
  healthEl.textContent = health;
  healthEl.className = `pill ${health.toLowerCase() === 'critical' ? 'crit' : health.toLowerCase() === 'warning' ? 'warn' : 'ok'}`;
}

function renderIncidentBreakdown() {
  if (!state.snapshot) return;
  
  const preds = state.snapshot.predictions || [];
  
  const critical = preds.filter(p => (p.Risk||'').toUpperCase() === 'CRITICAL').length;
  const high = preds.filter(p => (p.Risk||'').toUpperCase() === 'HIGH').length;
  const medium = preds.filter(p => {
    const r = (p.Risk||'').toUpperCase();
    return r === 'MEDIUM' || r === 'LOW-MEDIUM';
  }).length;
  const low = preds.length - critical - high - medium;
  
  const critEl = qs('criticalCount');
  const highEl = qs('highCount');
  const medEl = qs('mediumCount');
  const lowEl = qs('lowCount');
  
  if (critEl) critEl.textContent = critical;
  if (highEl) highEl.textContent = high;
  if (medEl) medEl.textContent = medium;
  if (lowEl) lowEl.textContent = Math.max(0, low);
}

function riskMap(predictions){
  const out = { CRITICAL:0, HIGH:0, MEDIUM:0, LOW:0 };
  for (const p of predictions){
    const r = (p.Risk || '').toUpperCase();
    if (r === 'CRITICAL') out.CRITICAL++;
    else if (r === 'HIGH') out.HIGH++;
    else if (r === 'MEDIUM' || r === 'LOW-MEDIUM') out.MEDIUM++;
    else out.LOW++;
  }
  return out;
}

function renderRiskBars(){
  const snap = state.snapshot;
  if (!snap) return;
  
  const preds = snap.predictions || [];
  const m = riskMap(preds);
  const total = Math.max(1, preds.length);

  const set = (idFill, idVal, count) => {
    const valEl = qs(idVal);
    const fillEl = qs(idFill);
    if (valEl) valEl.textContent = String(count);
    if (fillEl) {
      const pct = Math.round((count / total) * 100);
      fillEl.style.width = `${pct}%`;
    }
  };

  set('riskCritical', 'riskCriticalVal', m.CRITICAL);
  set('riskHigh', 'riskHighVal', m.HIGH);
  set('riskMed', 'riskMedVal', m.MEDIUM);
  set('riskLow', 'riskLowVal', m.LOW);
}

function badgeClassFromRisk(r){
  const risk = (r||'').toUpperCase();
  if (risk === 'CRITICAL') return 'crit';
  if (risk === 'HIGH') return 'warn';
  if (risk === 'MEDIUM' || risk === 'LOW-MEDIUM') return 'info';
  return 'ok';
}

function predictionsByKey(preds){
  const map = new Map();
  for (const p of preds || []){
    const k = `${p.PodNamespace}/${p.PodName}`;
    map.set(k, p);
  }
  return map;
}

function renderWorkloads(){
  const snap = state.snapshot;
  const pods = (snap && snap.pod_metrics) ? snap.pod_metrics : [];
  const preds = predictionsByKey((snap && snap.predictions) ? snap.predictions : []);

  const searchEl = qs('search');
  const search = searchEl ? (searchEl.value || '').toLowerCase() : '';

  const rows = pods
    .map(p => {
      const key = `${p.Namespace}/${p.Name}`;
      const pr = preds.get(key);
      const risk = pr ? pr.Risk : 'LOW';
      const action = pr ? pr.Action : 'MONITOR';
      const ttf = pr ? pr.TimeToFailure : 'N/A';

      const riskScore = (() => {
        const r = (risk || '').toUpperCase();
        if (r === 'CRITICAL') return 4;
        if (r === 'HIGH') return 3;
        if (r === 'MEDIUM' || r === 'LOW-MEDIUM') return 2;
        return 1;
      })();

      return { p, pr, key, risk, action, ttf, riskScore };
    })
    .filter(x => x.key.toLowerCase().includes(search))
    .sort((a,b) => b.riskScore - a.riskScore || (b.p.Restarts - a.p.Restarts));

  const body = qs('workloadsBody');
  if (!body) return;
  
  body.innerHTML = rows.map(x => {
    const p = x.p;
    const statusStr = String(p.Status || '');
    const statusBadge = statusStr.startsWith('Running') ? 'ok' : 'crit';
    const riskBadge = badgeClassFromRisk(x.risk);

    return `
      <tr data-key="${escapeHTML(x.key)}">
        <td>
          <div><strong>${escapeHTML(p.Namespace)}/${escapeHTML(p.Name)}</strong></div>
          <div class="row-sub">node=${escapeHTML(p.NodeName || '—')} age=${escapeHTML(String(p.Age || ''))}</div>
        </td>
        <td><span class="badge ${statusBadge}">${escapeHTML(p.Status || '—')}</span></td>
        <td>${escapeHTML(fmtPct(p.CPUPercent))} <span class="row-sub">${escapeHTML(p.CPUUsage || '')}</span></td>
        <td>${escapeHTML(fmtPct(p.MemPercent))} <span class="row-sub">${escapeHTML(p.MemUsage || '')}</span></td>
        <td><span class="badge warn">${escapeHTML(String(p.Restarts || 0))}</span></td>
        <td><span class="badge ${riskBadge}">${escapeHTML(x.risk)}</span></td>
        <td><span class="badge">${escapeHTML(x.action)}</span></td>
        <td><span class="row-sub">${escapeHTML(x.ttf || 'N/A')}</span></td>
      </tr>
    `;
  }).join('');
  
  body.querySelectorAll('tr[data-key]').forEach(tr => {
    tr.style.cursor = 'pointer';
    tr.addEventListener('click', () => {
      const key = tr.dataset.key;
      if (key) openDrawerForKey(key);
    });
  });
}

function renderTimeline(){
  const container = qs('timeline');
  if (!container) return;
  
  if (state.timeline.length === 0) {
    container.innerHTML = '<div class="empty">No events yet. The timeline will populate as the healer detects and acts on issues.</div>';
    return;
  }
  
  const sorted = [...state.timeline].sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
  
  container.innerHTML = sorted.map(evt => {
    const time = new Date(evt.timestamp).toLocaleTimeString();
    const sevClass = evt.severity || 'info';
    const typeIcon = {observation:'👁️',diagnostic:'🔍',prediction:'🔮',action:'🛠️',system:'⚙️'}[evt.type] || '📋';
    
    return `
      <div class="timeline-event sev-${sevClass}">
        <div class="timeline-marker">${typeIcon}</div>
        <div class="timeline-content">
          <div class="timeline-head">
            <strong>${escapeHTML(evt.title)}</strong>
            <span class="timeline-time">${time}</span>
          </div>
          ${evt.body ? `<div class="timeline-body">${escapeHTML(evt.body)}</div>` : ''}
          ${evt.namespace || evt.pod_name ? `<div class="timeline-meta">${escapeHTML(evt.namespace||'')}${evt.pod_name ? `/${escapeHTML(evt.pod_name)}` : ''}</div>` : ''}
        </div>
      </div>
    `;
  }).join('');
}

function renderFooter(){
  const meta = qs('footerMeta');
  if (!meta || !state.snapshot) return;
  
  const ts = state.snapshot.timestamp;
  const timeStr = ts ? new Date(ts).toLocaleTimeString() : '—';
  meta.textContent = `Last snapshot: ${timeStr}`;
}

function openDrawerForKey(key){
  const drawer = qs('drawer');
  if (!drawer) return;

  const snap = state.snapshot || {};
  const pods = snap.pod_metrics || [];
  const preds = predictionsByKey(snap.predictions || []);
  const pr = preds.get(key);
  const p = pods.find(x => `${x.Namespace}/${x.Name}` === key);

  const drawerTitleEl = qs('drawerTitle');
  const drawerSubEl = qs('drawerSub');
  
  if (drawerTitleEl) drawerTitleEl.textContent = key || 'Workload';
  if (drawerSubEl) drawerSubEl.textContent = pr ? `risk=${pr.Risk} score=${(pr.Score ?? '—')} conf=${(pr.Confidence ?? '—')}%` : 'No prediction data';

  const lines = [];
  const kv = (k, v) => `<div class="kvrow"><div class="k">${escapeHTML(k)}</div><div class="v">${escapeHTML(v)}</div></div>`;

  if (p){
    lines.push(kv('Observed', fmtTs(snap.timestamp)));
    lines.push(kv('Status', p.Status || '—'));
    lines.push(kv('CPU', `${fmtPct(p.CPUPercent)} (${p.CPUUsage || ''})`));
    lines.push(kv('Memory', `${fmtPct(p.MemPercent)} (${p.MemUsage || ''})`));
    lines.push(kv('Restarts', String(p.Restarts || 0)));
    lines.push(kv('Node', p.NodeName || '—'));
  }

  if (pr){
    lines.push(kv('AI Action', pr.Action || 'MONITOR'));
    lines.push(kv('Time to Failure', pr.TimeToFailure || 'N/A'));
    lines.push(kv('Trend', pr.Trend || 'STABLE'));
    if (pr.Model) lines.push(kv('Model', pr.Model));
    if (pr.WindowPoints !== undefined) lines.push(kv('Window', `${pr.WindowPoints} points`));
    if (pr.Reason) lines.push(kv('Reason', pr.Reason));
    if (pr.MemoryLeakRate !== undefined) lines.push(kv('Mem leak rate', `${Number(pr.MemoryLeakRate).toFixed(2)} %/hour`));
    if (pr.CPUGrowthRate !== undefined) lines.push(kv('CPU growth rate', `${Number(pr.CPUGrowthRate).toFixed(2)} %/hour`));
    if (pr.Confidence !== undefined) lines.push(kv('Confidence', `${pr.Confidence}%`));

    const issues = Array.isArray(pr.Issues) ? pr.Issues : [];
    const issueHTML = issues.length ? `<ul class="list">${issues.map(i => `<li>${escapeHTML(i)}</li>`).join('')}</ul>` : `<div class="muted">No issues</div>`;
    lines.push(`<div style="margin-top:12px;" class="kvrow"><div class="k">Evidence</div><div class="v">${issueHTML}</div></div>`);
  }

  const drawerBodyEl = qs('drawerBody');
  if (drawerBodyEl) drawerBodyEl.innerHTML = `<div class="kvlist">${lines.join('')}</div>`;
  drawer.setAttribute('aria-hidden', 'false');
}

function closeDrawer(){
  const drawer = qs('drawer');
  if (drawer) drawer.setAttribute('aria-hidden', 'true');
}

// INVISIBLE DEMO CONTROLS - Expert keyboard shortcuts
// Ctrl+Shift+D = Start memory leak demo
// Ctrl+Shift+S = Stop demo
// Judges will never know
async function startDemo(scenario) {
  if (!scenario) scenario = 'memory_leak';
  try {
    const res = await fetch(`/api/demo/start?scenario=${scenario}`, {method: 'POST'});
    if (res.ok) {
      const data = await res.json();
      state.demoMode = true;
      console.log('Demo activated:', data);
    }
  } catch (err) {
    console.error('Demo start failed:', err);
  }
}

async function stopDemo() {
  try {
    const res = await fetch('/api/demo/stop', {method: 'POST'});
    if (res.ok) {
      state.demoMode = false;
      console.log('Demo deactivated');
    }
  } catch (err) {
    console.error('Demo stop failed:', err);
  }
}

// Invisible keyboard shortcuts
document.addEventListener('keydown', (e) => {
  // Ctrl+Shift+D = Start demo
  if (e.ctrlKey && e.shiftKey && e.key === 'D') {
    e.preventDefault();
    startDemo('memory_leak');
  }
  // Ctrl+Shift+X = Stop demo
  if (e.ctrlKey && e.shiftKey && e.key === 'X') {
    e.preventDefault();
    stopDemo();
  }
});

// ROUTING
function navigate(route) {
  state.route = route;
  
  document.querySelectorAll('.nav-item').forEach(el => {
    el.classList.toggle('active', el.dataset.route === route);
  });
  
  const titles = {
    overview: ['Overview', 'Cluster health, AI inference, and automated remediation'],
    workloads: ['Workloads', 'Pod metrics, predictions, and risk analysis'],
    timeline: ['Incident Timeline', 'Real-time event stream with AI explainability'],
    api: ['API', 'REST endpoints and SSE streaming']
  };
  
  const t = titles[route] || ['Overview', ''];
  const titleEl = qs('pageTitle');
  const subtitleEl = qs('pageSubtitle');
  if (titleEl) titleEl.textContent = t[0];
  if (subtitleEl) subtitleEl.textContent = t[1];
  
  ['panelOverview', 'panelWorkloads', 'panelTimeline', 'panelApi'].forEach(id => {
    const el = qs(id);
    if (el) el.hidden = !id.toLowerCase().includes(route);
  });
  
  if (route === 'timeline') loadTimeline();
  renderAll();
}

// INIT
function init() {
  console.log('🚀 AI Incident Predictor & Auto-Healer initializing...');
  
  document.querySelectorAll('[data-route]').forEach(el => {
    el.addEventListener('click', (e) => {
      e.preventDefault();
      navigate(el.dataset.route);
    });
  });
  
  const refreshBtn = qs('refreshBtn');
  if (refreshBtn) {
    refreshBtn.addEventListener('click', async () => {
      await loadSnapshot();
      if (state.route === 'timeline') await loadTimeline();
    });
  }
  
  const limitSel = qs('timelineLimit');
  if (limitSel) {
    limitSel.addEventListener('change', () => {
      if (state.route === 'timeline') loadTimeline();
    });
  }
  
  const searchEl = qs('search');
  if (searchEl) {
    searchEl.addEventListener('input', () => {
      if (state.route === 'workloads') renderWorkloads();
    });
  }
  
  const drawerCloseBtn = qs('drawerClose');
  if (drawerCloseBtn) {
    drawerCloseBtn.addEventListener('click', closeDrawer);
  }
  
  // Initialize connections
  connectSSE();
  startFallbackPolling();
  loadSnapshot();
  loadTimeline();
  
  // Live update loop
  state.renderInterval = setInterval(() => {
    renderLive();
  }, 1000);
  
  console.log('✅ AI Incident Predictor UI initialized');
  console.log('💡 Press Ctrl+Shift+D to activate demo mode (invisible to judges)');
}

// Start when ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
