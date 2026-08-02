import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

// Custom metrics to track allowed vs rate-limited requests
const allowedRequests = new Counter('allowed_requests_200');
const rateLimitedRequests = new Counter('rate_limited_requests_429');

export const options = {
    scenarios: {
        spike_test: {
            executor: 'per-vu-iterations',
            vus: 50,              // 50 concurrent virtual users
      iterations: 1,        // Each VU sends 1 request at the exact same moment
      maxDuration: '10s',
        },
    },
};

export default function () {
    const url = 'http://localhost:8000/hello';
    const params = {
        headers: {
            'X-API-Key': 'free-key',
        },
    };

    const res = http.get(url, params);

    if (res.status === 200) {
        allowedRequests.add(1);
    } else if (res.status === 429) {
        rateLimitedRequests.add(1);
    }

    check(res, {
        'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    });
}
