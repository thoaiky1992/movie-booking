import http from 'k6/http';
import { check, fail } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const vus = positiveInteger('VUS', 200);
const iterationsPerVU = positiveInteger('ITERATIONS_PER_VU', 1);
const totalIterations = vus * iterationsPerVU;
const p95LimitMilliseconds = positiveInteger('P95_MS', 1000);
const p99LimitMilliseconds = positiveInteger('P99_MS', 2000);

const winners = new Counter('hot_seat_winners');
const conflicts = new Counter('hot_seat_conflicts');
const unexpectedResponses = new Counter('hot_seat_unexpected_responses');
const validOutcomes = new Rate('hot_seat_valid_outcomes');
const holdDuration = new Trend('hot_seat_hold_duration', true);
const expectedHoldStatuses = http.expectedStatuses(201, 409);

export const options = {
  scenarios: {
    hot_seat_contention: {
      executor: 'per-vu-iterations',
      vus,
      iterations: iterationsPerVU,
      maxDuration: __ENV.MAX_DURATION || '30s',
      gracefulStop: '0s',
    },
  },
  thresholds: {
    // An initially available seat must have exactly one winner, never two.
    hot_seat_winners: ['count==1'],
    hot_seat_unexpected_responses: ['count==0'],
    hot_seat_valid_outcomes: ['rate==1'],
    iterations: [`count==${totalIterations}`],
    'http_req_failed{scenario:hot_seat_contention}': ['rate==0'],
    'http_req_duration{scenario:hot_seat_contention}': [
      `p(95)<${p95LimitMilliseconds}`,
      `p(99)<${p99LimitMilliseconds}`,
    ],
  },
};

export function setup() {
  const baseURL = (__ENV.BASE_URL || 'http://localhost:8080').replace(/\/$/, '');
  const showtimeID = requiredPositiveInteger('SHOWTIME_ID');
  const seatID = requiredPositiveInteger('SEAT_ID');
  const userIDs = (__ENV.USER_IDS || '1,2,3,4,5')
    .split(',')
    .map((value) => Number.parseInt(value.trim(), 10));

  if (userIDs.length === 0 || userIDs.some((id) => !Number.isInteger(id) || id <= 0)) {
    fail('USER_IDS must be a comma-separated list of positive integers');
  }

  const response = http.get(`${baseURL}/api/v1/showtimes/${showtimeID}/seats`, {
    tags: { operation: 'preflight_seat_map' },
  });
  if (response.status !== 200) {
    fail(`seat-map preflight failed: status=${response.status}, body=${response.body}`);
  }

  let body;
  try {
    body = response.json();
  } catch (error) {
    fail(`seat-map preflight returned invalid JSON: ${error}`);
  }

  const seats = (body.data?.rows || []).flatMap((row) => row.seats || []);
  const targetSeat = seats.find((seat) => seat.id === seatID);
  if (!targetSeat) {
    fail(`seat ${seatID} does not belong to showtime ${showtimeID}`);
  }
  if (targetSeat.status !== 'AVAILABLE') {
    fail(`seat ${seatID} must be AVAILABLE before the test; current status=${targetSeat.status}`);
  }

  console.log(
    `Hot-seat contention: ${totalIterations} requests, ${vus} VUs, ` +
      `showtime=${showtimeID}, seat=${seatID}`,
  );
  return { baseURL, showtimeID, seatID, userIDs };
}

export default function (data) {
  const userID = data.userIDs[(__VU - 1) % data.userIDs.length];
  const response = http.post(
    `${data.baseURL}/api/v1/showtimes/${data.showtimeID}/holds`,
    JSON.stringify({ seat_ids: [data.seatID] }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-User-ID': String(userID),
        'X-Trace-ID': `k6-hot-seat-${__VU}-${__ITER}-${Date.now()}`,
      },
      responseCallback: expectedHoldStatuses,
      tags: { operation: 'hold_hot_seat' },
      timeout: __ENV.REQUEST_TIMEOUT || '10s',
    },
  );

  holdDuration.add(response.timings.duration);
  let validOutcome = false;
  let responseBody;
  try {
    responseBody = response.json();
  } catch (_) {
    responseBody = null;
  }

  if (response.status === 201) {
    const heldSeats = responseBody?.data?.seat_ids || [];
    validOutcome =
      responseBody?.success === true &&
      responseBody?.data?.status === 'PENDING' &&
      heldSeats.length === 1 &&
      heldSeats[0] === data.seatID;
    if (validOutcome) {
      winners.add(1);
      console.log(
        `WINNER booking_id=${responseBody.data.booking_id} ` +
          `user_id=${userID} seat_id=${data.seatID}`,
      );
    }
  } else if (response.status === 409) {
    validOutcome =
      responseBody?.success === false && responseBody?.error?.code === 'SEATS_UNAVAILABLE';
    if (validOutcome) {
      conflicts.add(1);
    }
  }

  validOutcomes.add(validOutcome);
  unexpectedResponses.add(!validOutcome, { status: String(response.status) });
  if (!validOutcome) {
    console.error(
      `UNEXPECTED status=${response.status} user_id=${userID} ` +
        `body=${String(response.body).slice(0, 500)}`,
    );
  }
  check(response, {
    'response is one valid winner or a seat conflict': () => validOutcome,
  });
}

function positiveInteger(name, fallback) {
  if (__ENV[name] === undefined || __ENV[name] === '') {
    return fallback;
  }
  const value = Number.parseInt(__ENV[name], 10);
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error(`${name} must be a positive integer`);
  }
  return value;
}

function requiredPositiveInteger(name) {
  if (__ENV[name] === undefined || __ENV[name] === '') {
    throw new Error(`${name} is required`);
  }
  return positiveInteger(name, 0);
}
