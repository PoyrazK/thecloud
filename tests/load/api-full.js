import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '30s', target: 20 }, // ramp up to 20 users
        { duration: '1m', target: 20 },  // stay at 20 users
        { duration: '30s', target: 0 },  // ramp down
    ],
    thresholds: {
        http_req_failed: ['rate<0.01'],
        http_req_duration: ['p(95)<500'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
    // 1. Health check
    const healthRes = http.get(`${BASE_URL}/health`);
    check(healthRes, {
        'health is 200': (r) => r.status === 200,
    });

    // 2. Login (This might fail if no user exists, in a real test we'd seed data)
    const loginPayload = JSON.stringify({
        email: 'admin@thecloud.local',
        password: 'Password123!',
    });
    const params = {
        headers: {
            'Content-Type': 'application/json',
        },
    };
    const loginRes = http.post(`${BASE_URL}/auth/login`, loginPayload, params);

    // We don't always expect login to succeed in a load test unless env is prepared
    // But we check it if we are testing the auth flow
    check(loginRes, {
        'login status is 200 or 401': (r) => [200, 401].includes(r.status),
    });

    let apiKey = '';
    if (loginRes.status === 200) {
        apiKey = loginRes.json('api_key');
    } else {
        // Fallback to an env var if login fails (e.g. for testing only authenticated routes)
        apiKey = __ENV.API_KEY || 'test-api-key';
    }

    const authParams = {
        headers: {
            'X-API-Key': apiKey,
        },
    };

    // 3. List Instances
    const instancesRes = http.get(`${BASE_URL}/instances`, authParams);
    check(instancesRes, {
        'instances status is 200 or 401': (r) => [200, 401].includes(r.status),
    });

    // 4. Dashboard summary
    const dashRes = http.get(`${BASE_URL}/api/dashboard/summary`, authParams);
    check(dashRes, {
        'dashboard status is 200 or 401': (r) => [200, 401].includes(r.status),
    });

    sleep(1);
}
