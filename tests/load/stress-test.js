import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '1m', target: 50 },  // below normal load
        { duration: '2m', target: 100 }, // normal load
        { duration: '2m', target: 200 }, // around breaking point
        { duration: '2m', target: 300 }, // beyond breaking point
        { duration: '2m', target: 0 },   // scale down
    ],
    thresholds: {
        http_req_failed: ['rate<0.05'], // allow up to 5% failure during stress test
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'test-api-key';

export default function () {
    const params = {
        headers: {
            'X-API-Key': API_KEY,
        },
    };

    // Stress the main listing endpoints
    const responses = http.batch([
        ['GET', `${BASE_URL}/instances`, null, params],
        ['GET', `${BASE_URL}/vpcs`, null, params],
        ['GET', `${BASE_URL}/databases`, null, params],
    ]);

    check(responses[0], { 'instances ok': (r) => r.status === 200 });

    sleep(0.5); // faster polling during stress
}
