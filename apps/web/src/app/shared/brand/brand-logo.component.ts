import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { productConfig } from '@veltrix-crm/product-config';

@Component({
  selector: 'app-brand-logo',
  template: `
    <span class="lockup" [attr.data-size]="size()">
      <img [src]="product.logoPath" alt="" aria-hidden="true" />
      @if (showName()) {
        <span class="name">{{ product.shortName }}</span>
      }
    </span>
  `,
  styles: `
    :host {
      display: inline-flex;
      min-width: 0;
      vertical-align: middle;
    }
    .lockup {
      display: inline-flex;
      align-items: center;
      gap: 0.62rem;
      min-width: 0;
    }
    img {
      width: 1.95rem;
      height: 1.95rem;
      flex: 0 0 auto;
    }
    .name {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 760;
      letter-spacing: -0.025em;
    }
    [data-size='small'] img {
      width: 1.55rem;
      height: 1.55rem;
    }
    [data-size='large'] img {
      width: 3.25rem;
      height: 3.25rem;
    }
    [data-size='large'] .name {
      font-size: 1.2rem;
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class BrandLogoComponent {
  readonly showName = input(true);
  readonly size = input<'small' | 'regular' | 'large'>('regular');
  readonly product = productConfig;
}
