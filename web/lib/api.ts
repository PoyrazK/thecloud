const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
const API_KEY_STORAGE = 'thecloud_api_key';

type ApiError = {
  message?: string;
};

type ApiResponse<T> = {
  data: T;
  error?: ApiError;
};

export function getApiUrl(): string {
  return API_URL;
}

export function getStoredApiKey(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  return window.localStorage.getItem(API_KEY_STORAGE) || '';
}

export function setStoredApiKey(key: string): void {
  if (typeof window === 'undefined') {
    return;
  }
  if (key) {
    window.localStorage.setItem(API_KEY_STORAGE, key);
  } else {
    window.localStorage.removeItem(API_KEY_STORAGE);
  }
}

export async function apiGet<T>(path: string): Promise<T> {
  const apiKey = getStoredApiKey();
  if (!apiKey) {
    throw new Error('API key required');
  }

  const response = await fetch(API_URL + path, {
    headers: {
      'X-API-Key': apiKey,
    },
  });

  const body = (await response.json().catch(() => ({}))) as ApiResponse<T>;
  if (!response.ok) {
    throw new Error(body.error?.message || response.statusText);
  }

  return body.data;
}
