/**
 * Sonic Stellar Logger - TypeScript Implementation
 * 
 * Provides type-safe logging for TS projects (React, Vue, Node.js).
 */

export type LogLevel = 'v' | 'd' | 'i' | 'w' | 'e';

export interface LogEntry {
  level?: LogLevel;
  tag?: string;
  text: string;
  body?: any;
  time?: string;
}

export class SonicLogger {
  private baseUrl: string;
  private deviceId: string;

  constructor(baseUrl: string, deviceId: string) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.deviceId = deviceId;
  }

  /**
   * Simplified Push API (Single Log)
   */
  async push(
    text: string, 
    options: { level?: LogLevel; tag?: string; body?: any } = {}
  ): Promise<boolean> {
    const { level = 'i', tag = 'TS', body = null } = options;
    
    try {
      const url = new URL(`${this.baseUrl}/api/log/push/${this.deviceId}`);
      url.searchParams.append('level', level);
      url.searchParams.append('tag', tag);

      const response = await fetch(url.toString(), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          text,
          body: body && typeof body === 'object' ? JSON.stringify(body) : body
        })
      });

      return response.ok;
    } catch (err) {
      console.error('[SonicLogger] Push failed:', err);
      return false;
    }
  }

  /**
   * Batch API (Multiple Logs)
   */
  async sendBatch(logs: LogEntry[]): Promise<boolean> {
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
            body: l.body && typeof l.body === 'object' ? JSON.stringify(l.body) : l.body
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
// const logger = new SonicLogger('http://localhost:58080', 'ts-worker-01');
// logger.push('Initializing system...', { level: 'i' });
