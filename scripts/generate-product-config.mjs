import { execFileSync } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const config = JSON.parse(
  await readFile(resolve(root, "packages/product-config/product.json"), "utf8"),
);
const required = [
  "productName",
  "shortName",
  "repositoryName",
  "description",
  "cookiePrefix",
  "logoPath",
  "themeColor",
  "backgroundColor",
  "defaultLocale",
  "supportedLocales",
];
for (const key of required) {
  if (config[key] === undefined || config[key] === "") {
    throw new Error(`product.json missing ${key}`);
  }
}

const ts = `// Code generated from product.json by scripts/generate-product-config.mjs. DO NOT EDIT.\nexport const productConfig = ${JSON.stringify(config, null, 2)} as const;\n\nexport type SupportedLocale = (typeof productConfig.supportedLocales)[number];\n`;
await writeFile(resolve(root, "packages/product-config/src/index.ts"), ts);

const goString = (value) => JSON.stringify(value);
const go = execFileSync("gofmt", [], {
  encoding: "utf8",
  input: `// Code generated from packages/product-config/product.json. DO NOT EDIT.\npackage brand\n\nvar Config = ProductConfig{\n\tProductName: ${goString(config.productName)},\n\tShortName: ${goString(config.shortName)},\n\tRepositoryName: ${goString(config.repositoryName)},\n\tDescription: ${goString(config.description)},\n\tCookiePrefix: ${goString(config.cookiePrefix)},\n\tSupportURL: ${goString(config.supportUrl)},\n\tSecurityURL: ${goString(config.securityUrl)},\n\tLogoPath: ${goString(config.logoPath)},\n\tThemeColor: ${goString(config.themeColor)},\n\tBackgroundColor: ${goString(config.backgroundColor)},\n\tDefaultLocale: ${goString(config.defaultLocale)},\n\tSupportedLocales: []string{${config.supportedLocales.map(goString).join(", ")}},\n\tDefaultTheme: ${goString(config.defaultTheme)},\n}\n`,
});
await writeFile(resolve(root, "apps/api/internal/platform/brand/generated.go"), go);

const manifest = {
  name: config.productName,
  short_name: config.shortName,
  description: config.description,
  id: "/",
  start_url: "/dashboard",
  scope: "/",
  display: "standalone",
  background_color: config.backgroundColor,
  theme_color: config.themeColor,
  icons: [
    {
      src: config.logoPath,
      sizes: "any",
      type: "image/svg+xml",
      purpose: "any maskable",
    },
  ],
};
await writeFile(
  resolve(root, "apps/web/public/manifest.webmanifest"),
  `${JSON.stringify(manifest, null, 2)}\n`,
);

const html = `<!-- Generated from packages/product-config/product.json. DO NOT EDIT. -->
<!doctype html>
<html lang="${config.defaultLocale}">
  <head>
    <meta charset="utf-8" />
    <title>${config.productName}</title>
    <base href="/" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="theme-color" content="${config.themeColor}" />
    <meta
      name="description"
      content="${config.description}"
    />
    <link rel="manifest" href="manifest.webmanifest" />
    <link rel="icon" type="image/svg+xml" href="${config.logoPath}" />
  </head>
  <body>
    <app-root></app-root>
  </body>
</html>
`;
await writeFile(resolve(root, "apps/web/src/index.html"), html);
