import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Emits a self-contained server bundle so the runtime image can carry the app
  // without node_modules. See deploy/ and the Dockerfile's runtime stage.
  output: "standalone",

  // The web app is a thin client: it must not become the identity layer, so it
  // holds no secrets of its own. CORE_API_URL is the internal address of
  // core-api and stays server-side — never a NEXT_PUBLIC_ variable.
};

export default nextConfig;
