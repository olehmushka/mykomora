import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
import i18next from "eslint-plugin-i18next";
import prettier from "eslint-config-prettier";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // SPEC.md, "Internationalisation": "no user-visible string is ever
  // hardcoded, enforced by lint on the web side". Scoped to the files M2
  // adds rather than applied repo-wide: every pre-M2 page still has
  // hardcoded English strings (people, family settings, sign-in), and
  // those are migrated into messages/*.json opportunistically as they're
  // touched, not as a blanket rewrite this milestone (see MILESTONES.md's
  // M2 note). Widen this glob as more of the app gets its strings
  // externalised.
  {
    files: [
      "src/app/locations/**/*.tsx",
      "src/app/settings/taxonomy/**/*.tsx",
      "src/components/locale-switcher.tsx",
    ],
    plugins: { i18next },
    rules: {
      "i18next/no-literal-string": [
        "error",
        {
          markupOnly: true,
          ignoreAttribute: [
            "className",
            "href",
            "id",
            "htmlFor",
            "name",
            "type",
            "value",
            "aria-labelledby",
          ],
        },
      ],
    },
  },
  // Must stay last: turns off stylistic rules that Prettier owns.
  prettier,
  globalIgnores([
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // Generated from ../core-api/api/openapi.yaml.
    "src/lib/api/schema.d.ts",
  ]),
]);

export default eslintConfig;
