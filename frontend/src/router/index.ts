import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import LoginView from "../views/LoginView.vue";
import CallbackView from "../views/CallbackView.vue";
import SummaryView from "../views/SummaryView.vue";
import ShopifyView from "../views/ShopifyView.vue";
import { isAuthed } from "../services/auth";

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: "/", name: "home", component: HomeView, meta: { requiresAuth: true } },
    { path: "/login", name: "login", component: LoginView },
    { path: "/callback", name: "callback", component: CallbackView },
    { path: "/summary", name: "summary", component: SummaryView, meta: { requiresAuth: true } },
    { path: "/shopify", name: "shopify", component: ShopifyView, meta: { requiresAuth: true } },
  ],
});

router.beforeEach((to) => {
  const requiresAuth = Boolean(to.meta.requiresAuth);
  if (requiresAuth && !isAuthed()) {
    return { name: "login" };
  }
  return true;
});

export default router;
