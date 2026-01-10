import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '2m', target: 50 },  // ramp up
        { duration: '4h', target: 50 },  // soak for 4 hours (reduced for demo/ci to 5m usually, but 4h is real soak)
        { duration: '2m', target: 0 },   // ramp down
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],
        // p(95) duration should stay stable over time
        http_req_duration: ['p(95)<300'],
    },
};

// For CI we might want a shorter version
if (__ENV.CI) {
    options.stages = [
        { duration: '1m', target: 20 },
        { duration: '5m', target: 20 },
        { duration: '1m', target: 0 },
    ];
}

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'test-api-key';

export default function () {
    const params = {
        headers: {
            'X-API-Key': API_KEY,
        },
    };

    const res = http.get(`${BASE_URL}/api/dashboard/summary`, params);
    check(res, { 'status is 200': (r) => r.status === 200 });

    sleep(2);
}
