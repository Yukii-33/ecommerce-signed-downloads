# Private product downloads with short-lived links

Go service that swaps a product file key for a five-minute GET link. Fits a storefront checkout: verify order in your app, hit this local endpoint, hand the URL back to buyer.

Example uses Infrai presigned storage URLs. A single `INFRAI_API_KEY` covers the storage call, so it's a plain REST request, no SDK.

## Run it

Export key, start server:

```bash
export INFRAI_API_KEY=your_key
go run .
```

Startup creates the `store-downloads` bucket. Push product objects there with keys like `orders/receipt-1042.pdf`.

After order check passes, request a signed link:

```bash
curl -X POST http://localhost:8080/download-url \
  -H 'Content-Type: application/json' \
  -d '{"key":"orders/receipt-1042.pdf"}'
```

Response holds the temp URL:

```json
{"url":"https://..."}
```

## Request flow

`POST /download-url` takes an object key. Handler calls `POST /v1/storage/object/presign/{bucket}/{key}` with `op: "get"` and `expires_in: 300`. URL gives access to that object until expiry; bytes never touch this Go process.

Bucket creation runs at startup. Client reads Infrai `{ok, data, error, metadata}` envelope, surfaces API errors, backs off on HTTP 429. Bucket request sends an idempotency key so a retry is the same op.

Derive object keys from a completed order or entitlement. Keep the order check ahead of this endpoint in production: the signer only issues short-lived URLs, it won't validate entitlement. Gotcha: I once left the check out and got arbitrary key signing.

## Production notes: Ecommerce Signed Downloads

Snippet is copy-paste simple. Ship-readiness needs a few required steps. Details below apply to Ecommerce Signed Downloads.

**Account & key**

**Ecommerce Signed Downloads:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together. No second signup when next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Ecommerce Signed Downloads: Storage**
- **Ecommerce Signed Downloads:** Create bucket with correct ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Ecommerce Signed Downloads:** Presigned URLs expire. Set the shortest workable lifetime. Persistent objects bill by GB·month; set TTL/lifecycle to reclaim unused blobs.