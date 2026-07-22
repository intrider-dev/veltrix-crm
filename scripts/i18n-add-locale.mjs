import { mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import process from "node:process";

const root = resolve(import.meta.dirname, "..");
const localeArg = process.argv.findIndex((value) => value === "--locale");
const requested = localeArg >= 0 ? process.argv[localeArg + 1] : "";
if (!requested) throw new Error("usage: pnpm i18n:add-locale --locale de-DE");
// Backend and PostgreSQL compare locale identifiers case-insensitively and
// persist a lowercase BCP 47 representation. Using the same representation
// for catalog paths keeps generated URLs portable on case-sensitive hosts.
const locale = new Intl.Locale(requested).toString().toLowerCase();
const nameArg = process.argv.findIndex((value) => value === "--name");
const requestedName = nameArg >= 0 ? process.argv[nameArg + 1]?.trim() : "";
const manifestPath = join(root, "packages/i18n/locale-manifest.json");
const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
if (manifest.locales.some((entry) => entry.tag.toLowerCase() === locale))
  throw new Error(`${locale} already exists`);
const nativeName =
  requestedName ||
  new Intl.DisplayNames([locale], { type: "language" }).of(locale) ||
  locale;
const direction =
  new Intl.Locale(locale).getTextInfo?.().direction ??
  (["ar", "fa", "he", "ur"].includes(new Intl.Locale(locale).language)
    ? "rtl"
    : "ltr");
manifest.locales.push({ tag: locale, nativeName, direction, required: true });
manifest.locales.sort((left, right) => left.tag.localeCompare(right.tag));
await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");

const productPath = join(root, "packages/product-config/product.json");
const product = JSON.parse(await readFile(productPath, "utf8"));
product.supportedLocales = [
  ...new Set([...product.supportedLocales, locale]),
].sort();
await writeFile(productPath, `${JSON.stringify(product, null, 2)}\n`, "utf8");

const source = join(root, "packages/i18n/source/en");
const target = join(root, "packages/i18n/locales", locale);
await mkdir(target, { recursive: true });
for (const file of (await readdir(source))
  .filter((name) => name.endsWith(".json"))
  .sort()) {
  const catalog = JSON.parse(await readFile(join(source, file), "utf8"));
  const scaffold = Object.fromEntries(
    Object.entries(catalog).map(([key, value]) => [key, `⟦TODO⟧ ${value}`]),
  );
  await writeFile(
    join(target, file),
    `${JSON.stringify(scaffold, null, 2)}\n`,
    "utf8",
  );
}
process.stdout.write(
  `Created ${locale} (${direction}) and enabled it in product config. ` +
    `Replace every ⟦TODO⟧ value, then run pnpm generate:brand && pnpm generate:i18n.\n`,
);
