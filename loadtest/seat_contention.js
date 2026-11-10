// Seat contention test: N customers try to hold the SAME seat at the same moment.
//
// Expected outcome (the double-booking guarantee):
//   - exactly 1 response is 201 (that customer holds the seat)
//   - the other N-1 responses are 409 (seat_unavailable)
//   - no other status codes
//
// Run:  make loadtest                       (defaults: BASE_URL=http://localhost:8080, USERS=200)
//       k6 run -e USERS=500 loadtest/seat_contention.js
//
// Requires the stack to be up with seed data (`make seed`).
import http from 'k6/http';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://localhost:8080';
const USERS = parseInt(__ENV.USERS || '200', 10);
const RUN_ID = Date.now().toString(36); // keeps user emails unique across runs
const JSON_HEADERS = { 'Content-Type': 'application/json' };

// 409 is an expected answer for the losing customers; 200 covers setup and teardown reads.
// Without this, k6 would count those as failed requests in http_req_failed.
http.setResponseCallback(http.expectedStatuses(200, 201, 409));

const winners = new Counter('seat_winners');       // 201: this customer got the seat
const conflicts = new Counter('seat_conflicts');   // 409: seat already taken
const unexpected = new Counter('unexpected_responses');

export const options = {
  scenarios: {
    one_seat_race: {
      // One VU per customer, one iteration each: exactly USERS holds, all started together.
      executor: 'per-vu-iterations',
      vus: USERS,
      iterations: 1,
      maxDuration: '60s',
    },
  },
  thresholds: {
    seat_winners: ['count==1'],
    seat_conflicts: [`count==${USERS - 1}`],
    unexpected_responses: ['count==0'],
    'http_req_duration{name:hold}': ['p(95)<1000'],
  },
};

export function setup() {
  // Pick a future showtime with at least one free seat; that seat is the contested one.
  const now = encodeURIComponent(new Date().toISOString());
  const shows = http.get(`${BASE}/api/v1/showtimes?page_size=20&from=${now}`).json('data') || [];
  let target = null;
  for (const show of shows) {
    const seats = http.get(`${BASE}/api/v1/showtimes/${show.id}/seats`).json('seats') || [];
    const free = seats.find((s) => s.status === 'AVAILABLE');
    if (free) {
      target = {
        showtimeId: show.id,
        seatId: free.id,
        seatLabel: `${free.row_label}${free.seat_number}`,
        // Earlier runs and manual testing may have occupied other seats; count them so the check is relative.
        occupiedBefore: seats.filter((s) => s.status !== 'AVAILABLE').length,
      };
      break;
    }
  }
  if (!target) {
    throw new Error('no upcoming showtime with a free seat; run `make seed` or free a seat');
  }

  // One registered customer per request, so every hold comes from a different user.
  const registrations = [];
  for (let i = 0; i < USERS; i++) {
    registrations.push([
      'POST',
      `${BASE}/api/v1/users`,
      JSON.stringify({ email: `k6-${RUN_ID}-${i}@loadtest.local`, full_name: `Load User ${i}` }),
      { headers: JSON_HEADERS },
    ]);
  }
  const users = http.batch(registrations).map((r, i) => {
    if (r.status !== 201) {
      throw new Error(`user ${i} registration failed: ${r.status} ${r.body}`);
    }
    return r.json('id');
  });

  console.log(`contested seat ${target.seatLabel} in showtime ${target.showtimeId}, ${users.length} customers`);
  return { ...target, users };
}

export default function (data) {
  const user = data.users[exec.vu.idInTest - 1];
  const res = http.post(
    `${BASE}/api/v1/showtimes/${data.showtimeId}/holds`,
    JSON.stringify({ seat_ids: [data.seatId] }),
    { headers: { ...JSON_HEADERS, 'X-User-ID': user }, tags: { name: 'hold' } },
  );

  switch (res.status) {
    case 201:
      winners.add(1);
      break;
    case 409:
      conflicts.add(1);
      break;
    default:
      unexpected.add(1);
      console.error(`hold returned ${res.status}: ${res.body}`);
  }
}

// After the run, the database must show the seat held by exactly one booking.
export function teardown(data) {
  const seats = http.get(`${BASE}/api/v1/showtimes/${data.showtimeId}/seats`).json('seats') || [];
  const seat = seats.find((s) => s.id === data.seatId);
  const occupiedAfter = seats.filter((s) => s.status !== 'AVAILABLE').length;
  check(seat, {
    [`seat ${data.seatLabel} is HELD after the race`]: (s) => s !== undefined && s.status === 'HELD',
    'exactly one more seat is occupied than before the race': () => occupiedAfter === data.occupiedBefore + 1,
  });
}
