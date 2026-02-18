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
};

function qs(id){ return document.getElementById(id); }

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

	// If the connection hangs (no open/error), fail fast so we can retry/poll.
	state.connectWatchdog = setTimeout(() => {
		state.connectWatchdog = null;
		if (!state.lastPingAt && state.streamStatus === 'connecting') {
			state.streamStatus = 'error';
			renderLive();
			qs('liveState').textContent = 'Stream timeout';
			try { es.close(); } catch (_) {}
			connectSSE();
		}
	}, 4000);

    es.addEventListener('open', () => {
      state.streamStatus = 'open';
      if (!state.connectedAt) state.connectedAt = Date.now();
      state.lastPingAt = Date.now();
		if (state.connectWatchdog) {
			clearTimeout(state.connectWatchdog);
			state.connectWatchdog = null;
		}
      renderLive();
      qs('liveState').textContent = 'Connected';
    });

    es.addEventListener('ready', () => {
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
      if (state.streamStatus !== 'open') state.streamStatus = 'open';
      renderLive();
    });

    es.addEventListener('update', async () => {
      const now = Date.now();
      state.lastPingAt = now;
      state.lastUpdateAt = now;
      state.streamStatus = 'open';
      await loadSnapshot();
      if (state.route === 'timeline') await loadTimeline();
    });

    es.addEventListener('error', () => {
      state.streamStatus = 'error';
      renderLive();
      qs('liveState').textContent = 'Reconnecting…';
		if (state.connectWatchdog) {
			clearTimeout(state.connectWatchdog);
			state.connectWatchdog = null;
		}

      // EventSource will retry internally, but we also force a hard reconnect
      // to recover reliably from proxies/port-forward interruptions.
      if (!state.reconnectTimer && es.readyState === EventSource.CLOSED) {
        state.reconnectTimer = setTimeout(() => {
          state.reconnectTimer = null;
          connectSSE();
        }, 2500);
      }
    });

  } catch (e) {
    state.streamStatus = 'error';
    renderLive();
    qs('liveState').textContent = 'Unavailable';
  }
}

function fmtPct(v){
  if (v === null || v === undefined) return '—';
  if (typeof v === 'number' && (Number.isNaN(v) || v < 0)) return '—';
  return `${v.toFixed(1)}%`;
}

function fmtTs(ts){
  if (!ts) return '—';
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString();
}

function badgeClassFromRisk(r){
  const risk = (r || '').toUpperCase();
  if (risk === 'CRITICAL') return 'crit';
  if (risk === 'HIGH') return 'warn';
  if (risk === 'MEDIUM' || risk === 'LOW-MEDIUM') return 'warn';
  return 'ok';
}

function healthPillClass(h){
  const v = (h || '').toUpperCase();
  if (v === 'CRITICAL') return 'crit';
  if (v === 'WARNING') return 'warn';
  return 'ok';
}

async function fetchJSON(url){
  const res = await fetch(url, { cache: 'no-store' });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return await res.json();
}

async function loadSnapshot(){
  state.snapshot = await fetchJSON('/api/v1/snapshot');
  render();
}

async function loadTimeline(){
  const limit = Number(qs('timelineLimit').value || 300);
  const data = await fetchJSON(`/api/v1/timeline?limit=${encodeURIComponent(limit)}`);
  state.timeline = (data && data.items) ? data.items : [];
  renderTimeline();
}

function setRoute(route){
  state.route = route;

  for (const el of document.querySelectorAll('.nav-item')){
    el.classList.toggle('active', el.dataset.route === route);
  }

  qs('panelOverview').hidden = route !== 'overview';
  qs('panelWorkloads').hidden = route !== 'workloads';
  qs('panelTimeline').hidden = route !== 'timeline';
  qs('panelApi').hidden = route !== 'api';

  const titles = {
    overview: ['Overview', 'Cluster health, AI inference, and automated remediation'],
    workloads: ['Workloads', 'Risk-ranked workloads with AI guidance'],
    timeline: ['Incident Timeline', 'Explainability feed: observations → inference → actions'],
    api: ['API', 'Versioned endpoints backing this console'],
  };

  const [t, s] = titles[route] || titles.overview;
  qs('pageTitle').textContent = t;
  qs('pageSubtitle').textContent = s;

  if (route === 'timeline') loadTimeline().catch(()=>{});
}

function renderKPIs(){
  const grid = qs('kpiGrid');
  const snap = state.snapshot;

  const pods = (snap && snap.pod_metrics) ? snap.pod_metrics : [];
  const preds = (snap && snap.predictions) ? snap.predictions : [];
  const actions = (snap && snap.healing_actions) ? snap.healing_actions : [];
  const stuck = (snap && snap.stuck_containers) ? snap.stuck_containers : [];
  const checks = (snap && snap.container_checks) ? snap.container_checks : [];

  const critCount = preds.filter(p => (p.Risk || '').toUpperCase() === 'CRITICAL').length;
  const highCount = preds.filter(p => (p.Risk || '').toUpperCase() === 'HIGH').length;

  const cards = [
    { title: 'Observed Pods', value: String(pods.length), sub: 'Excluded: kube-* / healer-*' },
    { title: 'Predictions', value: String(preds.length), sub: `Critical: ${critCount} | High: ${highCount}` },
    { title: 'Diagnostics', value: String(stuck.length + checks.length), sub: `Stuck: ${stuck.length} | Checks: ${checks.length}` },
    { title: 'Remediations', value: String(actions.length), sub: snap && snap.dry_run ? 'Dry-run: actions simulated' : 'Live: actions applied' },
  ];

  grid.innerHTML = cards.map(c => `
    <div class="card">
      <div class="card-title">${escapeHTML(c.title)}</div>
      <div class="card-value">${escapeHTML(c.value)}</div>
      <div class="card-sub">${escapeHTML(c.sub)}</div>
    </div>
  `).join('');
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
  const preds = (snap && snap.predictions) ? snap.predictions : [];
  const m = riskMap(preds);
  const total = Math.max(1, preds.length);

  const set = (idFill, idVal, count) => {
    qs(idVal).textContent = String(count);
    qs(idFill).style.width = `${Math.round((count/total)*100)}%`;
  };

  set('riskCritical', 'riskCriticalVal', m.CRITICAL);
  set('riskHigh', 'riskHighVal', m.HIGH);
  set('riskMed', 'riskMedVal', m.MEDIUM);
  set('riskLow', 'riskLowVal', m.LOW);
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

  const search = (qs('search').value || '').toLowerCase();

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

  for (const tr of body.querySelectorAll('tr[data-key]')){
    tr.addEventListener('click', () => {
      const key = tr.getAttribute('data-key');
      openDrawerForKey(key);
    });
  }
}

function renderTimeline(){
  const host = qs('timeline');
  const items = state.timeline || [];

  host.innerHTML = items.slice().reverse().map(e => {
    const sev = (e.severity || 'info').toLowerCase();
    const key = (e.namespace && e.pod_name) ? `${e.namespace}/${e.pod_name}` : '';
    const meta = [
      fmtTs(e.timestamp),
      e.type ? `type=${e.type}` : '',
      e.correlation_id ? `corr=${e.correlation_id}` : '',
      (e.namespace && e.pod_name) ? `${e.namespace}/${e.pod_name}` : ''
    ].filter(Boolean).join(' · ');

    return `
      <div class="event ${escapeHTML(sev)}" ${key ? `data-key="${escapeHTML(key)}"` : ''}>
        <div class="event-head">
          <div>
            <div class="event-title">${escapeHTML(e.title || 'Event')}</div>
            <div class="event-meta">${escapeHTML(meta)}</div>
          </div>
        </div>
        <div class="event-body">${escapeHTML(e.body || '')}</div>
      </div>
    `;
  }).join('');

  for (const ev of host.querySelectorAll('.event[data-key]')){
    ev.style.cursor = 'pointer';
    ev.addEventListener('click', () => {
      const key = ev.getAttribute('data-key');
      if (key) openDrawerForKey(key);
    });
  }
}

function renderTopMeta(){
  const snap = state.snapshot;
  if (!snap) return;

  const last = snap.timestamp;
  qs('lastSync').textContent = fmtTs(last);
  qs('mode').textContent = snap.dry_run ? 'DRY-RUN' : 'LIVE';

  const health = computeSystemHealth(snap);
  const pill = qs('systemHealth');
  pill.textContent = health;
  pill.className = `pill ${healthPillClass(health)}`;

  qs('footerMeta').textContent = `snapshot=${fmtTs(last)} | pods=${(snap.pod_metrics || []).length} | preds=${(snap.predictions || []).length}`;
}

function renderLive(){
  const dot = qs('liveDot');
  const text = qs('liveText');
  if (!dot || !text) return;

  const now = Date.now();
  const hbAgeMs = state.lastPingAt ? (now - state.lastPingAt) : null;
  const dataAgeMs = state.lastUpdateAt ? (now - state.lastUpdateAt) : null;
  const upMs = state.connectedAt ? (now - state.connectedAt) : null;
  let cls = '';
  let msg = 'Connecting…';

  if (state.streamStatus === 'error') {
    cls = 'warn';
    msg = 'Reconnecting…';
  } else if (!state.lastPingAt) {
    cls = '';
    msg = 'Connecting…';
  } else if (hbAgeMs !== null && hbAgeMs < 8000) {
    cls = 'on';
    const parts = [];
    if (upMs !== null) parts.push(`up ${Math.max(1, Math.round(upMs/1000))}s`);
    if (dataAgeMs !== null) parts.push(`data ${Math.round(dataAgeMs/1000)}s`);
    msg = `Live (${parts.join(' · ') || 'ok'})`;
  } else if (hbAgeMs !== null && hbAgeMs < 20000) {
    cls = 'warn';
    msg = `Stale (${Math.round(hbAgeMs/1000)}s)`;
  } else {
    cls = 'crit';
    msg = `Offline (${Math.round(hbAgeMs/1000)}s)`;
  }

  dot.className = `dot ${cls}`;
  text.textContent = msg;
}

function openDrawerForKey(key){
  const drawer = qs('drawer');
  if (!drawer) return;

  const snap = state.snapshot || {};
  const pods = snap.pod_metrics || [];
  const preds = predictionsByKey(snap.predictions || []);
  const pr = preds.get(key);
  const p = pods.find(x => `${x.Namespace}/${x.Name}` === key);

  qs('drawerTitle').textContent = key || 'Workload';
  qs('drawerSub').textContent = pr ? `risk=${pr.Risk} score=${(pr.Score ?? '—')} conf=${(pr.Confidence ?? '—')}%` : 'No prediction data for this workload';

  const lines = [];
  const kv = (k, v) => `
    <div class="kvrow">
      <div class="k">${escapeHTML(k)}</div>
      <div class="v">${escapeHTML(v)}</div>
    </div>
  `;

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
    const issueHTML = issues.length ? `<ul class="list">${issues.map(i => `<li>${escapeHTML(i)}</li>`).join('')}</ul>` : `<div class="muted">No issues recorded</div>`;
    lines.push(`
      <div style="margin-top:12px;" class="kvrow">
        <div class="k">Evidence</div>
        <div class="v" style="font-family: var(--sans);">
          ${issueHTML}
        </div>
      </div>
    `);

		const sigs = Array.isArray(pr.Signals) ? pr.Signals : [];
		if (sigs.length) {
			const sigRows = sigs
				.slice()
				.sort((a,b) => (Number(b.Score||0) - Number(a.Score||0)))
				.map(s => {
					const sev = (s.Severity || '').toLowerCase();
					const badge = sev === 'critical' ? 'crit' : (sev === 'warning' ? 'warn' : 'ok');
					return `
						<tr>
							<td><strong>${escapeHTML(s.Name || 'Signal')}</strong><div class="row-sub">${escapeHTML(s.Note || '')}</div></td>
							<td><span class="badge ${badge}">${escapeHTML(String(s.Severity || 'info'))}</span></td>
							<td>${escapeHTML((Number(s.Current||0)).toFixed(2))}</td>
							<td>${escapeHTML((Number(s.Baseline||0)).toFixed(2))}</td>
							<td>${escapeHTML((Number(s.Delta||0)).toFixed(2))}</td>
							<td>${escapeHTML((Number(s.Score||0)).toFixed(1))}</td>
						</tr>
					`;
				}).join('');

			lines.push(`
				<div style="margin-top:12px;" class="kvrow">
					<div class="k">Signals</div>
					<div class="v">
						<div class="table-wrap">
							<table class="table">
								<thead>
									<tr>
										<th>Signal</th>
										<th>Sev</th>
										<th>Cur</th>
										<th>Base</th>
										<th>Δ</th>
										<th>Pts</th>
									</tr>
								</thead>
								<tbody>
									${sigRows}
								</tbody>
							</table>
						</div>
					</div>
				</div>
			`);
		}
  }

  qs('drawerBody').innerHTML = `<div class="kvlist">${lines.join('')}</div>`;

  drawer.classList.add('open');
  drawer.setAttribute('aria-hidden', 'false');
}

function closeDrawer(){
  const drawer = qs('drawer');
  if (!drawer) return;
  drawer.classList.remove('open');
  drawer.setAttribute('aria-hidden', 'true');
}

function computeSystemHealth(snap){
  const preds = snap.predictions || [];
  const crit = preds.some(p => (p.Risk || '').toUpperCase() === 'CRITICAL');
  const high = preds.some(p => (p.Risk || '').toUpperCase() === 'HIGH');
  if (crit) return 'CRITICAL';
  if (high) return 'WARNING';
  return 'HEALTHY';
}

function render(){
  renderKPIs();
  renderRiskBars();
  renderWorkloads();
  renderTopMeta();
  renderLive();
}

function escapeHTML(s){
  return String(s)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function bootNav(){
  for (const el of document.querySelectorAll('.nav-item')){
    el.addEventListener('click', (ev) => {
      ev.preventDefault();
      setRoute(el.dataset.route);
    });
  }

  const onHash = () => {
    const h = (location.hash || '#overview').replace('#','');
    setRoute(h);
  };
  window.addEventListener('hashchange', onHash);
  onHash();
}

function bootSSE(){
  connectSSE();
}

async function boot(){
  bootNav();

  qs('refreshBtn').addEventListener('click', async () => {
    await loadSnapshot();
    if (state.route === 'timeline') await loadTimeline();
    connectSSE();
  });

  qs('timelineLimit').addEventListener('change', () => loadTimeline().catch(()=>{}));
  qs('search').addEventListener('input', () => renderWorkloads());
  qs('drawerClose').addEventListener('click', () => closeDrawer());

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') closeDrawer();
  });

  await loadSnapshot();
  bootSSE();

  setInterval(() => renderLive(), 1000);

	// Fallback polling: if SSE is down/hanging, keep the dashboard live.
	setInterval(async () => {
		const hbAgeMs = state.lastPingAt ? (Date.now() - state.lastPingAt) : null;
		const sseHealthy = state.streamStatus === 'open' && hbAgeMs !== null && hbAgeMs < 15000;
		if (sseHealthy) return;
		try {
			await loadSnapshot();
			if (state.route === 'timeline') await loadTimeline();
			qs('liveState').textContent = (state.streamStatus === 'open') ? 'Connected' : 'Polling (stream down)';
		} catch (_) {
			qs('liveState').textContent = 'API unreachable';
		}
	}, 6000);
}

boot().catch((e) => {
  console.error(e);
  qs('liveState').textContent = 'Error';
});
