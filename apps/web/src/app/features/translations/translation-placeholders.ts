const placeholderPattern = /\{([a-zA-Z][\w]*)\}/g;

export function extractPlaceholders(value: string): string[] {
  return [...value.matchAll(placeholderPattern)].map((match) => match[1]).sort();
}

export function placeholderMismatches(
  expected: readonly string[],
  translatedText: string,
): string[] {
  const expectedCounts = count(expected);
  const actualCounts = count(extractPlaceholders(translatedText));
  return [...new Set([...expectedCounts.keys(), ...actualCounts.keys()])]
    .filter((name) => expectedCounts.get(name) !== actualCounts.get(name))
    .sort();
}

function count(values: readonly string[]): Map<string, number> {
  const result = new Map<string, number>();
  for (const value of values) result.set(value, (result.get(value) ?? 0) + 1);
  return result;
}
