<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { handleCallback } from "../services/auth";

const router = useRouter();
const loading = ref(true);
const error = ref<string | null>(null);

onMounted(async () => {
  try {
    await handleCallback(window.location.search);
    router.replace("/");
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <main style="padding: 24px">
    <h1>Callback</h1>
    <p v-if="loading">Signing you in...</p>
    <p v-else-if="error" style="color: red">{{ error }}</p>
    <p v-else>Done. Redirecting...</p>
  </main>
</template>
