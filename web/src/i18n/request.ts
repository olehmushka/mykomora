import { cookies, headers } from "next/headers";
import { getRequestConfig } from "next-intl/server";

import { NEXT_LOCALE_COOKIE, resolveLocale } from "@/i18n/locale";

export default getRequestConfig(async () => {
  const [jar, requestHeaders] = await Promise.all([cookies(), headers()]);

  const locale = resolveLocale(
    jar.get(NEXT_LOCALE_COOKIE)?.value,
    requestHeaders.get("accept-language") ?? undefined,
  );

  return {
    locale,
    messages: (await import(`../../messages/${locale}.json`)).default,
  };
});
