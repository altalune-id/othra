#!/bin/sh
# Prepares the compose Garage once per start: layout, bucket, the fixed dev key and the bucket CORS rule. Safe to rerun.
set -eu

admin="${GARAGE_ADMIN_URL:-http://garage:3903}"
s3="${GARAGE_S3_URL:-http://garage:3900}"
token="${GARAGE_ADMIN_TOKEN:?}"
bucket="${GARAGE_BUCKET:?}"
key_id="${GARAGE_KEY_ID:?}"
key_secret="${GARAGE_KEY_SECRET:?}"
origins="${GARAGE_CORS_ORIGINS:?}"

api() {
  curl -fsS -H "Authorization: Bearer ${token}" -H 'Content-Type: application/json' "$@"
}

tries=0
until status=$(api "${admin}/v2/GetClusterStatus" 2>/dev/null); do
  tries=$((tries + 1))
  if [ "$tries" -ge 60 ]; then
    echo "garage-init: admin API not reachable at ${admin}" >&2
    exit 1
  fi
  sleep 1
done

if printf '%s' "$status" | grep -q '"role": null'; then
  node=$(printf '%s' "$status" | sed -n 's/.*"id": "\([0-9a-f]\{64\}\)".*/\1/p' | head -n 1)
  version=$(printf '%s' "$status" | sed -n 's/.*"layoutVersion": \([0-9]*\).*/\1/p')
  api -X POST "${admin}/v2/UpdateClusterLayout" \
    -d "{\"roles\":[{\"id\":\"${node}\",\"zone\":\"dc1\",\"capacity\":1000000000,\"tags\":[]}]}" >/dev/null
  api -X POST "${admin}/v2/ApplyClusterLayout" -d "{\"version\":$((version + 1))}" >/dev/null
  echo "garage-init: layout applied"
fi

if ! info=$(api "${admin}/v2/GetBucketInfo?globalAlias=${bucket}" 2>/dev/null); then
  info=$(api -X POST "${admin}/v2/CreateBucket" -d "{\"globalAlias\":\"${bucket}\"}")
  echo "garage-init: bucket ${bucket} created"
fi
bucket_id=$(printf '%s' "$info" | sed -n 's/.*"id": "\([0-9a-f]\{64\}\)".*/\1/p' | head -n 1)

if ! api "${admin}/v2/GetKeyInfo?id=${key_id}" >/dev/null 2>&1; then
  api -X POST "${admin}/v2/ImportKey" \
    -d "{\"accessKeyId\":\"${key_id}\",\"secretAccessKey\":\"${key_secret}\",\"name\":\"othra-dev\"}" >/dev/null
  echo "garage-init: key ${key_id} imported"
fi
api -X POST "${admin}/v2/AllowBucketKey" \
  -d "{\"bucketId\":\"${bucket_id}\",\"accessKeyId\":\"${key_id}\",\"permissions\":{\"read\":true,\"write\":true,\"owner\":true}}" >/dev/null

api -X POST "${admin}/v2/UpdateBucket?id=${bucket_id}" \
  -d '{"websiteAccess":{"enabled":true,"indexDocument":"index.html","errorDocument":null}}' >/dev/null

allowed=""
for origin in $origins; do
  allowed="${allowed}<AllowedOrigin>${origin}</AllowedOrigin>"
done
cors="<CORSConfiguration><CORSRule>${allowed}<AllowedMethod>PUT</AllowedMethod><AllowedMethod>GET</AllowedMethod>"
cors="${cors}<AllowedMethod>HEAD</AllowedMethod><AllowedHeader>content-type</AllowedHeader><ExposeHeader>etag</ExposeHeader>"
cors="${cors}<MaxAgeSeconds>3600</MaxAgeSeconds></CORSRule></CORSConfiguration>"
curl -fsS -X PUT --aws-sigv4 "aws:amz:garage:s3" --user "${key_id}:${key_secret}" \
  -H 'Content-Type: application/xml' --data-binary "$cors" "${s3}/${bucket}?cors" >/dev/null
echo "garage-init: bucket ${bucket} ready, CORS for ${origins}"
