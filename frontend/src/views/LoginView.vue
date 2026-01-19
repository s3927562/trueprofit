<script setup lang="ts">
import { startLogin, isAuthed, logout } from "../services/auth";

const onLogin = () => startLogin();
const onLogout = () => logout();
</script>

<template>
  <main style="padding: 24px">
    <h1>Login</h1>

    <p v-if="isAuthed()">You are signed in.</p>
    <p v-else>You are signed out.</p>

    <button v-if="!isAuthed()" @click="onLogin">Login with Cognito</button>
    <button v-else @click="onLogout">Logout</button>
  </main>
</template>
