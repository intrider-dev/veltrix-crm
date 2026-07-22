import { safeLocalReturnUrl } from './auth.guard';

describe('safeLocalReturnUrl', () => {
  it('preserves an internal invitation URL including its token', () => {
    expect(safeLocalReturnUrl('/invitations/accept?token=invite-123')).toBe(
      '/invitations/accept?token=invite-123',
    );
  });

  it.each([null, '', 'https://attacker.example', '//attacker.example', '/login?returnUrl=/'])(
    'falls back to the dashboard for an unsafe value: %s',
    (value) => expect(safeLocalReturnUrl(value)).toBe('/dashboard'),
  );
});
