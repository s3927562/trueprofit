import { config } from "./config";

type TokenResponse = {
  access_token: string;
  id_token?: string;
  refresh_token?: string;
  expires_in: number;
  token_type: string;
};

const STORAGE_KEY = "tp_auth";

type StoredAuth = {
  accessToken: string;
  idToken: string;
  refreshToken?: string;
  expiresAt: number; // epoch ms
};

function readStored(): StoredAuth | null {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as StoredAuth;
  } catch {
    return null;
  }
}

function writeStored(next: StoredAuth) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
}

function base64UrlEncode(bytes: ArrayBuffer): string {
  const bin = String.fromCharCode(...new Uint8Array(bytes));
  const b64 = btoa(bin);
  return b64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

async function sha256(input: string): Promise<ArrayBuffer> {
  return await crypto.subtle.digest("SHA-256", new TextEncoder().encode(input));
}

function randomString(len = 64): string {
  const bytes = new Uint8Array(len);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => ("0" + b.toString(16)).slice(-2)).join("");
}

function savePkce(verifier: string, state: string) {
  sessionStorage.setItem("tp_pkce_verifier", verifier);
  sessionStorage.setItem("tp_oauth_state", state);
}

function loadPkce(): { verifier: string; state: string } {
  const verifier = sessionStorage.getItem("tp_pkce_verifier") || "";
  const state = sessionStorage.getItem("tp_oauth_state") || "";
  if (!verifier || !state) throw new Error("Missing PKCE verifier/state (try logging in again).");
  return { verifier, state };
}

function clearPkce() {
  sessionStorage.removeItem("tp_pkce_verifier");
  sessionStorage.removeItem("tp_oauth_state");
}

export function isAuthed(): boolean {
  const data = readStored();
  if (!data) return false;
  return Date.now() < data.expiresAt && !!data.idToken;
}

export function getAccessToken(): string | null {
  const data = readStored();
  if (!data) return null;
  if (Date.now() >= data.expiresAt) return null;
  return data.idToken;
}

export function startLogin(): void {
  const verifier = randomString(64);
  const state = randomString(16);

  // PKCE challenge = BASE64URL(SHA256(verifier))
  sha256(verifier).then((digest) => {
    const challenge = base64UrlEncode(digest);

    savePkce(verifier, state);

    const domain = config.cognitoDomain().replace(/\/+$/, "");
    const clientId = config.cognitoClientId();
    const redirectUri = encodeURIComponent(config.redirectUri());

    const url =
      `${domain}/oauth2/authorize?` +
      `response_type=code&` +
      `client_id=${encodeURIComponent(clientId)}&` +
      `redirect_uri=${redirectUri}&` +
      `scope=${encodeURIComponent("openid email profile")}&` +
      `state=${encodeURIComponent(state)}&` +
      `code_challenge=${encodeURIComponent(challenge)}&` +
      `code_challenge_method=S256`;

    window.location.assign(url);
  });
}

export async function handleCallback(search: string): Promise<void> {
  const params = new URLSearchParams(search);
  const code = params.get("code");
  const returnedState = params.get("state");
  const err = params.get("error");
  const errDesc = params.get("error_description");

  if (err) throw new Error(`OAuth error: ${err}${errDesc ? ` - ${errDesc}` : ""}`);
  if (!code) throw new Error("Missing authorization code.");

  const { verifier, state } = loadPkce();
  if (!returnedState || returnedState !== state) throw new Error("State mismatch.");
  clearPkce();

  const domain = config.cognitoDomain().replace(/\/+$/, "");
  const tokenUrl = `${domain}/oauth2/token`;

  const body = new URLSearchParams();
  body.set("grant_type", "authorization_code");
  body.set("client_id", config.cognitoClientId());
  body.set("code", code);
  body.set("redirect_uri", config.redirectUri());
  body.set("code_verifier", verifier);

  const res = await fetch(tokenUrl, {
    method: "POST",
    headers: { "content-type": "application/x-www-form-urlencoded" },
    body: body.toString(),
  });

  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`Token exchange failed (${res.status}): ${text}`);
  }

  const tok = (await res.json()) as TokenResponse;

  const prev = readStored();
  const expiresAt = Date.now() + tok.expires_in * 1000 - 10_000;

  const stored: StoredAuth = {
    accessToken: tok.access_token,
    idToken: tok.id_token ?? prev?.idToken ?? "",
    refreshToken: tok.refresh_token ?? prev?.refreshToken,
    expiresAt,
  };

  writeStored(stored);
}

let refreshPromise: Promise<void> | null = null;

export async function refreshIfNeeded(): Promise<void> {
  const data = readStored();
  if (!data) return;

  const now = Date.now();
  const refreshSkewMs = 30_000; // refresh if expiring within 30s
  const needsRefresh = now >= data.expiresAt - refreshSkewMs;

  if (!needsRefresh) return;
  if (!data.refreshToken) return; // can't refresh; user will need to login again

  // Deduplicate concurrent refresh calls
  if (refreshPromise) return refreshPromise;

  refreshPromise = (async () => {
    const domain = config.cognitoDomain().replace(/\/+$/, "");
    const tokenUrl = `${domain}/oauth2/token`;

    const body = new URLSearchParams();
    body.set("grant_type", "refresh_token");
    body.set("client_id", config.cognitoClientId());
    body.set("refresh_token", data.refreshToken!);

    const res = await fetch(tokenUrl, {
      method: "POST",
      headers: { "content-type": "application/x-www-form-urlencoded" },
      body: body.toString(),
    });

    if (!res.ok) {
      // Refresh token may be revoked/expired -> force logout on next step
      throw new Error(`Refresh failed (${res.status})`);
    }

    const tok = (await res.json()) as TokenResponse;

    const expiresAt = Date.now() + tok.expires_in * 1000 - 10_000;

    // Refresh responses often return a new access_token; id_token may or may not be present.
    const next: StoredAuth = {
      accessToken: tok.access_token,
      idToken: tok.id_token ?? data.idToken,
      refreshToken: tok.refresh_token ?? data.refreshToken,
      expiresAt,
    };

    writeStored(next);
  })().finally(() => {
    refreshPromise = null;
  });

  return refreshPromise;
}

export function logout(): void {
  localStorage.removeItem(STORAGE_KEY);

  const domain = config.cognitoDomain().replace(/\/+$/, "");
  const clientId = config.cognitoClientId();
  const logoutUri = encodeURIComponent(config.logoutUri());

  // Cognito hosted UI logout endpoint
  const url = `${domain}/logout?client_id=${encodeURIComponent(clientId)}&logout_uri=${logoutUri}`;
  window.location.assign(url);
}
