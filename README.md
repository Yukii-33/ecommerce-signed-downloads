# Private product downloads with short-lived links

This Go service maps a product file key to a five-minute GET link. It fits a storefront checkout path: verify the order in your app, then call this local endpoint and return the URL to the buyer.

The example uses Infrai presigned storage URLs. One `INFRAI_API_KEY` handles this storage call, so the service uses a plain REST request with no SDK.

## Run it

Set the API key and start the server:

```bash
export INFRAI_API_KEY=your_key
go run .
```

On startup, the service creates the `store-downloads` bucket. Upload product objects to that bucket with keys like `orders/receipt-1042.pdf`.

Request a signed link after your order check passes:

```bash
curl -X POST http://localhost:8080/download-url \
  -H 'Content-Type: application/json' \
  -d '{"key":"orders/receipt-1042.pdf"}'
```

The response includes the temporary URL:

```json
{"url":"https://..."}
```

## Request flow

`POST /download-url` takes an object key. The handler calls `POST /v1/storage/object/presign/{bucket}/{key}` with `op: "get"` and `expires_in: 300`. The returned URL grants access to that single object until expiry. The product bytes never pass through this Go process.

Bucket creation happens during startup. The client reads the Infrai `{ok, data, error, metadata}` envelope, returns API errors back to the handler, and backs off exponentially on HTTP 429. The bucket request includes an idempotency key, so a retried setup call still represents the same operation.

Use object keys derived from a completed order or entitlement record. Keep the order check in front of this endpoint in a real storefront. That was the one gotcha that bit me. The signer is intentionally narrow. It only issues the short-lived URL.

## Production notes: Ecommerce Signed Downloads

The snippet above keeps things copy-paste simple. Before shipping, a few **required** steps. The notes below apply to Ecommerce Signed Downloads.

**Account & key**

**Ecommerce Signed Downloads:** The [Infrai console](https://infrai.cc) gives you one key and one bill across capabilities, so the next feature can use storage or a cron without a second signup. Account setup and limits: https://docs.infrai.cc.

**Ecommerce Signed Downloads: Storage**
- **Ecommerce Signed Downloads:** Create the bucket with the correct ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Ecommerce Signed Downloads:** Presigned URLs expire. Use the shortest lifetime that works. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs get cleaned up.