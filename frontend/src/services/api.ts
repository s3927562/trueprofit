import { config } from "./config";
import { getAccessToken, refreshIfNeeded } from "./auth";

type ApiErrorKind = "UNAUTHORIZED" | "FORBIDDEN" | "OTHER";

export class ApiError extends Error {
  kind: ApiErrorKind;
  status: number;
  constructor(kind: ApiErrorKind, status: number, message: string) {
    super(message);
    this.kind = kind;
    this.status = status;
  }
}

async function request(path: string): Promise<Response> {
  const base = config.apiBaseUrl().replace(/\/+$/, "");

  // Only refresh for protected endpoints
  const isProtected = path !== "/health";
  if (isProtected) {
    await refreshIfNeeded();
  }

  const token = getAccessToken();

  return fetch(`${base}${path}`, {
    method: "GET",
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
}

async function handle(res: Response): Promise<unknown> {
  if (res.ok) return res.json();

  const text = await res.text().catch(() => "");
  if (res.status === 401) {
    throw new ApiError(
      "UNAUTHORIZED",
      401,
      "You are not signed in, or your session expired. Please log in again.",
    );
  }
  if (res.status === 403) {
    throw new ApiError("FORBIDDEN", 403, "Signed in, but not allowed to access this resource.");
  }
  throw new ApiError(
    "OTHER",
    res.status,
    `Request failed (${res.status}): ${text || res.statusText}`,
  );
}

export type Transaction = {
  id: string; // SK
  amount: number;
  currency: string;
  category: string;
  note: string;
  createdAt: string;
};

export type TransactionListResponse = {
  items: Transaction[];
  nextToken: string;
};

export async function listTransactions(params?: {
  limit?: number;
  nextToken?: string;
}): Promise<TransactionListResponse> {
  const q = new URLSearchParams();
  if (params?.limit) q.set("limit", String(params.limit));
  if (params?.nextToken) q.set("nextToken", params.nextToken);

  const res = await request(`/transactions${q.toString() ? `?${q}` : ""}`);
  return (await handle(res)) as TransactionListResponse;
}

export async function createTransaction(input: {
  amount: number;
  currency: string;
  category: string;
  note: string;
}): Promise<Transaction> {
  const base = config.apiBaseUrl().replace(/\/+$/, "");
  await refreshIfNeeded();
  const token = getAccessToken();

  const res = await fetch(`${base}/transactions`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(input),
  });

  return (await handle(res)) as Transaction;
}

export type MonthlySummary = {
  month: string;
  currency: string;
  income: number;
  expense: number;
  net: number;
  byCategory: Record<string, number>;
  count: number;
};

export async function getMonthlySummary(month: string): Promise<MonthlySummary> {
  const res = await request(`/summary/monthly?month=${encodeURIComponent(month)}`);
  return (await handle(res)) as MonthlySummary;
}

export type ShopifyShop = {
  shop: string;
  scope: string;
  createdAt: string;
  lastEventAt: string;
  lastEventTopic: string;
  lastEventWebhookId: string;
};

export async function listShopifyShops(): Promise<ShopifyShop[]> {
  const res = await request("/integrations/shopify/shops");
  const data = (await handle(res)) as { items: ShopifyShop[] };
  return data.items ?? [];
}

export async function disconnectShopifyShop(shop: string): Promise<void> {
  const base = config.apiBaseUrl().replace(/\/+$/, "");
  await refreshIfNeeded();
  const token = getAccessToken();

  const res = await fetch(`${base}/integrations/shopify/shops?shop=${encodeURIComponent(shop)}`, {
    method: "DELETE",
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });

  await handle(res);
}

export async function syncShopifyShop(
  shop: string,
): Promise<{ ok: boolean; shop: string; note?: string }> {
  const base = config.apiBaseUrl().replace(/\/+$/, "");
  await refreshIfNeeded();
  const token = getAccessToken();

  const res = await fetch(`${base}/integrations/shopify/sync?shop=${encodeURIComponent(shop)}`, {
    method: "POST",
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });

  return (await handle(res)) as { ok: boolean; shop: string; note?: string };
}

export async function getShopifyAuthorizeUrl(shop: string): Promise<string> {
  const res = await request(`/integrations/shopify/connect?shop=${encodeURIComponent(shop)}`);
  const data = (await handle(res)) as { authorizeUrl: string };
  if (!data.authorizeUrl) throw new Error("Missing authorizeUrl from backend");
  return data.authorizeUrl;
}

export type AskNLQRequest = {
  question: string;
  shop_ids?: string[];
};

export type AskNLQResponse = unknown;

export async function askNLQ(input: AskNLQRequest): Promise<AskNLQResponse> {
  const base = config.apiBaseUrl().replace(/\/+$/, "");
  await refreshIfNeeded();
  const token = getAccessToken();

  const res = await fetch(`${base}/ask`, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(input),
  });

  return (await handle(res)) as AskNLQResponse;
}
