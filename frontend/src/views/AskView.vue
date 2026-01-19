<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import {
  ApiError,
  askNLQ,
  listShopifyShops,
  type ShopifyShop,
} from "../services/api";
import { logout, refreshIfNeeded } from "../services/auth";

type AskResponse = unknown;

const router = useRouter();

const loading = ref(false);
const error = ref<string | null>(null);

const question = ref<string>("");
const resp = ref<AskResponse | null>(null);

const showSQL = ref(false);
const showAssumptions = ref(true);

// Shop scope
const shopsLoading = ref(false);
const shopsError = ref<string | null>(null);
const shops = ref<ShopifyShop[]>([]);
const scopeMode = ref<"all" | "selected">("all");
const selectedShops = ref<string[]>([]);

function handleAuthError(e: unknown): boolean {
  if (e instanceof ApiError && e.kind === "UNAUTHORIZED") {
    logout();
    router.replace("/login");
    return true;
  }
  return false;
}

async function loadShops() {
  shopsLoading.value = true;
  shopsError.value = null;
  try {
    await refreshIfNeeded();
    const items = await listShopifyShops();
    shops.value = items ?? [];

    // Default selection: all shops
    selectedShops.value = shops.value.map((s) => s.shop);
  } catch (e) {
    if (handleAuthError(e)) return;
    shopsError.value = e instanceof Error ? e.message : String(e);
  } finally {
    shopsLoading.value = false;
  }
}

const shopOptions = computed(() => shops.value.map((s) => s.shop));

const effectiveShopIDs = computed(() => {
  if (scopeMode.value === "all") return [];
  // selected subset
  return selectedShops.value.filter((s) => s && s.trim().length > 0);
});

async function runAsk() {
  error.value = null;
  resp.value = null;

  const q = question.value.trim();
  if (!q) {
    error.value = "Please enter a question.";
    return;
  }

  loading.value = true;
  try {
    resp.value = await askNLQ({
      question: q,
      ...(effectiveShopIDs.value.length ? { shop_ids: effectiveShopIDs.value } : {}),
    });
  } catch (e) {
    if (handleAuthError(e)) return;
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

function doLogout() {
  logout();
}

const columns = computed<string[]>(() => {
  if (!resp.value) return [];
  if (resp.value.result?.columns) return resp.value.result.columns;
  return resp.value.columns ?? [];
});

const rows = computed<Array<Record<string, unknown>>>(() => {
  if (!resp.value) return [];
  if (resp.value.result?.rows) return resp.value.result.rows;
  return resp.value.rows ?? [];
});

const isScalar = computed<boolean>(() => {
  return resp.value?.type === "result" && resp.value?.result?.kind === "scalar";
});

const scalarLabel = computed<string>(() => {
  const c = resp.value?.result?.columns?.[0];
  return c || "value";
});

const scalarValue = computed<unknown>(() => resp.value?.result?.value);

function formatBytes(n: number) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

onMounted(async () => {
  await loadShops();
  if (!question.value) {
    question.value = "What is my net revenue in the last 30 days?";
  }
});
</script>

<template>
  <main style="padding: 24px; max-width: 1000px">
    <header style="display: flex; gap: 12px; align-items: flex-start; justify-content: space-between">
      <div>
        <h1 style="margin: 0">TrueProfit</h1>

        <div style="margin-top: 10px; display: flex; gap: 12px; flex-wrap: wrap">
          <router-link to="/">Transactions</router-link>
          <router-link to="/summary">Monthly Summary</router-link>
          <router-link to="/shopify">Connected Shops</router-link>
          <router-link to="/show">Ask (Text to SQL)</router-link>
        </div>

        <p style="color: #666; margin: 10px 0 0">
          Ask questions in plain English. We generate SQL, validate tenant access, and query Athena.
        </p>
      </div>

      <button @click="doLogout">Logout</button>
    </header>

    <!-- Shop scope -->
    <section style="margin-top: 18px; border: 1px solid #ddd; border-radius: 12px; padding: 14px">
      <div style="display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap">
        <h2 style="margin: 0">Scope</h2>
        <button type="button" @click="loadShops" :disabled="shopsLoading">
          {{ shopsLoading ? "Refreshing..." : "Refresh shops" }}
        </button>
      </div>

      <p v-if="shopsError" style="color: red; margin-top: 10px">{{ shopsError }}</p>

      <div style="margin-top: 10px; display: flex; gap: 14px; flex-wrap: wrap; align-items: center">
        <label style="display: flex; gap: 8px; align-items: center">
          <input type="radio" value="all" v-model="scopeMode" />
          All connected shops
        </label>

        <label style="display: flex; gap: 8px; align-items: center">
          <input type="radio" value="selected" v-model="scopeMode" />
          Select shops
        </label>
      </div>

      <div v-if="scopeMode === 'selected'" style="margin-top: 12px">
        <p style="margin: 0 0 8px; color: #666">
          Hold Ctrl/⌘ to select multiple shops.
        </p>
        <select
          multiple
          v-model="selectedShops"
          :disabled="shopsLoading || shopOptions.length === 0"
          style="width: 100%; min-height: 120px; padding: 10px; border-radius: 10px; border: 1px solid #d1d5db"
        >
          <option v-for="s in shopOptions" :key="s" :value="s">{{ s }}</option>
        </select>

        <p v-if="shopOptions.length === 0" style="color: #666; margin-top: 8px">
          No shops found. Connect a shop first on the Connected Shops page.
        </p>
      </div>
    </section>

    <!-- Ask -->
    <section style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px">
      <label style="display: block; font-weight: 600">
        Question
        <textarea
          v-model="question"
          rows="4"
          style="width: 100%; margin-top: 8px; padding: 10px 12px; border-radius: 10px; border: 1px solid #d1d5db"
          placeholder="e.g., Show marketing costs by day for the last 14 days."
        />
      </label>

      <div style="margin-top: 12px; display: flex; gap: 14px; flex-wrap: wrap; align-items: center">
        <button @click="runAsk" :disabled="loading || !question.trim()">
          {{ loading ? "Running..." : "Ask" }}
        </button>

        <label style="display: flex; gap: 8px; align-items: center">
          <input type="checkbox" v-model="showSQL" />
          Show SQL
        </label>

        <label style="display: flex; gap: 8px; align-items: center">
          <input type="checkbox" v-model="showAssumptions" />
          Show assumptions
        </label>
      </div>

      <p v-if="error" style="color: red; margin-top: 12px">{{ error }}</p>
    </section>

    <!-- Clarification -->
    <section
      v-if="resp && resp.type === 'clarification'"
      style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px"
    >
      <h2 style="margin: 0">Need clarification</h2>
      <p style="margin-top: 8px">
        {{ resp.clarifying_question || "Please clarify your request." }}
      </p>

      <div v-if="showAssumptions && resp.assumptions?.length" style="margin-top: 10px; color: #666">
        <h3 style="margin: 0 0 8px">Assumptions</h3>
        <ul style="padding-left: 18px; margin: 0">
          <li v-for="(a, i) in resp.assumptions" :key="i">{{ a }}</li>
        </ul>
      </div>
    </section>

    <!-- No shops -->
    <section
      v-else-if="resp && resp.type === 'no_shops'"
      style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px"
    >
      <h2 style="margin: 0">No shops connected</h2>
      <p style="margin-top: 8px">
        Connect a Shopify shop first, then try again.
        <router-link to="/shopify">Go to Connected Shops</router-link>
      </p>
    </section>

    <!-- SQL rejected / Athena failed -->
    <section
      v-else-if="resp && (resp.type === 'sql_rejected' || resp.type === 'athena_failed')"
      style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px"
    >
      <h2 style="margin: 0">
        {{ resp.type === "sql_rejected" ? "Query rejected" : "Query failed" }}
      </h2>

      <p style="margin-top: 8px; color: red">
        {{ resp.type === "sql_rejected" ? resp.reason : resp.error }}
      </p>

      <div v-if="showSQL" style="margin-top: 10px">
        <h3 style="margin: 0 0 8px">SQL</h3>
        <pre style="background: #0b1220; color: #e5e7eb; border-radius: 10px; padding: 12px; overflow: auto">{{
          (resp as unknown).model_sql || (resp as unknown).last_sql
        }}</pre>
      </div>

      <div v-if="showAssumptions && (resp as unknown).assumptions?.length" style="margin-top: 10px; color: #666">
        <h3 style="margin: 0 0 8px">Assumptions</h3>
        <ul style="padding-left: 18px; margin: 0">
          <li v-for="(a, i) in (resp as unknown).assumptions" :key="i">{{ a }}</li>
        </ul>
      </div>
    </section>

    <!-- Result -->
    <section
      v-else-if="resp && resp.type === 'result'"
      style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px"
    >
      <div style="display: flex; justify-content: space-between; gap: 12px; flex-wrap: wrap">
        <h2 style="margin: 0">Result</h2>
        <div style="color: #666">
          <span v-if="resp.cached">cached • </span>
          scanned {{ formatBytes(resp.scanned_bytes || 0) }} • {{ resp.exec_ms || 0 }} ms
        </div>
      </div>

      <div v-if="showAssumptions && resp.assumptions?.length" style="margin-top: 10px; color: #666">
        <h3 style="margin: 0 0 8px">Assumptions</h3>
        <ul style="padding-left: 18px; margin: 0">
          <li v-for="(a, i) in resp.assumptions" :key="i">{{ a }}</li>
        </ul>
      </div>

      <div v-if="showSQL" style="margin-top: 10px">
        <h3 style="margin: 0 0 8px">SQL</h3>
        <pre style="background: #0b1220; color: #e5e7eb; border-radius: 10px; padding: 12px; overflow: auto">{{
          resp.sql
        }}</pre>
      </div>

      <!-- Scalar -->
      <div v-if="isScalar" style="margin-top: 12px; border: 1px solid #eee; border-radius: 12px; padding: 14px">
        <div style="color: #666; font-size: 12px">{{ scalarLabel }}</div>
        <div style="font-size: 28px; font-weight: 700; margin-top: 4px">{{ scalarValue }}</div>
      </div>

      <!-- Table -->
      <div v-else style="margin-top: 12px">
        <table v-if="columns.length" style="width: 100%; border-collapse: collapse">
          <thead>
            <tr>
              <th
                v-for="c in columns"
                :key="c"
                style="border: 1px solid #e5e7eb; padding: 8px 10px; text-align: left; background: #fafafa"
              >
                {{ c }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in rows" :key="i">
              <td
                v-for="c in columns"
                :key="c"
                style="border: 1px solid #e5e7eb; padding: 8px 10px; vertical-align: top"
              >
                {{ r[c] }}
              </td>
            </tr>
          </tbody>
        </table>

        <p v-else style="color: #666">No rows returned.</p>
      </div>
    </section>

    <!-- Default -->
    <section v-else style="margin-top: 16px; color: #666">
      <p>Ask a question to query your analytics data.</p>
      <ul style="padding-left: 18px">
        <li>What is my gross revenue in the last 7 days?</li>
        <li>Show marketing costs by day for the last 14 days.</li>
        <li>Which shop has the highest net revenue this month?</li>
      </ul>
    </section>
  </main>
</template>
