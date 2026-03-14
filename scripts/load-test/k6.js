import http from 'k6/http';
import { sleep, check } from 'k6';

export const options = {
  vus: 100,
  duration: '1m',
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
  },
};

const productIds = ['1', '2', '3'];

export default function () {
  const productId = productIds[Math.floor(Math.random() * productIds.length)];

  const payload = JSON.stringify({
    user_id: `user-${Math.floor(Math.random() * 100000)}`,
    event: 'view_product',
    product_id: productId,
    timestamp: new Date().toISOString(),
  });

  const headers = { 'Content-Type': 'application/json' };

  const res = http.post('http://localhost:8082/events', payload, { headers });
  check(res, {
    'status is 202': (r) => r.status === 202,
  });

  http.get(`http://localhost:8081/products/${productId}`);
  sleep(0.1);
}
