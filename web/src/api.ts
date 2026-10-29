import type { AIInfo, AnalysisView, CreateOptions, MatchView } from './types';

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  });
  const text = await res.text();
  let body: unknown = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = null;
    }
  }
  if (!res.ok) {
    const msg =
      body && typeof body === 'object' && 'error' in body
        ? String((body as { error: unknown }).error)
        : `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return body as T;
}

export const api = {
  create: (opts?: CreateOptions) =>
    request<MatchView>('/api/matches', {
      method: 'POST',
      body: opts ? JSON.stringify(opts) : undefined,
    }),
  get: (id: string) => request<MatchView>(`/api/matches/${id}`),
  setConfig: (id: string, opts: CreateOptions) =>
    request<MatchView>(`/api/matches/${id}/config`, {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  play: (id: string, index: number) =>
    request<MatchView>(`/api/matches/${id}/actions`, {
      method: 'POST',
      body: JSON.stringify({ index }),
    }),
  ai: (id: string) => request<MatchView>(`/api/matches/${id}/ai`, { method: 'POST' }),
  analyze: (id: string) =>
    request<AnalysisView>(`/api/matches/${id}/analyze`, { method: 'POST' }),
  undo: (id: string) => request<MatchView>(`/api/matches/${id}/undo`, { method: 'POST' }),
  random: (id: string) => request<MatchView>(`/api/matches/${id}/random`, { method: 'POST' }),
  listAI: () => request<AIInfo[]>('/api/ai'),
  save: (id: string) =>
    request<{ name: string; path: string; json: string }>(`/api/matches/${id}/save`, {
      method: 'POST',
    }),
  load: (json: string) =>
    request<MatchView>('/api/saves/load', { method: 'POST', body: json }),
};
