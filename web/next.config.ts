import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

const nextConfig: NextConfig = {
  // Emits a self-contained server bundle so the runtime image can carry the app
  // without node_modules. See deploy/ and the Dockerfile's runtime stage.
  output: "standalone",

  // The web app is a thin client: it must not become the identity layer, so it
  // holds no secrets of its own. CORE_API_URL is the internal address of
  // core-api and stays server-side — never a NEXT_PUBLIC_ variable.
};

// No `[locale]` route segments: this is a single-family tool with no need for
// localised URLs, so next-intl resolves the locale per request (the
// NEXT_LOCALE cookie, then Accept-Language — see src/i18n/locale.ts) instead
// of from the path.
const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

export default withNextIntl(nextConfig);
