/**
 * Sonic Stellar Logger - JavaScript Implementation
 * 
 * A simple utility to send logs from JS/Node.js environments.
 */

class SonicLogger {
  /**
   * @param {string} baseUrl - The base URL of your Sonic Stellar server (e.g., http://localhost:58080)
   * @param {string} deviceId - A unique identifier for this client
   */
  constructor(baseUrl, deviceId) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.deviceId = deviceId;
  }

  /**
   * Post a single log entry using the simplified Push API
   * @param {string} text - The log message
   * @param {Object} options - Optional metadata
   * @param {string} options.level - 'v', 'd', 'i', 'w', 'e'
   * @param {string} options.tag - Category tag
   * @param {Object|string} options.body - Extra JSON data
   */
  async push(text, { level = 'i', tag = 'JS', body = null } = {}) {
    try {
      const url = new URL(`${this.baseUrl}/api/log/push/${this.deviceId}`);
      url.searchParams.append('level', level);
      url.searchParams.append('tag', tag);

      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          text,
          body: typeof body === 'object' ? JSON.stringify(body) : body
        })
      });

      return response.ok;
    } catch (err) {
      console.error('[SonicLogger] Push failed:', err);
      return false;
    }
  }

  /**
   * Post a batch of logs
   * @param {Array<Object>} logs - Array of log objects
   */
  async sendBatch(logs) {
    try {
      const response = await fetch(`${this.baseUrl}/api/log/batch/${this.deviceId}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          device_id: this.deviceId,
          logs: logs.map(l => ({
            ...l,
            body: typeof l.body === 'object' ? JSON.stringify(l.body) : l.body
          }))
        })
      });

      return response.ok;
    } catch (err) {
      console.error('[SonicLogger] Batch failed:', err);
      return false;
    }
  }
}

// Example usage:
// const logger = new SonicLogger('http://localhost:58080', 'web-client-001');
// logger.push('User clicked login', { level: 'd', tag: 'UI', body: { x: 10, y: 20 } });
