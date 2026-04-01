/**
 * i18n engine for IronGolem OS.
 *
 * Supports typed translation keys, parameterised strings, a React hook
 * and context provider, and automatic fallback to English when a key is
 * missing in the requested locale.
 */

import { createContext, useContext, useState, useCallback, createElement } from "react";
import type { ReactNode } from "react";
import { en } from "./locales/en";
import { es } from "./locales/es";
import { zh } from "./locales/zh";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

/** Supported locale codes. */
export type Locale = "en" | "es" | "fr" | "de" | "ja" | "zh" | "ko";

/** All valid translation keys (derived from the English locale). */
export type TranslationKey = keyof typeof en;

/** Parameters that can be interpolated into translated strings. */
export type TranslationParams = Record<string, string | number>;

/** A complete translations map for one locale. */
export type Translations = Record<TranslationKey, string>;

// ---------------------------------------------------------------------------
// Locale registry
// ---------------------------------------------------------------------------

const localeMap: Partial<Record<Locale, Translations>> = {
  en: en as Translations,
  es: es as Translations,
  zh: zh as Translations,
};

let currentLocale: Locale = "en";

// ---------------------------------------------------------------------------
// Core API
// ---------------------------------------------------------------------------

/** Get the active locale. */
export function getLocale(): Locale {
  return currentLocale;
}

/** Set the active locale. */
export function setLocale(locale: Locale): void {
  currentLocale = locale;
}

/** Return the list of locales that have translation data loaded. */
export function getSupportedLocales(): Locale[] {
  return Object.keys(localeMap) as Locale[];
}

/**
 * Translate a key, optionally interpolating parameters.
 *
 * Fallback chain: requested locale -> "en".
 * If a key is missing entirely, the raw key string is returned.
 *
 * Interpolation uses `{{paramName}}` placeholders.
 */
export function t(key: TranslationKey, params?: TranslationParams): string {
  const translations = localeMap[currentLocale];
  let value: string | undefined = translations?.[key];

  // Fallback to English
  if (value === undefined && currentLocale !== "en") {
    value = (localeMap.en as Translations)?.[key];
  }

  // Last resort: return the raw key
  if (value === undefined) {
    return key;
  }

  // Interpolate parameters
  if (params) {
    for (const [paramKey, paramValue] of Object.entries(params)) {
      value = value.replace(
        new RegExp(`\\{\\{${paramKey}\\}\\}`, "g"),
        String(paramValue),
      );
    }
  }

  return value;
}

// ---------------------------------------------------------------------------
// React integration
// ---------------------------------------------------------------------------

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: TranslationKey, params?: TranslationParams) => string;
}

const I18nContext = createContext<I18nContextValue>({
  locale: "en",
  setLocale: () => {},
  t,
});

/**
 * React hook for translations.
 *
 * Returns `{ t, locale, setLocale }` bound to the current provider context.
 */
export function useTranslation() {
  return useContext(I18nContext);
}

/**
 * React context provider for i18n.
 *
 * Wrap your app (or a subtree) with this to enable `useTranslation()`.
 */
export function TranslationProvider({
  children,
  defaultLocale = "en",
}: {
  children: ReactNode;
  defaultLocale?: Locale;
}) {
  const [locale, setLocaleState] = useState<Locale>(defaultLocale);

  const handleSetLocale = useCallback((newLocale: Locale) => {
    setLocale(newLocale);
    setLocaleState(newLocale);
  }, []);

  const translate = useCallback(
    (key: TranslationKey, params?: TranslationParams) => {
      // Ensure the module-level locale is in sync
      setLocale(locale);
      return t(key, params);
    },
    [locale],
  );

  return createElement(
    I18nContext.Provider,
    {
      value: {
        locale,
        setLocale: handleSetLocale,
        t: translate,
      },
    },
    children,
  );
}
