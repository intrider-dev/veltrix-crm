import { extractPlaceholders, placeholderMismatches } from './translation-placeholders';

describe('translation placeholder validation', () => {
  it('extracts and sorts typed placeholders', () => {
    expect(extractPlaceholders('Hello {name}, you have {count} tasks for {name}.')).toEqual([
      'count',
      'name',
      'name',
    ]);
  });

  it('accepts reordered placeholders with equal multiplicity', () => {
    expect(placeholderMismatches(['count', 'name'], '{name}: {count}')).toEqual([]);
  });

  it('reports missing, extra, and duplicate-count mismatches', () => {
    expect(placeholderMismatches(['count', 'name', 'name'], '{name} {unexpected}')).toEqual([
      'count',
      'name',
      'unexpected',
    ]);
  });
});
