<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { getMonthlySummary, type MonthlySummary, ApiError } from "../services/api";
import { logout } from "../services/auth";
import { useRouter } from "vue-router";

const router = useRouter();

const month = ref<string>(new Date().toISOString().slice(0, 7)); // YYYY-MM
const loading = ref(false);
const error = ref<string | null>(null);
const data = ref<MonthlySummary | null>(null);

const categories = computed(() => {
  if (!data.value) return [];
  return Object.entries(data.value.byCategory).sort((a, b) => Math.abs(b[1]) - Math.abs(a[1]));
});

async function load() {
  loading.value = true;
  error.value = null;
  try {
    data.value = await getMonthlySummary(month.value);
  } catch (e) {
    if (e instanceof ApiError && e.kind === "UNAUTHORIZED") {
      logout();
      router.replace("/login");
      return;
    }
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <main style="padding: 24px; max-width: 900px">
    <header style="display: flex; justify-content: space-between; align-items: center">
      <h1 style="margin: 0">Monthly Summary</h1>
      <router-link to="/">← Back</router-link>
    </header>

    <section style="margin-top: 16px; display: flex; gap: 8px; align-items: end">
      <label>
        Month (YYYY-MM)
        <input v-model="month" type="month" />
      </label>
      <button @click="load" :disabled="loading">{{ loading ? "Loading..." : "Load" }}</button>
    </section>

    <p v-if="error" style="color: red; margin-top: 12px">{{ error }}</p>

    <section v-if="data" style="margin-top: 16px">
      <div style="display: flex; gap: 12px; flex-wrap: wrap">
        <div style="border: 1px solid #ddd; padding: 12px; border-radius: 8px; min-width: 180px">
          <div style="color: #666">Income</div>
          <div style="font-size: 20px">
            <b>{{ data.income.toFixed(2) }} {{ data.currency }}</b>
          </div>
        </div>

        <div style="border: 1px solid #ddd; padding: 12px; border-radius: 8px; min-width: 180px">
          <div style="color: #666">Expense</div>
          <div style="font-size: 20px">
            <b>{{ data.expense.toFixed(2) }} {{ data.currency }}</b>
          </div>
        </div>

        <div style="border: 1px solid #ddd; padding: 12px; border-radius: 8px; min-width: 180px">
          <div style="color: #666">Net</div>
          <div style="font-size: 20px">
            <b>{{ data.net.toFixed(2) }} {{ data.currency }}</b>
          </div>
        </div>

        <div style="border: 1px solid #ddd; padding: 12px; border-radius: 8px; min-width: 180px">
          <div style="color: #666">Count</div>
          <div style="font-size: 20px">
            <b>{{ data.count }}</b>
          </div>
        </div>
      </div>

      <h2 style="margin-top: 20px">By Category (net contribution)</h2>
      <p style="color: #666; margin-top: 6px">Positive = income, negative = expense.</p>

      <ul style="padding-left: 18px; margin-top: 8px">
        <li v-for="[cat, val] in categories" :key="cat" style="margin-bottom: 6px">
          <b>{{ cat }}</b
          >: {{ val.toFixed(2) }} {{ data.currency }}
        </li>
      </ul>

      <div v-if="data" style="margin-top: 16px">
        <h3>Category chart</h3>
        <div v-for="[cat, val] in categories" :key="cat" style="margin: 8px 0">
          <div style="display: flex; justify-content: space-between">
            <span
              ><b>{{ cat }}</b></span
            >
            <span>{{ val.toFixed(2) }} {{ data.currency }}</span>
          </div>
          <div style="height: 10px; background: #eee; border-radius: 6px; overflow: hidden">
            <div
              :style="{
                height: '10px',
                width:
                  Math.min(
                    100,
                    Math.round((Math.abs(val) / Math.max(1, Math.abs(data.net))) * 100),
                  ) + '%',
                background: val >= 0 ? '#4caf50' : '#f44336',
              }"
            ></div>
          </div>
        </div>
      </div>
    </section>
  </main>
</template>
