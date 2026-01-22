<template>
  <div id="app-container">
    <aside class="sidebar">
      <div class="brand">
        <a-avatar size="small" style="background-color: var(--accent-color)"><template #icon>⚡️</template></a-avatar>
        Sonic Stellar
      </div>

      <a-input-search v-model:value="searchText" placeholder="Filter (Cmd+K)" allow-clear
        style="margin-bottom: 8px"></a-input-search>

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
        <div style="margin-top: 4px; font-size: 12px; color: var(--text-secondary);">
          Status: <span :style="{ color: isConnected ? '#4ec9b0' : '#f44747' }">{{ isConnected ? 'Live' : 'Offline'
          }}</span>
        </div>
        <a-button size="small" type="link" @click="fetchHistory" style="padding: 0; margin-top: 8px;">Load
          History</a-button>
      </a-card>
    </aside>

    <main class="main">
      <!-- Header Bar -->
      <div class="navbar">
        <span style="font-weight: 600; font-size: 13px;">{{ logFile }}</span>
        <a-tag color="#2db7f5">{{ logs.length }} Events</a-tag>
      </div>

      <!-- Header Row -->
      <div class="log-row header-row">
        <div class="col-time">Time</div>
        <div class="col-status">Status</div>
        <div class="col-method">Method</div>
        <div class="col-path">Path / Query</div>
        <div class="col-meta">Client</div>
      </div>

      <!-- Virtual List Container -->
      <div class="log-list" ref="listRef" @scroll="handleScroll">
        <!-- Spacer to simulate full scroll height -->
        <div :style="{ height: `${totalHeight}px`, position: 'relative' }">
          <!-- Offset wrapper for visible items -->
          <div :style="{ transform: `translateY(${offsetY}px)` }">
            <div v-for="log in visibleLogs" :key="log.id || Math.random()" class="log-row">
              <!-- Time -->
              <div class="col-time" :title="log.time">{{ log.timeOnly }}</div>

              <!-- Status -->
              <div class="col-status status-badge" :class="getStatusClass(log.status)">{{ log.status }}</div>

              <!-- Method -->
              <div class="col-method" :style="{ color: getMethodColor(log.method) }">{{ log.method }}</div>

              <!-- Path + Query + Body Icon -->
              <div class="col-path">
                <div class="path-container">
                  <template v-if="log.path">
                    <span :title="log.path">{{ formatPath(log.path) }}</span>
                    <span v-if="getDisplayQuery(log)" class="query-string">?{{ getDisplayQuery(log) }}</span>
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
                      @click="isPaused = true" title="Click to Pause & Inspect">BODY</a-tag>
                  </a-popover>
                </div>
              </div>

              <!-- Meta -->
              <div class="col-meta">
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

// --- Virtual Scroll Logic ---
const itemHeight = 36;
const scrollTop = ref(0);
const containerHeight = ref(800);
const maxLogs = 2000;

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
};

const updateContainerHeight = () => {
  if (listRef.value) containerHeight.value = listRef.value.clientHeight;
};

// --- Batching Updates Logic ---
// We collect logs in a temporary array and flush to the reactive 'logs' every 100ms
let flushTimer = null;
const flushLogs = () => {
  if (renderBuffer.value.length === 0) return;

  const newLogs = [...logs.value, ...renderBuffer.value];
  renderBuffer.value = [];

  // Keep limit
  if (newLogs.length > maxLogs) {
    logs.value = newLogs.slice(newLogs.length - maxLogs);
  } else {
    logs.value = newLogs;
  }

  // Auto scroll
  nextTick(() => {
    if (listRef.value) {
      const el = listRef.value;
      if (el.scrollHeight - el.scrollTop - el.clientHeight < 200) {
        el.scrollTop = el.scrollHeight;
      }
    }
  });
};

const addLog = (entry) => {
  entry.timeOnly = parseTime(entry.time);
  const frozen = Object.freeze(entry);

  if (isPaused.value) {
    backlog.value.push(frozen);
    return;
  }

  renderBuffer.value.push(frozen);
  if (!flushTimer) {
    flushTimer = setInterval(flushLogs, 100); // 10 FPS for updates is plenty and saves CPU
  }
};

const filteredLogs = computed(() => {
  const all = logs.value;
  if (!searchText.value) return all;
  const q = searchText.value.toLowerCase();
  return all.filter(l =>
    (l.path && l.path.toLowerCase().includes(q)) ||
    (l.ip && l.ip.includes(q)) ||
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

const fetchHistory = async () => {
  try {
    const response = await fetch('/api/history');
    const history = await response.json();
    if (history) {
      logs.value = history.reverse().map(l => ({
        ...l,
        timeOnly: parseTime(l.time),
        id: l.id || Math.random()
      })).map(Object.freeze);

      nextTick(() => {
        updateContainerHeight();
        if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight;
      });
    }
  } catch (e) { }
};

const togglePause = () => {
  isPaused.value = !isPaused.value;
  if (!isPaused.value) {
    if (backlog.value.length > 0) {
      logs.value = [...logs.value, ...backlog.value].slice(-maxLogs);
      backlog.value = [];
    }
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight;
    });
  }
};

const clearLogs = () => {
  logs.value = [];
  backlog.value = [];
  renderBuffer.value = [];
};

import { onUnmounted } from 'vue';

onMounted(() => {
  connect();
  fetchHistory();
  setInterval(fetchStats, 5000);
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
  /* Dark Mode (Default) */
  --bg-color: #1e1e1e;
  --sidebar-bg: #252526;
  --header-bg: #252526;
  --border-color: #333333;
  --row-hover: #2a2d2e;
  --text-primary: #cccccc;
  --text-secondary: #858585;
  --card-bg: rgba(255, 255, 255, 0.04);
  --accent-color: #007acc;
  --path-color: #d4d4d4;
  --header-text: #fff;
  --tag-bg: rgba(255, 255, 255, 0.1);
  --tag-color: #ccc;
  --scrollbar-track: #1e1e1e;
  --scrollbar-thumb: #424242;
  --scrollbar-thumb-border: #1e1e1e;
}

:root[data-theme='light'] {
  --bg-color: #ffffff;
  --sidebar-bg: #f8f9fa;
  --header-bg: #f1f3f5;
  --border-color: #e9ecef;
  --row-hover: #f1f3f5;
  --text-primary: #343a40;
  --text-secondary: #868e96;
  --card-bg: #fff;
  --accent-color: #228be6;
  --path-color: #212529;
  --header-text: #495057;
  --tag-bg: #e9ecef;
  --tag-color: #495057;
  --scrollbar-track: #fff;
  --scrollbar-thumb: #ced4da;
  --scrollbar-thumb-border: #fff;
}

body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, Arial, sans-serif;
  background-color: var(--bg-color);
  color: var(--text-primary);
  margin: 0;
  overflow: hidden;
}

#app-container {
  display: flex;
  height: 100vh;
  width: 100vw;
}

.sidebar {
  width: 280px;
  background: var(--sidebar-bg);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  padding: 16px;
  gap: 16px;
  transition: background 0.2s, border-color 0.2s;
}

.brand {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--bg-color);
  min-width: 0;
  transition: background 0.2s;
}

.navbar {
  height: 48px;
  border-bottom: 1px solid var(--border-color);
  background: var(--header-bg);
  display: flex;
  align-items: center;
  padding: 0 16px;
  justify-content: space-between;
  transition: background 0.2s, border-color 0.2s;
}

.log-list {
  flex: 1;
  overflow-y: auto;
  position: relative;
}

.log-row {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  border-bottom: 1px solid var(--border-color);
  font-size: 12px;
  cursor: default;
  gap: 12px;
  transition: background 0.1s;
  height: 36px;
}

.log-row:hover {
  background-color: var(--row-hover);
}

.header-row {
  background: var(--header-bg);
  font-weight: 600;
  color: var(--header-text);
  border-bottom: 2px solid var(--border-color);
}

.col-time {
  width: 80px;
  color: var(--text-secondary);
  white-space: nowrap;
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
}

.col-method {
  width: 50px;
  font-weight: 600;
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
  gap: 8px;

  .path-container {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .body-container {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}

.col-status {
  width: 50px;
  flex-shrink: 0;
  text-align: center;
}

.col-meta {
  width: 140px;
  display: flex;
  gap: 4px;
  justify-content: flex-end;
  flex-shrink: 0;
}

.meta-tag {
  background: var(--tag-bg) !important;
  color: var(--tag-color) !important;
}

.query-string {
  color: var(--accent-color);
  margin-left: 8px;
  opacity: 0.8;
}

.status-badge {
  font-weight: 600;
}

.c-2xx {
  color: #4ec9b0;
}

.c-3xx {
  color: #569cd6;
}

.c-4xx {
  color: #ce9178;
}

.c-5xx {
  color: #f44747;
}

:root[data-theme='light'] .c-2xx {
  color: #059669;
}

:root[data-theme='light'] .c-3xx {
  color: #2563eb;
}

:root[data-theme='light'] .c-4xx {
  color: #d97706;
}

:root[data-theme='light'] .c-5xx {
  color: #dc2626;
}

.popover-json {
  max-width: 400px;
  max-height: 300px;
  overflow: auto;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

::-webkit-scrollbar {
  width: 10px;
  height: 10px;
  background: var(--scrollbar-track);
}

::-webkit-scrollbar-thumb {
  background: var(--scrollbar-thumb);
  border-radius: 5px;
  border: 2px solid var(--scrollbar-thumb-border);
}

::-webkit-scrollbar-corner {
  background: var(--scrollbar-track);
}
</style>
