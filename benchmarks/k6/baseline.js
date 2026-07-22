import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const baseURL = __ENV.BASE_URL || 'http://localhost:8080';
const profile = __ENV.K6_PROFILE || 'baseline';
const measuredVUs = profile === 'stretch' ? 100 : Number(__ENV.K6_VUS || 50);
const measuredDuration = __ENV.K6_DURATION || '5m';
const warmupDuration = __ENV.K6_WARMUP || '1m';

const readLatency = new Trend('crm_read_latency', true);
const writeLatency = new Trend('crm_write_latency', true);
const searchLatency = new Trend('crm_search_latency', true);
const errors = new Rate('crm_errors');
const operations = new Counter('crm_operations');

export const options = {
  discardResponseBodies: false,
  summaryTrendStats: ['avg', 'med', 'p(90)', 'p(95)', 'p(99)', 'max'],
  scenarios: {
    warmup: {
      executor: 'constant-vus',
      exec: 'warmup',
      vus: Math.min(10, measuredVUs),
      duration: warmupDuration,
      gracefulStop: '10s',
    },
    measured: {
      executor: 'constant-vus',
      exec: 'measured',
      vus: measuredVUs,
      startTime: warmupDuration,
      duration: measuredDuration,
      gracefulStop: '15s',
    },
  },
  thresholds: {
    crm_errors: ['rate<0.005'],
    crm_read_latency: ['p(95)<150'],
    crm_write_latency: ['p(95)<250'],
    crm_search_latency: ['p(95)<250'],
  },
};

let state;

function ensureSession() {
  if (state) return state;
  const response = http.post(
    `${baseURL}/api/v1/auth/login`,
    JSON.stringify({
      email: __ENV.DEMO_EMAIL || 'admin@demo.local',
      password: __ENV.DEMO_PASSWORD || 'Demo123!',
    }),
    { headers: { 'Content-Type': 'application/json', Accept: 'application/json' }, tags: { operation: 'login' } },
  );
  if (!check(response, { 'login succeeds': (value) => value.status === 200 })) {
    throw new Error(`login failed with ${response.status}: ${response.body}`);
  }
  const body = response.json();
  if (!body.workspaces?.length) throw new Error('benchmark user has no workspace');
  const cookies = http.cookieJar().cookiesForURL(baseURL);
  state = { workspaceId: body.workspaces[0].id, csrf: cookies['XSRF-TOKEN']?.[0] || '' };
  return state;
}

function headers(extra = {}) {
  const session = ensureSession();
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    'X-XSRF-TOKEN': session.csrf,
    ...extra,
  };
}

function request(measuredRun) {
  const session = ensureSession();
  const sequence = exec.scenario.iterationInTest % 100;
  let response;
  let kind;
  if (sequence < 45) {
    response = http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/contacts?limit=50`, {
      headers: headers(), tags: { operation: 'contacts.list' },
    });
    kind = 'read';
  } else if (sequence < 65) {
    response = http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/dashboard`, {
      headers: headers(), tags: { operation: 'dashboard.read' },
    });
    kind = 'read';
  } else if (sequence < 82) {
    response = http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/search?q=synthetic`, {
      headers: headers(), tags: { operation: 'search.global' },
    });
    kind = 'search';
  } else if (sequence < 92) {
    const contactPage = http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/contacts?limit=1`, {
      headers: headers(), tags: { operation: 'contacts.list_for_detail' },
    });
    const contact = contactPage.status === 200 ? contactPage.json().items?.[0] : null;
    response = contact
      ? http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/contacts/${contact.id}`, {
          headers: headers(), tags: { operation: 'contacts.get' },
        })
      : contactPage;
    kind = 'read';
  } else {
    const unique = `${exec.vu.idInTest}-${exec.scenario.iterationInTest}-${Date.now()}`;
    response = http.post(
      `${baseURL}/api/v1/workspaces/${session.workspaceId}/contacts`,
      JSON.stringify({ firstName: 'Load', lastName: `Contact ${unique}`, status: 'active' }),
      {
        headers: headers({ 'Idempotency-Key': `k6-${profile}-${unique}` }),
        tags: { operation: 'contacts.create' },
      },
    );
    kind = 'write';
  }

  const succeeded = response.status >= 200 && response.status < 300;
  if (measuredRun) {
    errors.add(!succeeded);
    operations.add(1);
    if (kind === 'write') writeLatency.add(response.timings.duration);
    else if (kind === 'search') searchLatency.add(response.timings.duration);
    else readLatency.add(response.timings.duration);
  }
  check(response, { [`${kind} succeeds`]: () => succeeded });
  sleep(Number(__ENV.K6_THINK_TIME || 0.1));
}

export function warmup() {
  request(false);
}

export function measured() {
  request(true);
}

export function handleSummary(data) {
  const output = __ENV.K6_RESULT_PATH || `/benchmarks/results/k6-${profile}.summary.json`;
  return {
    stdout: `\nMeasured profile: ${profile}; VUs: ${measuredVUs}; duration: ${measuredDuration}\n`,
    [output]: JSON.stringify(data, null, 2),
  };
}
