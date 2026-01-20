function requireEnv(name: string): string {
  // @ts-expect-error - import.meta.env type is extended by Vite
  const value = import.meta.env[name];
  if (!value) throw new Error(`Missing env: ${name}`);
  return value;
}

export const config = {
  cognitoDomain: () => requireEnv("REACT_APP_COGNITO_DOMAIN"),
  cognitoClientId: () => requireEnv("REACT_APP_COGNITO_CLIENT_ID"),
  redirectUri: () => requireEnv("REACT_APP_COGNITO_REDIRECT_URI"),
  logoutUri: () => requireEnv("REACT_APP_COGNITO_LOGOUT_URI"),
  apiBaseUrl: () => requireEnv("REACT_APP_API_BASE_URL"),
};
