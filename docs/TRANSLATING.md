# Translating WiseLabz

The web UI reads every string through `t('namespace.key')` (react-i18next).
English (`web/src/i18n/en.ts`) is the source of truth and the fallback for any
key a locale does not translate, so a partial translation is fine to ship.

Users pick a language under **Settings → Appearance → Language**. The default,
"Automatic", follows the browser's language list.

## Add a language

1. Create `web/src/i18n/locales/<code>.ts` (use a BCP 47 tag such as `de` or
   `pt-BR`). Export a `Catalog` — the same nested shape as `en`, any subset of
   the keys:

   ```ts
   import type { Catalog } from '../languages';

   export const de: Catalog = {
     nav: { dashboard: 'Übersicht', settings: 'Einstellungen' },
   };
   ```

   `web/src/i18n/locales/pt-BR.ts` is a worked example.

2. Register it in `LANGUAGES` in `web/src/i18n/languages.ts`. The `name` is
   shown in the picker in the language's own script, and the loader keeps the
   locale out of the main bundle until it is selected:

   ```ts
   de: { name: 'Deutsch', load: () => import('./locales/de').then((m) => m.de) },
   ```

3. Run `bun test src/i18n` and `bun run lint && bun run build` from `web/`.
   The test fails if your file uses a key that does not exist in `en.ts`.

## Rules

- Keep keys exactly as in `en.ts`; only translate the values.
- Keep `{{placeholders}}` untouched (e.g. `{{count}}`, `{{name}}`).
- Don't translate product names, API terms or code identifiers.
- When English changes, update the locale in the same PR if you can; stale
  or missing keys fall back to English rather than breaking.
