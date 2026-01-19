function requireEnv(name: string): string {
  const value = import.meta.env[name];
  if (!value) throw new Error(`Missing env: ${name}`);
  return value;
}

export const config = {
  cognitoDomain: () => requireEnv("VITE_COGNITO_DOMAIN"),
  cognitoClientId: () => requireEnv("VITE_COGNITO_CLIENT_ID"),
  redirectUri: () => requireEnv("VITE_COGNITO_REDIRECT_URI"),
  logoutUri: () => requireEnv("VITE_COGNITO_LOGOUT_URI"),
  apiBaseUrl: () => requireEnv("VITE_API_BASE_URL"),
};
