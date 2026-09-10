// server/entra.test.ts
import { describe, expect, it } from "vitest";
import { buildAuthorizeUrl } from "./entra";

describe("buildAuthorizeUrl", () => {
  it("includes all required OIDC params", () => {
    const url = buildAuthorizeUrl(
      {
        authorizeUrl: "https://login.microsoftonline.com/tenant/oauth2/v2.0/authorize",
        tokenUrl: "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
        clientId: "chat-app-client",
        clientSecret: "unused-here",
        redirectUri: "http://localhost:5173/callback",
        scope: "openid profile api://agent-gateway/.default",
      },
      "test-state",
    );
    expect(url).toContain("https://login.microsoftonline.com/tenant/oauth2/v2.0/authorize?");
    expect(url).toContain("client_id=chat-app-client");
    expect(url).toContain("response_type=code");
    expect(url).toContain("state=test-state");
  });
});
