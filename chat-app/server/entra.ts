// server/entra.ts
export interface EntraChatConfig {
  authorizeUrl: string;
  tokenUrl: string;
  clientId: string;
  clientSecret: string;
  redirectUri: string;
  scope: string;
}

export function buildAuthorizeUrl(cfg: EntraChatConfig, state: string): string {
  const params = new URLSearchParams({
    client_id: cfg.clientId,
    response_type: "code",
    redirect_uri: cfg.redirectUri,
    scope: cfg.scope,
    state,
  });
  return `${cfg.authorizeUrl}?${params.toString()}`;
}

export async function exchangeCodeForToken(cfg: EntraChatConfig, code: string): Promise<string> {
  const params = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: cfg.clientId,
    client_secret: cfg.clientSecret,
    code,
    redirect_uri: cfg.redirectUri,
  });
  const resp = await fetch(cfg.tokenUrl, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: params.toString(),
  });
  const body = (await resp.json()) as { access_token?: string; error?: string };
  if (!resp.ok || !body.access_token) {
    throw new Error(`entra code exchange failed: ${body.error ?? resp.statusText}`);
  }
  return body.access_token;
}
