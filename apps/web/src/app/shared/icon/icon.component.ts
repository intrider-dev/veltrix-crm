import { ChangeDetectionStrategy, Component, input } from '@angular/core';

export type IconName =
  | 'activity'
  | 'add'
  | 'attachment'
  | 'audit'
  | 'back'
  | 'building'
  | 'bookmark'
  | 'chat'
  | 'chevron'
  | 'close'
  | 'check'
  | 'checks'
  | 'clock'
  | 'contact'
  | 'dashboard'
  | 'deal'
  | 'language'
  | 'like'
  | 'lead'
  | 'mail'
  | 'microphone'
  | 'menu'
  | 'moon'
  | 'search'
  | 'shield'
  | 'settings'
  | 'calendar'
  | 'automation'
  | 'report'
  | 'notification'
  | 'phone'
  | 'pin'
  | 'play'
  | 'pause'
  | 'reaction'
  | 'reply'
  | 'retry'
  | 'download'
  | 'eye'
  | 'eyeOff'
  | 'file'
  | 'project'
  | 'send'
  | 'sun'
  | 'video';

@Component({
  selector: 'app-icon',
  template: `
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.8"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      @switch (name()) {
        @case ('dashboard') {
          <path d="M4 4h6v7H4zM14 4h6v4h-6zM14 12h6v8h-6zM4 15h6v5H4z" />
        }
        @case ('contact') {
          <circle cx="12" cy="8" r="3" />
          <path d="M5 20c.7-4 3.1-6 7-6s6.3 2 7 6" />
        }
        @case ('building') {
          <path d="M4 21V5l10-2v18M14 8h6v13M8 8h2M8 12h2M8 16h2M17 12h1M17 16h1M2 21h20" />
        }
        @case ('deal') {
          <path d="M4 7h16v12H4zM8 7V5h8v2M4 12h16M10 12v2h4v-2" />
        }
        @case ('lead') {
          <path d="M5 19c2-5 4-8 7-8s5 3 7 8M12 11V4M9 7l3-3 3 3" />
        }
        @case ('calendar') {
          <path d="M4 6h16v14H4zM8 3v6M16 3v6M4 10h16" />
        }
        @case ('automation') {
          <path d="M6 7h8a4 4 0 0 1 4 4v1M18 8v4h-4M18 17h-8a4 4 0 0 1-4-4v-1M6 16v-4h4" />
        }
        @case ('report') {
          <path d="M5 20V10M12 20V4M19 20v-7M3 20h18" />
        }
        @case ('notification') {
          <path d="M6 16h12l-1.5-2v-4a4.5 4.5 0 0 0-9 0v4zM10 19h4" />
        }
        @case ('chat') {
          <path d="M4 5h16v11H9l-5 4zM8 9h8M8 12h5" />
        }
        @case ('mail') {
          <path d="M3 5h18v14H3zM3 7l9 7 9-7" />
        }
        @case ('phone') {
          <path
            d="M7 3H4a1 1 0 0 0-1 1c0 9.4 7.6 17 17 17a1 1 0 0 0 1-1v-3l-4-2-2 2c-3.4-1.4-6-4-7.4-7.4L9 7z"
          />
        }
        @case ('video') {
          <path d="M3 6h12v12H3zM15 10l6-3v10l-6-3z" />
        }
        @case ('attachment') {
          <path d="M8 12.5 14.5 6a3 3 0 0 1 4.2 4.2l-8 8a5 5 0 0 1-7.1-7.1l8-8" />
        }
        @case ('microphone') {
          <rect x="9" y="3" width="6" height="12" rx="3" />
          <path d="M5 11a7 7 0 0 0 14 0M12 18v3M8 21h8" />
        }
        @case ('pin') {
          <path d="m9 3 6 6-2 2 3 4-1 1-4-3-2 2-6-6zM9 15l-5 5" />
        }
        @case ('bookmark') {
          <path d="M6 4h12v17l-6-4-6 4z" />
        }
        @case ('like') {
          <path
            d="M7 21H4V10h3M7 10l4-7c2 0 3 1.5 2 4l-.5 2H19a2 2 0 0 1 2 2l-2 8a2 2 0 0 1-2 2H7z"
          />
        }
        @case ('reaction') {
          <circle cx="12" cy="12" r="9" />
          <path d="M8.5 10h.01M15.5 10h.01M8 14c1 1.5 2.3 2 4 2s3-.5 4-2" />
        }
        @case ('reply') {
          <path d="m10 8-5 4 5 4v-3h3c3.5 0 5.5 1.3 7 4-1-5-3.5-7-7-7h-3z" />
        }
        @case ('check') {
          <path d="m5 12 4 4L19 6" />
        }
        @case ('checks') {
          <path d="m2.5 12 4 4 10-10M10 15l2 2 10-10" />
        }
        @case ('clock') {
          <circle cx="12" cy="12" r="9" />
          <path d="M12 7v5l3 2" />
        }
        @case ('retry') {
          <path
            d="M20 7v5h-5M4 17v-5h5M18.5 11A7 7 0 0 0 6 7.5L4 12M5.5 13A7 7 0 0 0 18 16.5l2-4.5"
          />
        }
        @case ('download') {
          <path d="M12 3v12M7 10l5 5 5-5M4 20h16" />
        }
        @case ('eye') {
          <path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z" />
          <circle cx="12" cy="12" r="2.5" />
        }
        @case ('eyeOff') {
          <path
            d="m3 3 18 18M10.6 6.2A10 10 0 0 1 12 6c6 0 9.5 6 9.5 6a15 15 0 0 1-2.3 3M6.5 6.5C4 8.2 2.5 12 2.5 12s3.5 6 9.5 6c1.4 0 2.6-.3 3.7-.7M10 10a2.8 2.8 0 0 0 4 4"
          />
        }
        @case ('file') {
          <path d="M6 3h8l4 4v14H6zM14 3v5h5" />
        }
        @case ('play') {
          <path d="m8 5 11 7-11 7z" />
        }
        @case ('pause') {
          <path d="M8 5v14M16 5v14" />
        }
        @case ('send') {
          <path d="m3 11 18-8-8 18-2-7zM11 14l4-5" />
        }
        @case ('project') {
          <path d="M4 7h6l2-2h8v14H4zM4 10h16M8 14h8M8 17h5" />
        }
        @case ('language') {
          <circle cx="12" cy="12" r="9" />
          <path d="M3 12h18M12 3c3 3 4 6 4 9s-1 6-4 9M12 3c-3 3-4 6-4 9s1 6 4 9" />
        }
        @case ('activity') {
          <path d="M4 12h3l2-6 4 12 2-6h5" />
        }
        @case ('settings') {
          <circle cx="12" cy="12" r="3" />
          <path
            d="M12 2v3M12 19v3M4.9 4.9 7 7M17 17l2.1 2.1M2 12h3M19 12h3M4.9 19.1 7 17M17 7l2.1-2.1"
          />
        }
        @case ('shield') {
          <path d="M12 3 20 6v5c0 5-3.2 8.2-8 10-4.8-1.8-8-5-8-10V6z" />
          <path d="m8.5 12 2.2 2.2 4.8-5" />
        }
        @case ('audit') {
          <path d="M5 3h14v18H5zM8 8h8M8 12h8M8 16h5" />
        }
        @case ('search') {
          <circle cx="10.5" cy="10.5" r="6.5" />
          <path d="m16 16 5 5" />
        }
        @case ('menu') {
          <path d="M4 7h16M4 12h16M4 17h16" />
        }
        @case ('close') {
          <path d="m6 6 12 12M18 6 6 18" />
        }
        @case ('chevron') {
          <path d="m9 6 6 6-6 6" />
        }
        @case ('back') {
          <path d="m15 18-6-6 6-6" />
        }
        @case ('add') {
          <path d="M12 5v14M5 12h14" />
        }
        @case ('sun') {
          <circle cx="12" cy="12" r="4" />
          <path
            d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M19.1 4.9l-1.4 1.4M6.3 17.7l-1.4 1.4"
          />
        }
        @case ('moon') {
          <path d="M20 15.5A8 8 0 0 1 8.5 4 8 8 0 1 0 20 15.5Z" />
        }
      }
    </svg>
  `,
  styles: `
    :host {
      display: inline-grid;
      width: 1.25rem;
      height: 1.25rem;
      flex: 0 0 auto;
      place-items: center;
      vertical-align: middle;
    }
    svg {
      width: 100%;
      height: 100%;
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class IconComponent {
  readonly name = input.required<IconName>();
}
