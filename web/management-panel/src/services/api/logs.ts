/**
 * Logs API
 */

import { apiClient } from './client';
import { LOGS_TIMEOUT_MS } from '@/utils/constants';

export interface LogsQuery {
  after?: number;
}

export interface LogsResponse {
  lines: string[];
  'line-count': number;
  'latest-timestamp': number;
}

export interface ErrorLogFile {
  name: string;
  size?: number;
  modified?: number;
}

export interface ErrorLogsResponse {
  files?: ErrorLogFile[];
}

export type LogDataTarget = 'application' | 'request' | 'error-request' | 'temporary' | 'all';

export interface LogStorageBucket {
  size: number;
  files: number;
}

export interface LogStorageResponse {
  'log-directory': string;
  'total-size': number;
  'total-files': number;
  application: LogStorageBucket;
  request: LogStorageBucket;
  'error-request': LogStorageBucket;
  temporary: LogStorageBucket;
}

export const logsApi = {
  fetchLogs: (params: LogsQuery = {}): Promise<LogsResponse> =>
    apiClient.get('/logs', { params, timeout: LOGS_TIMEOUT_MS }),

  fetchStorage: (): Promise<LogStorageResponse> =>
    apiClient.get('/logs/storage', { timeout: LOGS_TIMEOUT_MS }),

  clearLogs: (target: LogDataTarget = 'application') =>
    apiClient.delete('/logs', { params: { target } }),

  fetchErrorLogs: (): Promise<ErrorLogsResponse> =>
    apiClient.get('/request-error-logs', { timeout: LOGS_TIMEOUT_MS }),

  downloadErrorLog: (filename: string) =>
    apiClient.getRaw(`/request-error-logs/${encodeURIComponent(filename)}`, {
      responseType: 'blob',
      timeout: LOGS_TIMEOUT_MS,
    }),

  downloadRequestLogById: (id: string) =>
    apiClient.getRaw(`/request-log-by-id/${encodeURIComponent(id)}`, {
      responseType: 'blob',
      timeout: LOGS_TIMEOUT_MS,
    }),
};
