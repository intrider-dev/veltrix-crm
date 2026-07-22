import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const baseURL = __ENV.BASE_URL || 'http://localhost:8080';
const profile = __ENV.K6_PROFILE || 'baseline';
// Avoid K6_VUS/K6_DURATION: k6 itself consumes those names and replaces the
// declared scenarios, which would skip the named warmup/measured executors.
const measuredVUs = profile === 'stretch' ? 100 : Number(__ENV.CRM_BENCHMARK_VUS || 50);
const measuredDuration = __ENV.CRM_BENCHMARK_DURATION || '5m';
const warmupDuration = __ENV.CRM_BENCHMARK_WARMUP || '1m';

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

export function setup() {
  // Authenticate once before VUs start. Logging in independently from every
  // VU measures the intentional login rate limiter instead of CRM workload.
  // The auth security path has separate negative tests; this scenario shares
  // one pre-established demo session across the synthetic team.
  const response = http.post(
    `${baseURL}/api/v1/auth/login`,
    JSON.stringify({
      email: __ENV.DEMO_EMAIL || 'admin@demo.local',
      password: __ENV.DEMO_PASSWORD || 'Demo123!',
    }),
    {
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      tags: { operation: 'benchmark.setup.login' },
    },
  );
  if (response.status !== 200) {
    throw new Error(`benchmark setup login failed with ${response.status}: ${response.body}`);
  }
  const body = response.json();
  if (!body.workspaces?.length) throw new Error('benchmark user has no workspace');
  const csrf = response.cookies['XSRF-TOKEN']?.[0]?.value || '';
  const cookie = Object.entries(response.cookies)
    .flatMap(([name, values]) => values.map((value) => `${name}=${value.value}`))
    .join('; ');
  if (!csrf || !cookie) throw new Error('benchmark setup did not receive session and CSRF cookies');
  return { workspaceId: body.workspaces[0].id, csrf, cookie };
}

function ensureSession(setupData) {
  if (state) return state;
  if (!setupData?.workspaceId || !setupData?.csrf || !setupData?.cookie) {
    throw new Error('benchmark setup session is unavailable');
  }
  state = setupData;
  return state;
}

function headers(session, extra = {}) {
  return {
    Accept: 'application/json',
    'Content-Type': 'application/json',
    'X-XSRF-TOKEN': session.csrf,
    Cookie: session.cookie,
    ...extra,
  };
}

function request(setupData, measuredRun) {
  const session = ensureSession(setupData);
  const sequence = exec.scenario.iterationInTest % 100;
  let response;
  let kind;
  if (sequence < 45) {
    response = http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/contacts?limit=50`, {
      headers: headers(session), tags: { operation: 'contacts.list' },
    });
    kind = 'read';
  } else if (sequence < 65) {
    response = http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/dashboard`, {
      headers: headers(session), tags: { operation: 'dashboard.read' },
    });
    kind = 'read';
  } else if (sequence < 82) {
    response = http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/search?q=Contact%20042424`, {
      headers: headers(session), tags: { operation: 'search.global' },
    });
    kind = 'search';
  } else if (sequence < 92) {
    const contactPage = http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/contacts?limit=1`, {
      headers: headers(session), tags: { operation: 'contacts.list_for_detail' },
    });
    const contact = contactPage.status === 200 ? contactPage.json().items?.[0] : null;
    response = contact
      ? http.get(`${baseURL}/api/v1/workspaces/${session.workspaceId}/contacts/${contact.id}`, {
          headers: headers(session), tags: { operation: 'contacts.get' },
        })
      : contactPage;
    kind = 'read';
  } else {
    const unique = `${exec.vu.idInTest}-${exec.scenario.iterationInTest}-${Date.now()}`;
    response = http.post(
      `${baseURL}/api/v1/workspaces/${session.workspaceId}/contacts`,
      JSON.stringify({ firstName: 'Load', lastName: `Contact ${unique}`, status: 'active' }),
      {
        headers: headers(session, { 'Idempotency-Key': `k6-${profile}-${unique}` }),
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
  sleep(Number(__ENV.CRM_BENCHMARK_THINK_TIME || 0.1));
}

export function warmup(setupData) {
  request(setupData, false);
}

export function measured(setupData) {
  request(setupData, true);
}

export function handleSummary(data) {
  const output = __ENV.K6_RESULT_PATH || `/benchmarks/results/k6-${profile}.summary.json`;
  // k6 includes setup() data in custom summaries. It contains the shared
  // synthetic session cookie and CSRF value, which are inputs to the run and
  // must never be retained as benchmark evidence.
  delete data.setup_data;
  return {
    stdout: `\nMeasured profile: ${profile}; VUs: ${measuredVUs}; duration: ${measuredDuration}\n`,
    [output]: JSON.stringify(data, null, 2),
  };
}
