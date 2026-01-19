<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ApiError, listTransactions, createTransaction, type Transaction } from "../services/api";
import { logout, refreshIfNeeded } from "../services/auth";

const router = useRouter();

/** Transactions */
const txLoading = ref(false);
const txLoadingMore = ref(false);
const txError = ref<string | null>(null);
const txItems = ref<Transaction[]>([]);
const txNextToken = ref<string>(""); // empty => no more pages

/** Create form */
const formAmount = ref<number>(0);
const formCurrency = ref<string>("USD");
const formCategory = ref<string>("Sales");
const formNote = ref<string>("");

function handleAuthError(e: unknown): boolean {
  if (e instanceof ApiError && e.kind === "UNAUTHORIZED") {
    logout();
    router.replace("/login");
    return true;
  }
  return false;
}

async function loadTxFirstPage() {
  txLoading.value = true;
  txError.value = null;
  try {
    await refreshIfNeeded();

    const res = await listTransactions({ limit: 20 });
    txItems.value = res.items ?? [];
    txNextToken.value = res.nextToken ?? "";
  } catch (e) {
    if (handleAuthError(e)) return;
    txError.value = e instanceof Error ? e.message : String(e);
  } finally {
    txLoading.value = false;
  }
}

async function loadTxMore() {
  if (!txNextToken.value) return;

  txLoadingMore.value = true;
  txError.value = null;
  try {
    await refreshIfNeeded();

    const res = await listTransactions({ limit: 20, nextToken: txNextToken.value });
    txItems.value = [...txItems.value, ...(res.items ?? [])];
    txNextToken.value = res.nextToken ?? "";
  } catch (e) {
    if (handleAuthError(e)) return;
    txError.value = e instanceof Error ? e.message : String(e);
  } finally {
    txLoadingMore.value = false;
  }
}

async function submitTx() {
  txError.value = null;
  try {
    await refreshIfNeeded();

    const created = await createTransaction({
      amount: Number(formAmount.value),
      currency: formCurrency.value,
      category: formCategory.value,
      note: formNote.value,
    });

    // Put newest on top; keep pagination token unchanged
    txItems.value = [created, ...txItems.value];
    formAmount.value = 0;
    formNote.value = "";
  } catch (e) {
    if (handleAuthError(e)) return;
    txError.value = e instanceof Error ? e.message : String(e);
  }
}

function doLogout() {
  logout();
}

onMounted(async () => {
  await loadTxFirstPage();
});
</script>

<template>
  <main style="padding: 24px; max-width: 1000px">
    <header
      style="display: flex; gap: 12px; align-items: flex-start; justify-content: space-between"
    >
      <div>
        <h1 style="margin: 0">TrueProfit</h1>

        <div style="margin-top: 10px; display: flex; gap: 12px; flex-wrap: wrap">
          <router-link to="/summary">Monthly Summary</router-link>
          <router-link to="/shopify">Connected Shops</router-link>
          <router-link to="/ask">Ask (Text to SQL)</router-link>
        </div>
      </div>

      <button @click="doLogout">Logout</button>
    </header>

    <!-- Transactions -->
    <section style="margin-top: 24px">
      <div
        style="
          display: flex;
          justify-content: space-between;
          align-items: center;
          gap: 12px;
          flex-wrap: wrap;
        "
      >
        <h2 style="margin: 0">Transactions</h2>

        <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap">
          <button type="button" @click="loadTxFirstPage" :disabled="txLoading">
            {{ txLoading ? "Refreshing..." : "Refresh" }}
          </button>
        </div>
      </div>

      <form
        @submit.prevent="submitTx"
        style="margin-top: 12px; display: flex; gap: 8px; flex-wrap: wrap; align-items: end"
      >
        <label>
          Amount (income +, expense -)
          <input v-model.number="formAmount" type="number" step="0.01" required />
        </label>

        <label>
          Currency
          <input v-model="formCurrency" type="text" maxlength="3" required />
        </label>

        <label>
          Category
          <input v-model="formCategory" type="text" required />
        </label>

        <label style="flex: 1 1 250px">
          Note
          <input v-model="formNote" type="text" />
        </label>

        <button type="submit">Add</button>
      </form>

      <p v-if="txError" style="color: red; margin-top: 10px">{{ txError }}</p>

      <div style="margin-top: 12px">
        <p v-if="txLoading">Loading transactions...</p>
        <p v-else-if="txItems.length === 0">No transactions yet.</p>

        <ul v-else style="padding-left: 18px; margin-top: 8px">
          <li v-for="t in txItems" :key="t.id" style="margin-bottom: 10px">
            <b>{{ t.amount.toFixed(2) }} {{ t.currency }}</b>
            - {{ t.category }}
            <span style="color: #666">({{ new Date(t.createdAt).toLocaleString() }})</span>
            <div v-if="t.note" style="color: #444">{{ t.note }}</div>
          </li>
        </ul>

        <div style="margin-top: 12px; display: flex; gap: 8px; align-items: center">
          <button v-if="txNextToken" type="button" @click="loadTxMore" :disabled="txLoadingMore">
            {{ txLoadingMore ? "Loading..." : "Load more" }}
          </button>
          <span v-else style="color: #666">No more pages.</span>
        </div>
      </div>
    </section>
  </main>
</template>
