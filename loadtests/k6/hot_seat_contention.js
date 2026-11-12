import http from 'k6/http';
import { check, fail } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const vus = positiveInteger('VUS', 200);
const duration = __ENV.DURATION || '10s';
const p95LimitMilliseconds = positiveInteger('P95_MS', 1000);
const p99LimitMilliseconds = positiveInteger('P99_MS', 2000);

const winners = new Counter('hot_seat_winners');
const conflicts = new Counter('hot_seat_conflicts');
const unexpectedResponses = new Counter('hot_seat_unexpected_responses');
const validOutcomes = new Rate('hot_seat_valid_outcomes');
const bookingSucceeded = new Rate('hot_seat_booking_succeeded');
const holdDuration = new Trend('hot_seat_hold_duration', true);
const expectedHoldStatuses = http.expectedStatuses(201, 409);

export const options = {
  scenarios: {
    hot_seat_contention: {
      executor: 'constant-vus',
      vus,
      duration,
      gracefulStop: '10s',
    },
  },
  thresholds: {
    // A reused seat group can have no winner, but it must never have two winners.
    hot_seat_winners: ['count<=1'], // at most one winner
    hot_seat_unexpected_responses: ['count==0'], // no unexpected responses allowed
    hot_seat_valid_outcomes: ['rate==1'], // all responses must be valid
    iterations: ['count>0'], // at least one request must run
    'http_req_failed{scenario:hot_seat_contention}': ['rate==0'], // no failed requests allowed
    'http_req_duration{scenario:hot_seat_contention}': [
      `p(95)<${p95LimitMilliseconds}`, // 95th percentile must be below the limit
      `p(99)<${p99LimitMilliseconds}`, // 99th percentile must be below the limit
    ],
  },
};

export function setup() {
  const baseURL = (__ENV.BASE_URL || 'http://localhost:8080').replace(/\/$/, '');
  const showtimeID = requiredPositiveInteger('SHOWTIME_ID');
  const seatIDs = requiredPositiveIntegerList('SEAT_IDS', 10);
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
  const seatsByID = new Map(seats.map((seat) => [seat.id, seat]));
  const missingSeatIDs = seatIDs.filter((seatID) => !seatsByID.has(seatID));
  if (missingSeatIDs.length > 0) {
    fail(`seats ${missingSeatIDs.join(',')} do not belong to showtime ${showtimeID}`);
  }

  const targetSeats = seatIDs.map((seatID) => seatsByID.get(seatID));
  const initialStatuses = targetSeats
    .map((seat) => `${seat.id}:${seat.status}`)
    .join(',');
  console.log(
    `Hot-seat contention: continuous requests for ${duration}, ${vus} VUs, ` +
      `showtime=${showtimeID}, seats=${seatIDs.join(',')}, ` +
      `initial_statuses=${initialStatuses}`,
  );
  const unavailableSeats = targetSeats.filter((seat) => seat.status !== 'AVAILABLE');
  if (unavailableSeats.length > 0) {
    console.warn(
      `Seats already unavailable: ${unavailableSeats
        .map((seat) => `${seat.id}:${seat.status}`)
        .join(',')}; requests will still run and no booking winner is expected.`,
    );
  }
  return { baseURL, showtimeID, seatIDs, userIDs };
}

export default function (data) {
  const userID = data.userIDs[(__VU - 1) % data.userIDs.length];
  const response = http.post(
    `${data.baseURL}/api/v1/showtimes/${data.showtimeID}/holds`,
    JSON.stringify({ seat_ids: data.seatIDs }),
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

  let wonSeat = false;
  if (response.status === 201) {
    const heldSeats = responseBody?.data?.seat_ids || [];
    validOutcome =
      responseBody?.success === true &&
      responseBody?.data?.status === 'PENDING' &&
      sameSeatIDs(heldSeats, data.seatIDs);
    if (validOutcome) {
      wonSeat = true;
      winners.add(1);
      console.log(
        `WINNER booking_id=${responseBody.data.booking_id} ` +
          `user_id=${userID} seat_ids=${data.seatIDs.join(',')}`,
      );
    }
  } else if (response.status === 409) {
    validOutcome =
      responseBody?.success === false && responseBody?.error?.code === 'SEATS_UNAVAILABLE';
    if (validOutcome) {
      conflicts.add(1);
    }
  }

  // Recording every request keeps both metrics visible in the summary even
  // when a reused seat produces zero winners.
  if (!wonSeat) {
    winners.add(0);
  }
  bookingSucceeded.add(wonSeat);
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

function requiredPositiveIntegerList(name, maxItems) {
  if (__ENV[name] === undefined || __ENV[name] === '') {
    throw new Error(`${name} is required`);
  }

  const values = __ENV[name].split(',').map((value) => Number(value.trim()));
  if (
    values.length === 0 ||
    values.length > maxItems ||
    values.some((value) => !Number.isInteger(value) || value <= 0) ||
    new Set(values).size !== values.length
  ) {
    throw new Error(`${name} must contain 1 to ${maxItems} unique positive integers`);
  }
  return values.sort((left, right) => left - right);
}

function sameSeatIDs(actual, expected) {
  return (
    Array.isArray(actual) &&
    actual.length === expected.length &&
    expected.every((seatID) => actual.includes(seatID))
  );
}
