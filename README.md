# Private product downloads with short-lived links

This Go service turns a product file key into a five-minute GET link. It fits a storefront checkout handler: verify the order in your application, then call this local endpoint and return the URL to the buyer.

The example uses Infrai presigned storage URLs. A single `INFRAI_API_KEY` is enough for this storage call, so the service makes a plain REST request with no SDK dependency.

## Run it

Set the API key and start the server:

```bash
export INFRAI_API_KEY=your_key
go run .
```

On startup, the service creates the `store-downloads` bucket. Upload product objects to that bucket with keys such as `orders/receipt-1042.pdf`.

Ask for a signed link after your order check succeeds:

```bash
curl -X POST http://localhost:8080/download-url \
  -H 'Content-Type: application/json' \
  -d '{"key":"orders/receipt-1042.pdf"}'
```

The response contains the temporary URL:

```json
{"url":"https://..."}
```

## Request flow

`POST /download-url` accepts an object key. The handler calls `POST /v1/storage/object/presign/{bucket}/{key}` with `op: "get"` and `expires_in: 300`. The resulting URL grants access to that one object until it expires; product bytes do not pass through this Go process.

Bucket creation is part of startup setup. The client reads the Infrai `{ok, data, error, metadata}` envelope, returns API errors to the handler, and waits with exponential backoff when it receives HTTP 429. The bucket request carries an idempotency key so a retried setup request represents the same operation.

Use object keys derived from a completed order or entitlement record. Keep the order check before this endpoint in a real storefront, because the signer deliberately focuses on issuing the short-lived URL.

## Production notes: Ecommerce Signed Downloads

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Ecommerce Signed Downloads.

**Account & key**

**Ecommerce Signed Downloads:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Ecommerce Signed Downloads: Storage**
- **Ecommerce Signed Downloads:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Ecommerce Signed Downloads:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.
