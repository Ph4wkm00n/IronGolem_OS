/**
 * @irongolem/i18n
 *
 * Internationalization package for IronGolem OS. Provides a translation
 * engine with React integration, locale management, and fallback chains.
 */

export {
  t,
  getLocale,
  setLocale,
  getSupportedLocales,
  useTranslation,
  TranslationProvider,
} from "./i18n";

export type { Locale, TranslationKey, TranslationParams, Translations } from "./i18n";

// Locale data (re-exported for direct access when needed)
export { en } from "./locales/en";
export { es } from "./locales/es";
export { zh } from "./locales/zh";
