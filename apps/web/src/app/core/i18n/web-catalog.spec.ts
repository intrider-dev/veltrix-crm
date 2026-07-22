import english from '../../../../public/i18n/en/web.json';
import russian from '../../../../public/i18n/ru/web.json';

describe('application translation catalogs', () => {
  it('keep English and Russian keys in exact parity', () => {
    expect(Object.keys(russian).sort()).toEqual(Object.keys(english).sort());
  });

  it('keep placeholders identical across required locales', () => {
    const placeholders = (value: string) =>
      [...value.matchAll(/\{([a-zA-Z][\w]*)\}/g)].map((match) => match[1]).sort();
    for (const key of Object.keys(english) as (keyof typeof english)[]) {
      expect(placeholders(russian[key]), key).toEqual(placeholders(english[key]));
    }
  });
});
