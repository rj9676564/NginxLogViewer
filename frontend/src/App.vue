<template>
  <div id="app-container">
    <aside class="sidebar">
      <div class="brand">
        <a-avatar :size="32" shape="square">SS</a-avatar>
        <span>Sonic Stellar</span>
      </div>

      <div class="filter-section">
        <div class="search-wrapper">
          <a-input v-model:value="searchText" placeholder="Search logs..." allow-clear class="modern-search">
            <template #prefix>🔍</template>
          </a-input>
        </div>

        <div class="select-group">
          <a-select v-model:value="selectedDevice" placeholder="Device" show-search allow-clear
            @change="handleFilterChange" class="modern-select">
            <template #suffixIcon>📱</template>
            <a-select-option v-for="d in devices" :key="d" :value="d">{{ d }}</a-select-option>
          </a-select>

          <div class="sub-filters">
            <a-select v-model:value="selectedLevel" placeholder="Level" allow-clear
              @change="handleFilterChange" class="modern-select level-select">
              <template #suffixIcon>📊</template>
              <a-select-option value="v">Verbose</a-select-option>
              <a-select-option value="d">Debug</a-select-option>
              <a-select-option value="e">Error</a-select-option>
            </a-select>
            
            <a-select v-model:value="selectedTag" placeholder="Tag" show-search allow-clear
              @change="handleFilterChange" class="modern-select tag-select">
              <template #suffixIcon>🏷️</template>
              <a-select-option v-for="t in tags" :key="t" :value="t">{{ t }}</a-select-option>
            </a-select>
          </div>
        </div>
      </div>

      <div style="display: flex; gap: 8px;">
        <a-button type="primary" :danger="isPaused" @click="togglePause" block ghost>
          <template #icon>
            <span v-if="!isPaused">⏸</span>
            <span v-else>▶️</span>
          </template>
          {{ isPaused ? (backlog.length > 0 ? `Resume (${backlog.length})` : 'Resume') : 'Pause' }}
        </a-button>
        <a-button @click="clearLogs" block>
          🗑 Clear
        </a-button>
      </div>

      <!-- Stats Card -->
      <a-card size="small" :bordered="false" style="background: var(--card-bg); margin-top: auto;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
          <span style="font-size: 12px; font-weight: 600; color: var(--text-secondary);">STATISTICS</span>
          <a-button type="text" size="small" @click="toggleTheme">
            {{ isDarkMode ? '🌙' : '☀️' }}
          </a-button>
        </div>

        <a-statistic title="Total Requests" :value="stats.pv" :value-style="{ color: varTextPrimary }"></a-statistic>
        <div
          style="margin-top: 8px; display: flex; justify-content: space-between; font-size: 12px; color: var(--text-secondary);">
          <span>Unique Visitors</span>
          <span>{{ stats.uv }}</span>
        </div>
        <div style="margin-top: 4px; font-size: 12px; color: var(--text-secondary); display: flex; align-items: center; gap: 6px;">
          Status: <span :style="{ color: isConnected ? '#4ec9b0' : '#f44747' }">{{ isConnected ? 'Live' : 'Offline' }}</span>
          <div v-if="isConnected" class="pulse-dot"></div>
        </div>
        <a-button size="small" type="link" @click="fetchHistory" style="padding: 0; margin-top: 8px;">Load
          History</a-button>
      </a-card>

      <!-- Color Config Card -->
      <a-card size="small" :bordered="false" style="background: var(--card-bg); margin-top: 12px;">
        <div style="font-size: 11px; font-weight: 600; color: var(--text-secondary); margin-bottom: 8px;">LEVEL COLORS
        </div>
        <div style="display: flex; flex-direction: column; gap: 8px;">
          <div style="display: flex; align-items: center; justify-content: space-between;">
            <span style="font-size: 12px;">v (Verbose)</span>
            <input type="color" v-model="levelColors.v" style="border:none; padding:0; width:24px; height:24px; cursor:pointer; background:transparent;" />
          </div>
          <div style="display: flex; align-items: center; justify-content: space-between;">
            <span style="font-size: 12px;">d (Debug)</span>
            <input type="color" v-model="levelColors.d" style="border:none; padding:0; width:24px; height:24px; cursor:pointer; background:transparent;" />
          </div>
          <div style="display: flex; align-items: center; justify-content: space-between;">
            <span style="font-size: 12px;">e (Error)</span>
            <input type="color" v-model="levelColors.e" style="border:none; padding:0; width:24px; height:24px; cursor:pointer; background:transparent;" />
          </div>
        </div>
      </a-card>
    </aside>

    <main class="main">
      <!-- Header Bar -->
      <div class="navbar">
        <div style="display: flex; align-items: center; gap: 12px;">
          <span style="font-weight: 600; font-size: 13px;">{{ logFile }}</span>
          <a-tag color="#2db7f5">{{ logs.length }} Events</a-tag>
        </div>
        <a-button size="small" @click="exportCSV" title="Download filtered logs as CSV">
          <template #icon>📥</template> Export CSV
        </a-button>
      </div>

      <!-- Header Row -->
      <div class="log-row header-row">
        <div class="col-time">Time</div>
        <div class="col-level">Lvl</div>
        <div class="col-tag">Tag</div>
        <div class="col-status">Status</div>
        <div class="col-method">Mthd</div>
        <div class="col-path">Path / Content</div>
        <div class="col-meta">Client</div>
      </div>

      <!-- Virtual List Container -->
      <div class="log-list" ref="listRef" @scroll="handleScroll">
        <!-- Spacer to simulate full scroll height -->
        <div :style="{ height: `${totalHeight}px`, position: 'relative' }">
          <!-- Offset wrapper for visible items -->
          <div :style="{ transform: `translateY(${offsetY}px)` }">
            <div v-for="log in visibleLogs" :key="log.id" class="log-row" @click="showDetail(log)">
              <!-- Time -->
              <div class="col-time" :title="log.time">{{ log.timeOnly }}</div>

              <!-- Level -->
              <div class="col-level">
                <a-tag :color="getLevelColor(log.level)" style="font-size: 10px; min-width: 45px; text-align: center; margin: 0;">
                  {{ (log.level || 'info').toUpperCase() }}
                </a-tag>
              </div>

              <!-- Tag -->
              <div class="col-tag">
                <span v-if="log.tag" class="small-badge" @click.stop="selectedTag = log.tag; handleFilterChange()">{{ log.tag }}</span>
                <span v-else>-</span>
              </div>

              <!-- Status -->
              <div class="col-status status-badge" :class="getStatusClass(log.status)">{{ log.status }}</div>

              <!-- Method -->
              <div class="col-method" :style="{ color: getMethodColor(log.method) }">{{ log.method }}</div>

              <!-- Path + Query + Body Icon -->
              <div class="col-path" :style="getLogStyle(log)">
                <div class="path-container">
                  <template v-if="log.path">
                    <span style="font-weight: 500;" v-html="highlightText((getLogDetails(log)?.text) || formatPath(log.path))"></span>
                    <span v-if="getDisplayQuery(log) && !getLogDetails(log)" class="query-string" :style="getLogStyle(log)">?{{ getDisplayQuery(log) }}</span>
                  </template>
                  <template v-else>
                    <span style="color: var(--text-secondary); opacity: 0.7; font-style: italic;">{{ log.raw }}</span>
                  </template>
                </div>

                <!-- Antd Popover for Body (Hover) -->
                <div class="body-container">
                  <a-popover placement="bottom" title="Request Body" trigger="hover"
                    v-if="log.body && log.body !== '-'">
                    <template #content>
                      <div class="popover-json" :style="{ color: isDarkMode ? '#e2e8f0' : '#333' }">{{ log.body }}</div>
                    </template>
                    <a-tag color="orange" style="margin-left: 8px; cursor: pointer; border-radius: 2px;"
                      title="Click to Inspect Body">BODY</a-tag>
                  </a-popover>
                </div>
              </div>

              <!-- Meta -->
              <div class="col-meta">
                <div class="row-actions">
                  <a-button type="text" size="small" @click.stop="copyLog(log)" title="Copy Log">
                    <template #icon>📋</template>
                  </a-button>
                  <a-button type="text" size="small" @click.stop="showDetail(log)" title="View Details">
                    <template #icon>🔍</template>
                  </a-button>
                </div>
                <a-tooltip v-if="log.os" :title="log.ua">
                  <a-tag :bordered="false" class="meta-tag">{{ log.os }}</a-tag>
                </a-tooltip>
                <a-tooltip v-if="log.device === 'Mobile'" title="Mobile Device">
                  <span>📱</span>
                </a-tooltip>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>

    <!-- Detail Drawer -->
    <a-drawer v-model:open="detailVisible" title="Log Entry Details" placement="right" width="600" :closable="true"
      :body-style="{ padding: '0' }">
      <div v-if="selectedLog" class="detail-content">
        <div class="detail-section">
          <h3>Basic Info</h3>
          <div class="detail-grid">
            <div class="grid-item"><span>ID</span><strong>{{ selectedLog.id }}</strong></div>
            <div class="grid-item"><span>Time</span><strong>{{ selectedLog.time }}</strong></div>
            <div class="grid-item"><span>Status</span><strong :class="getStatusClass(selectedLog.status)">{{
                selectedLog.status }}</strong></div>
            <div class="grid-item"><span>Method</span><strong :style="{ color: getMethodColor(selectedLog.method) }">{{
                selectedLog.method }}</strong></div>
            <div class="grid-item"><span>IP Address</span><strong>{{ selectedLog.ip }}</strong></div>
          </div>
        </div>

        <div class="detail-section">
          <h3>Request</h3>
          <div class="grid-item full"><span>Path</span><code class="code-block">{{ selectedLog.path }}</code></div>
          <div class="grid-item full" v-if="selectedLog.query"><span>Query</span><code class="code-block">{{
              selectedLog.query }}</code></div>
          <div class="grid-item full" v-if="selectedLog.body"><span>Body</span><code class="code-block">{{
              selectedLog.body }}</code></div>
        </div>

        <div class="detail-section">
          <h3>Client Details</h3>
          <div class="detail-grid">
            <div class="grid-item"><span>Device ID</span><a-tag color="blue">{{ selectedLog.device_id || 'N/A' }}</a-tag>
            </div>
            <div class="grid-item"><span>Level</span><a-tag :color="getLevelColor(selectedLog.level)">{{
                selectedLog.level || 'info' }}</a-tag></div>
            <div class="grid-item"><span>Tag</span><a-tag v-if="selectedLog.tag">{{ selectedLog.tag }}</a-tag></div>
            <div class="grid-item"><span>Browser</span><strong>{{ selectedLog.browser }}</strong></div>
            <div class="grid-item"><span>OS</span><strong>{{ selectedLog.os }}</strong></div>
          </div>
          <div class="grid-item full" style="margin-top: 12px;"><span>User Agent</span><div class="ua-text">{{
              selectedLog.ua }}</div>
          </div>
        </div>

        <div class="detail-section">
          <h3>Raw Content</h3>
          <pre class="raw-pre">{{ selectedLog.raw }}</pre>
        </div>
      </div>
    </a-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue';

const logs = ref([]);
const backlog = ref([]); // Buffer for paused logs
const renderBuffer = ref([]); // Buffer for batching updates
const searchText = ref('');
const isPaused = ref(false);
const isConnected = ref(false);
const stats = ref({ pv: 0, uv: 0 });
const listRef = ref(null);
const logFile = ref('access.log');
const isDarkMode = ref(localStorage.getItem('theme') !== 'light');

const detailVisible = ref(false);
const selectedLog = ref(null);

const devices = ref([]);
const tags = ref([]);
const selectedDevice = ref(undefined);
const selectedLevel = ref(undefined);
const selectedTag = ref(undefined);
const levelColors = ref(JSON.parse(localStorage.getItem('levelColors')) || {
  v: isDarkMode.value ? '#cccccc' : '#000000',
  d: '#007acc',
  e: '#f44747'
});

watch(levelColors, (val) => {
  localStorage.setItem('levelColors', JSON.stringify(val));
}, { deep: true });

// --- Virtual Scroll Logic ---
const itemHeight = 48; // Updated to match .log-row height
const scrollTop = ref(0);
const containerHeight = ref(800);
const maxLogs = 2000;
const isAtTop = ref(true);

const visibleCount = computed(() => Math.ceil(containerHeight.value / itemHeight) + 5);
const startIndex = computed(() => Math.floor(scrollTop.value / itemHeight));

const visibleLogs = computed(() => {
  const all = filteredLogs.value;
  return all.slice(startIndex.value, startIndex.value + visibleCount.value);
});

const totalHeight = computed(() => filteredLogs.value.length * itemHeight);
const offsetY = computed(() => startIndex.value * itemHeight);

const handleScroll = (e) => {
  scrollTop.value = e.target.scrollTop;
  isAtTop.value = e.target.scrollTop < 10;
};

const updateContainerHeight = () => {
  if (listRef.value) containerHeight.value = listRef.value.clientHeight;
};

const handleFilterChange = () => {
  clearLogs();
  fetchHistory();
};

// --- Batching Updates Logic ---
// We collect logs in a temporary array and flush to the reactive 'logs' every 100ms
let flushTimer = null;
const flushLogs = () => {
  if (renderBuffer.value.length === 0) return;

  const addedLogs = [...renderBuffer.value].reverse();
  const addedCount = addedLogs.length;
  const newLogs = [...addedLogs, ...logs.value];
  renderBuffer.value = [];

  // Keep limit
  if (newLogs.length > maxLogs) {
    logs.value = newLogs.slice(0, maxLogs);
  } else {
    logs.value = newLogs;
  }

  // Auto scroll logic
  if (isAtTop.value) {
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = 0;
    });
  } else {
    // Adjust scroll to prevent jumping when logs are added at the top
    const adjustment = addedCount * itemHeight;
    nextTick(() => {
      if (listRef.value) {
        listRef.value.scrollTop += adjustment;
      }
    });
  }
};

const handleDeviceChange = () => {
  clearLogs();
  fetchHistory();
};

const getLogLevel = (log) => {
  if (log.level) return log.level;
  if (!log.query && !log.path) return null;
  let qStr = '';
  if (log.query && log.query !== '-') {
    qStr = log.query;
  } else if (log.path && log.path.includes('?')) {
    qStr = log.path.split('?')[1];
  }
  
  if (!qStr) return null;
  const parts = qStr.split('&');
  for (const p of parts) {
    if (p.startsWith('level=')) return p.substring(6);
  }
  return null;
};

const getLogDetails = (log) => {
  if (log.tag || log.level) return { tag: log.tag, text: log.query || log.path };
  if (!log.path || !log.path.startsWith('/log/')) return null;
  const qStr = log.query || (log.path.includes('?') ? log.path.split('?')[1] : '');
  if (!qStr) return null;
  const params = new URLSearchParams(qStr);
  return {
    tag: params.get('tag'),
    text: params.get('text')
  };
};

const getLogStyle = (log) => {
  const level = getLogLevel(log);
  if (level && levelColors.value[level]) {
    return { color: levelColors.value[level], fontWeight: level === 'e' ? '600' : 'normal' };
  }
  return {};
};

const highlightText = (text) => {
  if (!searchText.value || !text) return text;
  const q = searchText.value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const reg = new RegExp(`(${q})`, 'gi');
  return String(text).replace(reg, '<mark class="hl">$1</mark>');
};

const getLevelColor = (lvl) => {
  if (lvl === 'e' || lvl === 'error') return 'red';
  if (lvl === 'd' || lvl === 'debug') return 'blue';
  return 'default';
};

const showDetail = (log) => {
  selectedLog.value = log;
  detailVisible.value = true;
};

const copyLog = (log) => {
  const text = JSON.stringify(log, null, 2);
  navigator.clipboard.writeText(text).then(() => {
    // Simple toast would be better but we don't have it imported easily
    // We'll just use a small effect or skip for now to avoid errors
  });
};

const addLog = (entry) => {
  entry.timeOnly = parseTime(entry.time);
  const frozen = Object.freeze(entry);

  if (isPaused.value) {
    backlog.value.unshift(frozen);
    return;
  }

  renderBuffer.value.push(frozen);
  if (!flushTimer) {
    flushTimer = setInterval(flushLogs, 100); // 10 FPS for updates is plenty and saves CPU
  }
};

const filteredLogs = computed(() => {
  let all = logs.value;
  
  // Filtering is handled by backend for history, but we also filter here for real-time logs
  if (selectedDevice.value) {
    all = all.filter(l => l.device_id === selectedDevice.value);
  }
  if (selectedLevel.value) {
    all = all.filter(l => l.level === selectedLevel.value);
  }
  if (selectedTag.value) {
    all = all.filter(l => l.tag === selectedTag.value);
  }

  if (!searchText.value) return all;
  const q = searchText.value.toLowerCase();
  return all.filter(l =>
    (l.path && l.path.toLowerCase().includes(q)) ||
    (l.ip && l.ip.includes(q)) ||
    (l.device_id && l.device_id.toLowerCase().includes(q)) ||
    (l.tag && l.tag.toLowerCase().includes(q)) ||
    (l.status && String(l.status).includes(q))
  );
});

const varTextPrimary = computed(() => isDarkMode.value ? '#cccccc' : '#333333');

const toggleTheme = () => {
  isDarkMode.value = !isDarkMode.value;
  localStorage.setItem('theme', isDarkMode.value ? 'dark' : 'light');
};

watch(isDarkMode, (val) => {
  document.documentElement.setAttribute('data-theme', val ? 'dark' : 'light');
}, { immediate: true });

const getMethodColor = (m) => {
  const map = { GET: '#4ec9b0', POST: '#569cd6', PUT: '#dcdcaa', DELETE: '#f44747' };
  if (!isDarkMode.value) {
    const lightMap = { GET: '#059669', POST: '#2563eb', PUT: '#d97706', DELETE: '#dc2626' };
    return lightMap[m] || '#666';
  }
  return map[m] || '#cccccc';
};

const getStatusClass = (s) => {
  if (s < 300) return 'c-2xx';
  if (s < 400) return 'c-3xx';
  if (s < 500) return 'c-4xx';
  return 'c-5xx';
};

const formatPath = (path) => {
  if (!path) return '';
  return path.split('?')[0];
};

const getDisplayQuery = (log) => {
  let q = '';
  // Favor log.query if it exists and is not '-'
  if (log.query && log.query !== '-') {
    q = log.query;
    if (q.startsWith('?')) q = q.substring(1);
  } else if (log.path && log.path.includes('?')) {
    // Otherwise try to extract from path
    q = log.path.split('?')[1];
  }

  if (!q) return '';

  try {
    return decodeURIComponent(q);
  } catch (e) {
    return q;
  }
};

const parseTime = (raw) => {
  if (!raw) return '--:--:--';
  // Handle ISO format: 2026-01-27T11:10:07.403091
  if (raw.includes('T')) {
    const timePart = raw.split('T')[1];
    return timePart.split('.')[0];
  }
  // Handle Nginx format: [27/Jan/2026:11:10:07 +0800]
  const parts = raw.split(':');
  if (parts.length >= 4) {
    return parts.slice(1, 4).join(':').split(' ')[0];
  }
  return raw;
};

let socket = null;

const connect = () => {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const host = window.location.host;
  socket = new WebSocket(`${proto}://${host}/ws`);
  socket.onopen = () => isConnected.value = true;
  socket.onclose = () => { isConnected.value = false; setTimeout(connect, 2000); };
  socket.onmessage = e => {
    try {
      addLog(JSON.parse(e.data));
    } catch (err) { }
  };
}

const fetchStats = async () => {
  try { stats.value = await (await fetch('/api/stats')).json(); } catch (e) { }
}

const fetchDevices = async () => {
  try { devices.value = await (await fetch('/api/devices')).json(); } catch (e) { }
}

const fetchTags = async () => {
  try { tags.value = await (await fetch('/api/tags')).json(); } catch (e) { }
}

const fetchHistory = async () => {
  try {
    const params = new URLSearchParams();
    if (selectedDevice.value) params.set('device', selectedDevice.value);
    if (selectedLevel.value) params.set('level', selectedLevel.value);
    if (selectedTag.value) params.set('tag', selectedTag.value);
    
    const url = `/api/history?${params.toString()}`;
    const response = await fetch(url);
    const history = await response.json();
    if (history) {
      logs.value = history.map(l => ({
        ...l,
        timeOnly: parseTime(l.time),
        id: l.id || Math.random()
      })).map(Object.freeze);

      nextTick(() => {
        updateContainerHeight();
        if (listRef.value) listRef.value.scrollTop = 0;
      });
    }
  } catch (e) { }
};

const togglePause = () => {
  isPaused.value = !isPaused.value;
  if (!isPaused.value) {
    if (backlog.value.length > 0) {
      logs.value = [...backlog.value, ...logs.value].slice(0, maxLogs);
      backlog.value = [];
    }
    nextTick(() => {
      if (listRef.value && isAtTop.value) listRef.value.scrollTop = 0;
    });
  }
};

const clearLogs = () => {
  logs.value = [];
  backlog.value = [];
  renderBuffer.value = [];
};

const exportCSV = () => {
  const all = filteredLogs.value;
  if (!all.length) return;

  const headers = ['ID', 'Time', 'Level', 'Tag', 'DeviceID', 'Method', 'Path', 'Status', 'IP'];
  const rows = all.map(l => [
    l.id,
    l.time,
    l.level || 'info',
    l.tag || '',
    l.device_id || '',
    l.method,
    l.path,
    l.status,
    l.ip
  ]);

  const csvContent = [
    headers.join(','),
    ...rows.map(r => r.map(v => `"${String(v).replace(/"/g, '""')}"`).join(','))
  ].join('\n');

  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
  const link = document.createElement("a");
  const url = URL.createObjectURL(blob);
  link.setAttribute("href", url);
  link.setAttribute("download", `logs_export_${new Date().getTime()}.csv`);
  link.style.visibility = 'hidden';
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
};

import { onUnmounted } from 'vue';

onMounted(() => {
  connect();
  fetchHistory();
  fetchDevices();
  fetchTags();
  setInterval(fetchStats, 5000);
  setInterval(fetchDevices, 10000);
  setInterval(fetchTags, 15000);
  updateContainerHeight();
  window.addEventListener('resize', updateContainerHeight);
});

onUnmounted(() => {
  if (flushTimer) clearInterval(flushTimer);
  window.removeEventListener('resize', updateContainerHeight);
});
</script>

<style>
@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;600&family=JetBrains+Mono:wght@400;500&display=swap');

:root {
  /* Premium Dark Mode (Default) 🛰️ */
  --bg-color: #0d0f14;
  --sidebar-bg: rgba(22, 27, 34, 0.85);
  --header-bg: rgba(13, 15, 20, 0.7);
  --border-color: rgba(255, 255, 255, 0.08);
  --row-hover: rgba(255, 255, 255, 0.04);
  --text-primary: #e6edf3;
  --text-secondary: #7d8590;
  --card-bg: rgba(255, 255, 255, 0.03);
  --accent-color: #58a6ff;
  --accent-gradient: linear-gradient(135deg, #58a6ff 0%, #1f6feb 100%);
  --path-color: #d2a8ff;
  --header-text: #fff;
  --tag-bg: rgba(110, 118, 129, 0.4);
  --tag-color: #adbac7;
  --scrollbar-track: #0d0f14;
  --scrollbar-thumb: #30363d;
  --scrollbar-thumb-border: #0d0f14;
  --glass-blend: blur(12px) saturate(180%);
}

:root[data-theme='light'] {
  --bg-color: #f6f8fa;
  --sidebar-bg: rgba(255, 255, 255, 0.8);
  --header-bg: rgba(246, 248, 250, 0.7);
  --border-color: #d0d7de;
  --row-hover: #f3f4f6;
  --text-primary: #1f2328;
  --text-secondary: #656d76;
  --card-bg: #ffffff;
  --accent-color: #0969da;
  --accent-gradient: linear-gradient(135deg, #0969da 0%, #03449d 100%);
  --path-color: #8250df;
  --header-text: #24292f;
  --tag-bg: #eff2f5;
  --tag-color: #57606a;
  --scrollbar-track: #f6f8fa;
  --scrollbar-thumb: #d0d7de;
  --scrollbar-thumb-border: #f6f8fa;
}

body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji";
  background-color: var(--bg-color);
  color: var(--text-primary);
  margin: 0;
  overflow: hidden;
  -webkit-font-smoothing: antialiased;
}

#app-container {
  display: flex;
  height: 100vh;
  width: 100vw;
  background-image: radial-gradient(circle at 50% 50%, rgba(88, 166, 255, 0.05) 0%, transparent 50%);
}

.sidebar {
  width: 300px;
  background: var(--sidebar-bg);
  backdrop-filter: var(--glass-blend);
  -webkit-backdrop-filter: var(--glass-blend);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  padding: 24px 16px;
  gap: 20px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  z-index: 10;
}

.brand {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.brand .ant-avatar {
  box-shadow: 0 4px 12px rgba(88, 166, 255, 0.3);
  background: var(--accent-gradient) !important;
}

/* AntD Overrides for Modernity */
.ant-btn {
  border-radius: 8px !important;
  font-weight: 500 !important;
  transition: all 0.2s !important;
}

.modern-search, .modern-select .ant-select-selector {
  border-radius: 10px !important;
  background: rgba(110, 118, 129, 0.08) !important;
  border: 1px solid var(--border-color) !important;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1) !important;
  height: 40px !important;
  display: flex !important;
  align-items: center !important;
  padding: 0 12px !important;
}

.modern-search:hover, .modern-select:hover .ant-select-selector {
  border-color: var(--accent-color) !important;
  background: rgba(110, 118, 129, 0.12) !important;
}

.modern-search.ant-input-affix-wrapper-focused, .ant-select-focused .ant-select-selector {
  box-shadow: 0 0 0 2px rgba(88, 166, 255, 0.2) !important;
  border-color: var(--accent-color) !important;
  background: var(--bg-color) !important;
}

.ant-input {
  background: transparent !important;
  border: none !important;
}

.ant-input:focus {
  box-shadow: none !important;
}

.ant-select-selection-placeholder {
  line-height: 38px !important;
}

.ant-select-selection-item {
  line-height: 38px !important;
  font-weight: 500;
}

.filter-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.modern-select {
  width: 100%;
}

.sub-filters {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}

.level-select {
  flex: 1;
}

.tag-select {
  flex: 1.5;
}

.ant-card {
  border-radius: 12px !important;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1) !important;
}

.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: transparent;
  min-width: 0;
  position: relative;
}

.navbar {
  height: 64px;
  border-bottom: 1px solid var(--border-color);
  background: var(--header-bg);
  backdrop-filter: var(--glass-blend);
  -webkit-backdrop-filter: var(--glass-blend);
  display: flex;
  align-items: center;
  padding: 0 24px;
  justify-content: space-between;
  position: sticky;
  top: 0;
  z-index: 5;
}

.log-list {
  flex: 1;
  overflow-y: auto;
  position: relative;
  scrollbar-gutter: stable;
}

.log-row {
  display: flex;
  align-items: center;
  padding: 0 24px;
  border-bottom: 1px solid var(--border-color);
  font-size: 13px;
  cursor: pointer;
  gap: 16px;
  transition: all 0.2s ease;
  height: 48px;
}

.log-row:hover {
  background-color: var(--row-hover);
  transform: translateX(4px);
  border-left: 2px solid var(--accent-color);
}

.header-row {
  position: sticky;
  top: 0;
  background: var(--header-bg);
  font-weight: 600;
  color: var(--text-secondary);
  border-bottom: 2px solid var(--border-color);
  text-transform: uppercase;
  font-size: 11px;
  letter-spacing: 0.05em;
  z-index: 2;
  height: 40px;
  transform: none !important;
}

.col-time {
  min-width: 85px;
  color: var(--text-secondary);
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  flex-shrink: 0;
}

.col-level {
  width: 65px;
  flex-shrink: 0;
  display: flex;
  justify-content: center;
}

.col-tag {
  width: 100px;
  flex-shrink: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.small-badge {
  background: var(--tag-bg);
  color: var(--text-primary);
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  border: 1px solid var(--border-color);
  cursor: pointer;
}

.small-badge:hover {
  border-color: var(--accent-color);
}

.col-method {
  width: 55px;
  font-weight: 700;
  text-align: center;
  font-size: 11px;
  flex-shrink: 0;
}

.col-path {
  flex: 1;
  font-family: 'JetBrains Mono', monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--path-color);
  display: flex;
  align-items: center;
  gap: 10px;
}

.path-container {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  overflow: hidden;
}

.col-status {
  width: 60px;
  flex-shrink: 0;
  text-align: center;
  font-weight: 700;
  font-family: 'JetBrains Mono', monospace;
}

.status-badge {
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(110, 118, 129, 0.1);
}

.col-meta {
  width: 180px;
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  flex-shrink: 0;
  align-items: center;
}

.row-actions {
  display: none;
  background: var(--bg-color);
  border-radius: 6px;
  padding: 2px 4px;
  border: 1px solid var(--border-color);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.log-row:hover .row-actions {
  display: flex;
  position: absolute;
  right: 180px;
}

.meta-tag {
  background: var(--tag-bg) !important;
  color: var(--tag-color) !important;
}

.query-string {
  color: var(--accent-color);
  margin-left: 8px;
  opacity: 0.7;
}

.status-badge {
  font-weight: 600;
}

.c-2xx { color: #3fb950; }
.c-3xx { color: #58a6ff; }
.c-4xx { color: #d29922; }
.c-5xx { color: #f85149; }

:root[data-theme='light'] .c-2xx { color: #1a7f37; }
:root[data-theme='light'] .c-3xx { color: #0969da; }
:root[data-theme='light'] .c-4xx { color: #9a6700; }
:root[data-theme='light'] .c-5xx { color: #cf222e; }

/* Detail Drawer Styles */
.detail-content {
  padding: 32px;
  background: var(--bg-color);
  color: var(--text-primary);
  height: 100%;
  overflow-y: auto;
}

.detail-section {
  margin-bottom: 32px;
}

.detail-section h3 {
  font-size: 12px;
  font-weight: 700;
  color: var(--accent-color);
  margin-bottom: 20px;
  border-bottom: 1px solid var(--border-color);
  padding-bottom: 12px;
  text-transform: uppercase;
  letter-spacing: 0.1em;
}

.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.grid-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.grid-item span {
  font-size: 10px;
  color: var(--text-secondary);
  font-weight: 600;
  text-transform: uppercase;
}

.grid-item strong {
  font-size: 14px;
  word-break: break-all;
}

.grid-item.full {
  grid-column: span 2;
}

.code-block {
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  background: var(--tag-bg);
  padding: 12px;
  border-radius: 8px;
  display: block;
  white-space: pre-wrap;
  word-break: break-all;
  border: 1px solid var(--border-color);
}

.ua-text {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.6;
  background: var(--tag-bg);
  padding: 12px;
  border-radius: 8px;
}

.raw-pre {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  background: #0d1117;
  color: #39d353;
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
  white-space: pre-wrap;
  border: 1px solid var(--border-color);
}

.popover-json {
  max-width: 500px;
  max-height: 400px;
  overflow: auto;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  padding: 12px;
}

::-webkit-scrollbar {
  width: 8px;
  height: 8px;
  background: var(--scrollbar-track);
}

::-webkit-scrollbar-thumb {
  background: var(--scrollbar-thumb);
  border-radius: 10px;
  border: 2px solid var(--scrollbar-thumb-border);
}

::-webkit-scrollbar-corner {
  background: var(--scrollbar-track);
}

.pulse-dot {
  width: 8px;
  height: 8px;
  background-color: #3fb950;
  border-radius: 50%;
  box-shadow: 0 0 0 rgba(63, 185, 80, 0.4);
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(63, 185, 80, 0.7); }
  70% { transform: scale(1); box-shadow: 0 0 0 10px rgba(63, 185, 80, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(63, 185, 80, 0); }
}

mark.hl {
  background: rgba(255, 235, 59, 0.3);
  color: inherit;
  padding: 0 1px;
  border-radius: 2px;
  box-shadow: 0 0 8px rgba(255, 235, 59, 0.2);
}
</style>
