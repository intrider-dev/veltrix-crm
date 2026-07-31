// Code generated from product.json by scripts/generate-product-config.mjs. DO NOT EDIT.
export const productConfig = {
  "productName": "VeltrixCRM",
  "shortName": "VeltrixCRM",
  "repositoryName": "veltrix-crm",
  "description": "A resource-conscious, multi-tenant Sales CRM with reproducible performance evidence.",
  "cookiePrefix": "veltrix_crm",
  "supportUrl": "https://github.com/veltrixcrm/veltrix-crm/issues",
  "securityUrl": "https://github.com/veltrixcrm/veltrix-crm/security/policy",
  "logoPath": "/icons/veltrix-mark.svg",
  "themeColor": "#174b40",
  "backgroundColor": "#f4f6f3",
  "defaultLocale": "en",
  "supportedLocales": [
    "en",
    "ru"
  ],
  "defaultTheme": "system"
} as const;

export type SupportedLocale = (typeof productConfig.supportedLocales)[number];
