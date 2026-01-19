<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import {
  listShopifyShops,
  disconnectShopifyShop,
  syncShopifyShop,
  type ShopifyShop,
  ApiError,
  getShopifyAuthorizeUrl,
} from "../services/api";
import { logout } from "../services/auth";

const router = useRouter();

const shopInput = ref<string>("");
const loading = ref(false);
const error = ref<string | null>(null);
const items = ref<ShopifyShop[]>([]);
const msg = ref<string | null>(null);

function handleAuthError(e: unknown): boolean {
  if (e instanceof ApiError && e.kind === "UNAUTHORIZED") {
    logout();
    router.replace("/login");
    return true;
  }
  return false;
}

async function refreshList() {
  loading.value = true;
  error.value = null;
  msg.value = null;

  try {
    items.value = await listShopifyShops();
  } catch (e) {
    if (handleAuthError(e)) return;
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

function normalizeShop(s: string): string {
  return s.trim().toLowerCase();
}

async function connectShop() {
  msg.value = null;
  error.value = null;

  const shop = normalizeShop(shopInput.value);
  if (!shop.endsWith(".myshopify.com")) {
    error.value = "Shop must look like: your-store.myshopify.com";
    return;
  }

  try {
    const authorizeUrl = await getShopifyAuthorizeUrl(shop);
    window.location.assign(authorizeUrl); // now redirect to Shopify
  } catch (e) {
    if (handleAuthError(e)) return;
    error.value = e instanceof Error ? e.message : String(e);
  }
}

async function disconnect(shop: string) {
  msg.value = null;
  error.value = null;
  try {
    await disconnectShopifyShop(shop);
    msg.value = `Disconnected: ${shop}`;
    await refreshList();
  } catch (e) {
    if (handleAuthError(e)) return;
    error.value = e instanceof Error ? e.message : String(e);
  }
}

async function sync(shop: string) {
  msg.value = null;
  error.value = null;
  try {
    const res = await syncShopifyShop(shop);
    msg.value = `Sync started for ${res.shop}. ${res.note ?? ""}`.trim();
  } catch (e) {
    if (handleAuthError(e)) return;
    error.value = e instanceof Error ? e.message : String(e);
  }
}

onMounted(async () => {
  // show callback message if present
  const p = new URLSearchParams(window.location.search);
  if (p.get("connected") === "1") {
    const s = p.get("shop");
    msg.value = s ? `Connected: ${s}` : "Shop connected!";
    // optional: clean URL
    router.replace({ path: "/shopify" });
  }

  await refreshList();
});
</script>

<template>
  <main style="padding: 24px; max-width: 900px">
    <header style="display: flex; justify-content: space-between; align-items: center">
      <h1 style="margin: 0">Shopify Integrations</h1>
      <router-link to="/">← Back</router-link>
    </header>

    <section style="margin-top: 16px">
      <h2>Connect a new shop</h2>
      <p style="color: #666; margin-top: 6px">
        You can connect multiple Shopify stores to this account.
      </p>

      <div style="display: flex; gap: 8px; flex-wrap: wrap; align-items: end">
        <label style="flex: 1 1 320px">
          Shop domain
          <input v-model="shopInput" placeholder="your-store.myshopify.com" />
        </label>
        <button @click="connectShop">Connect</button>
      </div>
    </section>

    <p v-if="msg" style="margin-top: 12px; color: #0a6">{{ msg }}</p>
    <p v-if="error" style="margin-top: 12px; color: red">{{ error }}</p>

    <section style="margin-top: 20px">
      <h2>Connected shops</h2>

      <p v-if="loading">Loading...</p>
      <p v-else-if="items.length === 0">No shops connected yet.</p>

      <div v-else style="display: flex; flex-direction: column; gap: 10px; margin-top: 10px">
        <div
          v-for="s in items"
          :key="s.shop"
          style="
            border: 1px solid #ddd;
            padding: 12px;
            border-radius: 10px;
            display: flex;
            justify-content: space-between;
            gap: 12px;
            flex-wrap: wrap;
          "
        >
          <div>
            <div style="font-size: 16px">
              <b>{{ s.shop }}</b>
            </div>
            <div style="color: #666">scope: {{ s.scope || "-" }}</div>
            <div style="color: #666">
              connected: {{ s.createdAt ? new Date(s.createdAt).toLocaleString() : "-" }}
            </div>
          </div>

          <div style="display: flex; gap: 8px; align-items: center">
            <button @click="sync(s.shop)">Sync</button>
            <button @click="disconnect(s.shop)">Disconnect</button>
          </div>
        </div>
        <button style="margin-top: 12px" @click="refreshList" :disabled="loading">
          {{ loading ? "Refreshing..." : "Refresh list" }}
        </button>
      </div>
    </section>
  </main>
</template>
