<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import {
  askNLQ,
  listShopifyShops,
  ApiError,
  type ShopifyShop,
  type AskResponse,
} from "@/services/api";
import { logout, refreshIfNeeded } from "@/services/auth";

/* ---------------- state ---------------- */

const router = useRouter();

const question = ref("");
const loading = ref(false);
const error = ref<string | null>(null);
const resp = ref<AskResponse | null>(null);

const showSQL = ref(false);
const showAssumptions = ref(true);

/* -------- shop scope (optional) -------- */

const shopsLoading = ref(false);
const shopsError = ref<string | null>(null);
const shops = ref<ShopifyShop[]>([]);
const scopeMode = ref<"all" | "selected">("all");
const selectedShops = ref<string[]>([]);

/* ---------------- helpers ---------------- */

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
    selectedShops.value = shops.value.map((s) => s.shop);
  } catch (e) {
    if (handleAuthError(e)) return;
    shopsError.value = e instanceof Error ? e.message : String(e);
  } finally {
    shopsLoading.value = false;
  }
}

const shopOptions = computed(() => shops.value.map((s) => s.shop));

const effectiveShopIDs = computed<string[]>(() => {
  if (scopeMode.value === "all") return [];
  return selectedShops.value.filter((s) => s.trim().length > 0);
});

/* ---------------- actions ---------------- */

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

/* ---------------- computed (typed) ---------------- */

const columns = computed<string[]>(() => {
  if (!resp.value || resp.value.type !== "result") return [];
  if (resp.value.result?.columns) return resp.value.result.columns;
  return resp.value.columns ?? [];
});

const rows = computed<Array<Record<string, unknown>>>(() => {
  if (!resp.value || resp.value.type !== "result") return [];
  if (resp.value.result?.rows) return resp.value.result.rows;
  return resp.value.rows ?? [];
});

const isScalar = computed<boolean>(() => {
  return resp.value?.type === "result" && resp.value.result?.kind === "scalar";
});

const scalarLabel = computed<string>(() => {
  if (resp.value?.type !== "result") return "value";
  return resp.value.result?.columns?.[0] ?? "value";
});

const scalarValue = computed<unknown>(() => {
  if (resp.value?.type !== "result") return null;
  return resp.value.result?.value ?? null;
});

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
  question.value ||= "What is my net revenue in the last 30 days?";
});
</script>

<template>
  <main style="padding: 24px; max-width: 1000px">
    <!-- Header -->
    <header style="display: flex; justify-content: space-between; gap: 12px">
      <div>
        <h1 style="margin: 0">Ask (Text to SQL)</h1>
        <p style="color: #666">
          Ask questions in plain English. We safely generate SQL and query Athena.
        </p>
      </div>
      <button @click="doLogout">Logout</button>
    </header>

    <!-- Scope -->
    <section style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px">
      <h2 style="margin: 0">Scope</h2>

      <p v-if="shopsError" style="color: red">{{ shopsError }}</p>

      <label>
        <input type="radio" value="all" v-model="scopeMode" />
        All connected shops
      </label>

      <label style="margin-left: 16px">
        <input type="radio" value="selected" v-model="scopeMode" />
        Select shops
      </label>

      <div v-if="scopeMode === 'selected'" style="margin-top: 10px">
        <select multiple v-model="selectedShops" style="width: 100%; min-height: 120px">
          <option v-for="s in shopOptions" :key="s" :value="s">{{ s }}</option>
        </select>
      </div>
    </section>

    <!-- Ask -->
    <section style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px">
      <label>
        Question
        <textarea v-model="question" rows="4" style="width: 100%" />
      </label>

      <div style="margin-top: 10px; display: flex; gap: 12px">
        <button @click="runAsk" :disabled="loading">
          {{ loading ? "Running..." : "Ask" }}
        </button>

        <label><input type="checkbox" v-model="showSQL" /> Show SQL</label>
        <label><input type="checkbox" v-model="showAssumptions" /> Show assumptions</label>
      </div>

      <p v-if="error" style="color: red">{{ error }}</p>
    </section>

    <!-- Clarification -->
    <section
      v-if="resp && resp.type === 'clarification'"
      style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px"
    >
      <h2>Need clarification</h2>
      <p>{{ resp.clarifying_question }}</p>

      <ul v-if="showAssumptions && resp.assumptions">
        <li v-for="(a, i) in resp.assumptions" :key="i">{{ a }}</li>
      </ul>
    </section>

    <!-- No shops -->
    <section
      v-else-if="resp && resp.type === 'no_shops'"
      style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px"
    >
      <h2>No shops connected</h2>
      <p>Please connect a Shopify shop first.</p>
    </section>

    <!-- SQL rejected / Athena failed -->
    <section
      v-else-if="resp && (resp.type === 'sql_rejected' || resp.type === 'athena_failed')"
      style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px"
    >
      <h2>{{ resp.type === "sql_rejected" ? "Query rejected" : "Query failed" }}</h2>

      <p style="color: red">
        {{ resp.type === "sql_rejected" ? resp.reason : resp.error }}
      </p>

      <pre v-if="showSQL"
        >{{ resp.type === "sql_rejected" ? resp.model_sql : resp.last_sql }}
      </pre>

      <ul v-if="showAssumptions && resp.assumptions">
        <li v-for="(a, i) in resp.assumptions" :key="i">{{ a }}</li>
      </ul>
    </section>

    <!-- Result -->
    <section
      v-else-if="resp && resp.type === 'result'"
      style="margin-top: 16px; border: 1px solid #ddd; border-radius: 12px; padding: 14px"
    >
      <h2>Result</h2>

      <p style="color: #666">
        <span v-if="resp.cached">cached • </span>
        scanned {{ formatBytes(resp.scanned_bytes || 0) }} • {{ resp.exec_ms }} ms
      </p>

      <pre v-if="showSQL">{{ resp.sql }}</pre>

      <ul v-if="showAssumptions && resp.assumptions">
        <li v-for="(a, i) in resp.assumptions" :key="i">{{ a }}</li>
      </ul>

      <!-- Scalar -->
      <div v-if="isScalar">
        <strong>{{ scalarLabel }}</strong>
        <div style="font-size: 28px">{{ scalarValue }}</div>
      </div>

      <!-- Table -->
      <table v-else>
        <thead>
          <tr>
            <th v-for="c in columns" :key="c">{{ c }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, i) in rows" :key="i">
            <td v-for="c in columns" :key="c">{{ r[c] }}</td>
          </tr>
        </tbody>
      </table>
    </section>
  </main>
</template>
