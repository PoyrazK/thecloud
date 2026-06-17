export interface CloudApiConfig {
  baseUrl: string;
  apiKey: string;
  tenantId: string;
}

export interface ApiEnvelope<T> {
  data?: T;
  error?: {
    message?: string;
    code?: string;
  };
  meta?: {
    request_id?: string;
    timestamp?: string;
  };
}

export class CloudApiError extends Error {
  status: number;
  code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = "CloudApiError";
    this.status = status;
    this.code = code;
  }
}

const STORAGE_KEY = "thecloud.console.api.v1";
const DEFAULT_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
const RESPONSE_CACHE_TTL_MS = 4000;
let sessionApiKey = "";

interface ResponseCacheEntry {
  timestamp: number;
  value: unknown;
}

const responseCache = new Map<string, ResponseCacheEntry>();

function apiKeyFingerprint(apiKey: string): string {
  if (!apiKey) return "no-key";
  let hash = 0;
  for (let index = 0; index < apiKey.length; index += 1) {
    hash = (hash * 31 + apiKey.charCodeAt(index)) >>> 0;
  }
  return hash.toString(16);
}

function cacheKeyForRequest(config: CloudApiConfig, path: string): string {
  return `${config.baseUrl}|${config.tenantId}|${apiKeyFingerprint(config.apiKey)}|${path}`;
}

export function clearApiResponseCache(): void {
  responseCache.clear();
}

function normalizeBaseUrl(url: string): string {
  return url.trim().replace(/\/+$/, "") || DEFAULT_BASE_URL;
}

export function getDefaultCloudApiConfig(): CloudApiConfig {
  return {
    baseUrl: normalizeBaseUrl(DEFAULT_BASE_URL),
    apiKey: "",
    tenantId: "",
  };
}

function safeParseConfig(raw: string | null): Partial<CloudApiConfig> {
  if (!raw) {
    return {};
  }

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return {};
    }
    return parsed as Partial<CloudApiConfig>;
  } catch {
    return {};
  }
}

function getSessionApiKey(): string {
  return sessionApiKey;
}

function setSessionApiKey(value: string): void {
  sessionApiKey = value.trim();
}

export function getStoredCloudApiConfig(): CloudApiConfig {
  if (typeof window === "undefined") {
    return getDefaultCloudApiConfig();
  }

  const parsed = safeParseConfig(window.localStorage.getItem(STORAGE_KEY));
  const storedApiKey = getSessionApiKey();

  return {
    baseUrl: normalizeBaseUrl(parsed.baseUrl ?? DEFAULT_BASE_URL),
    apiKey: storedApiKey,
    tenantId: (parsed.tenantId ?? "").trim(),
  };
}

export function saveStoredCloudApiConfig(update: Partial<CloudApiConfig>): CloudApiConfig {
  const current = getStoredCloudApiConfig();
  const nextApiKey = (update.apiKey ?? current.apiKey).trim();
  const next: CloudApiConfig = {
    ...current,
    ...update,
    baseUrl: normalizeBaseUrl(update.baseUrl ?? current.baseUrl),
    apiKey: nextApiKey,
    tenantId: (update.tenantId ?? current.tenantId).trim(),
  };

  setSessionApiKey(nextApiKey);

  if (typeof window !== "undefined") {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        baseUrl: next.baseUrl,
        tenantId: next.tenantId,
      })
    );
  }

  clearApiResponseCache();

  return next;
}

export function clearStoredCloudApiConfig(): void {
  setSessionApiKey("");

  if (typeof window !== "undefined") {
    window.localStorage.removeItem(STORAGE_KEY);
  }

  clearApiResponseCache();
}

function extractErrorMessage(payload: unknown, fallback: string): { message: string; code?: string } {
  if (!payload || typeof payload !== "object") {
    return { message: fallback };
  }

  const candidate = payload as {
    message?: string;
    error?: {
      message?: string;
      code?: string;
    };
  };

  if (candidate.error?.message) {
    return {
      message: candidate.error.message,
      code: candidate.error.code,
    };
  }

  if (candidate.message) {
    return { message: candidate.message };
  }

  return { message: fallback };
}

export async function cloudApiRequest<T>(
  path: string,
  init?: RequestInit,
  providedConfig?: CloudApiConfig
): Promise<T> {
  const config = providedConfig ?? getStoredCloudApiConfig();
  const method = (init?.method ?? "GET").toUpperCase();
  const isReadRequest = method === "GET" && !init?.body;
  const requestCacheKey = cacheKeyForRequest(config, path);

  if (isReadRequest) {
    const cached = responseCache.get(requestCacheKey);
    if (cached && Date.now() - cached.timestamp < RESPONSE_CACHE_TTL_MS) {
      return cached.value as T;
    }
  }

  if (!config.apiKey) {
    throw new CloudApiError("Missing API key. Configure access in Settings.", 401, "MISSING_API_KEY");
  }

  const headers = new Headers(init?.headers);
  headers.set("X-API-Key", config.apiKey);

  if (config.tenantId) {
    headers.set("X-Tenant-ID", config.tenantId);
  }

  if (init?.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${config.baseUrl}${path}`, {
    ...init,
    headers,
    cache: "no-store",
  });

  if (response.status === 204) {
    return null as T;
  }

  const contentType = response.headers.get("content-type") ?? "";
  const isJson = contentType.includes("application/json");
  const payload: unknown = isJson ? await response.json().catch(() => null) : await response.text().catch(() => "");

  if (!response.ok) {
    const { message, code } = extractErrorMessage(payload, `Request failed with status ${response.status}`);
    throw new CloudApiError(message, response.status, code);
  }

  let result: T;
  if (
    isJson &&
    payload &&
    typeof payload === "object" &&
    "data" in (payload as ApiEnvelope<T>) &&
    "meta" in (payload as ApiEnvelope<T>) &&
    typeof (payload as ApiEnvelope<T>).meta === "object" &&
    !!(payload as ApiEnvelope<T>).meta &&
    (
      Boolean((payload as ApiEnvelope<T>).meta?.request_id) ||
      Boolean((payload as ApiEnvelope<T>).meta?.timestamp)
    )
  ) {
    result = (payload as ApiEnvelope<T>).data as T;
  } else {
    result = payload as T;
  }

  if (isReadRequest) {
    responseCache.set(requestCacheKey, {
      timestamp: Date.now(),
      value: result,
    });
  } else {
    clearApiResponseCache();
  }

  return result;
}
