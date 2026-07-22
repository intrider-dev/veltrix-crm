import {
  isSafeWebhookUrl,
  parseJsonObject,
  trimmedOrNull,
  uniqueTokens,
} from './feature-validation';

describe('feature validation', () => {
  it('normalizes optional values and comma-separated tokens', () => {
    expect(trimmedOrNull('  ')).toBeNull();
    expect(trimmedOrNull(' Acme ')).toBe('Acme');
    expect(uniqueTokens('deal.created, contact.updated deal.created')).toEqual([
      'deal.created',
      'contact.updated',
    ]);
  });

  it('accepts only JSON objects for typed rule parameters', () => {
    expect(parseJsonObject('{"title":"Follow up"}')).toEqual({ title: 'Follow up' });
    expect(parseJsonObject('[]')).toBeNull();
    expect(parseJsonObject('{')).toBeNull();
  });

  it('allows HTTP webhook URLs and rejects executable schemes', () => {
    expect(isSafeWebhookUrl('https://hooks.example.test/crm')).toBe(true);
    expect(isSafeWebhookUrl('javascript:alert(1)')).toBe(false);
  });
});
