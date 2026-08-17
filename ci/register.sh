#!/usr/bin/env bash
set -Eeuo pipefail

: "${MODULE_ID:?MODULE_ID is required}"
: "${LIVESHOP_RELEASE_VERSION:?LIVESHOP_RELEASE_VERSION is required}"
: "${BACKEND_ORIGIN:?BACKEND_ORIGIN is required}"
: "${PLATFORM_REGISTRY_URL:?PLATFORM_REGISTRY_URL is required}"
: "${ARTIFACT_URLS:?ARTIFACT_URLS is required}"
: "${KERNEL_ROOT:?KERNEL_ROOT is required}"
: "${WORKLOAD_PRIVATE_KEY:?WORKLOAD_PRIVATE_KEY is required}"
: "${WORKLOAD_KEY_ID:?WORKLOAD_KEY_ID is required}"
: "${WORKLOAD_ISSUER:?WORKLOAD_ISSUER is required}"
: "${WORKLOAD_SUBJECT:?WORKLOAD_SUBJECT is required}"
: "${WORKLOAD_AUDIENCE:?WORKLOAD_AUDIENCE is required}"

manifest_source="$1/business/module.json"
manifest_file="$(mktemp)"
artifact_file="$(mktemp)"
release_response="$(mktemp)"
activation_response="$(mktemp)"
trap 'rm -f "$manifest_file" "$artifact_file" "$release_response" "$activation_response"' EXIT

resolved_artifacts='{}'
while IFS= read -r surface; do
  entry="$(jq -r --arg surface "$surface" '.[$surface]' <<<"$ARTIFACT_URLS")"
  curl -fsS --retry 20 --retry-delay 1 "$entry" -o "$artifact_file"
  integrity="sha256:$(sha256sum "$artifact_file" | awk '{print $1}')"
  resolved_artifacts="$(jq -c     --arg surface "$surface"     --arg entry "$entry"     --arg integrity "$integrity"     '. + {($surface): {entry: $entry, integrity: $integrity}}'     <<<"$resolved_artifacts")"
done < <(jq -r 'keys[]' <<<"$ARTIFACT_URLS")

jq   --arg version "$LIVESHOP_RELEASE_VERSION"   --arg origin "$BACKEND_ORIGIN"   --arg grpc "$GRPC_ENDPOINT"   --argjson artifacts "$resolved_artifacts"   '
    .metadata.version = $version
    | .spec.backend.origin = $origin
    | if ($grpc != "" and .spec.backend.grpc != null)
      then .spec.backend.grpc.endpoint = $grpc
      else .
      end
    | .spec.contributions |= map(
        ($artifacts[.surface] // error("missing artifact for surface " + .surface)) as $artifact
        | .artifact.version = $version
        | .artifact.entry = $artifact.entry
        | .artifact.integrity = $artifact.integrity
      )
  ' "$manifest_source" > "$manifest_file"

workload_token="$(
  docker run --rm     -e WORKLOAD_PRIVATE_KEY     -e WORKLOAD_KEY_ID     -e WORKLOAD_ISSUER     -e WORKLOAD_SUBJECT     -e WORKLOAD_AUDIENCE     -v "$KERNEL_ROOT:/src:ro"     -w /src     golang:1.25-alpine     go run ./cmd/workloadtoken
)"

release_status="$(curl -sS -o "$release_response" -w '%{http_code}'   --header "Authorization: Bearer $workload_token"   --header 'Content-Type: application/json; charset=utf-8'   --data-binary "@$manifest_file"   "$PLATFORM_REGISTRY_URL/releases")"
if [ "$release_status" -lt 200 ] || [ "$release_status" -ge 300 ]; then
  printf 'Release registration failed with HTTP %s: ' "$release_status" >&2
  jq -c '{code,message,data}' "$release_response" >&2 || cat "$release_response" >&2
  exit 1
fi
test "$(jq -r '.code' "$release_response")" = "0"

activation_status="$(jq -nc   --arg moduleId "$MODULE_ID"   --arg version "$LIVESHOP_RELEASE_VERSION"   '{moduleId: $moduleId, version: $version}'   | curl -sS -o "$activation_response" -w '%{http_code}'       --header "Authorization: Bearer $workload_token"       --header 'Content-Type: application/json; charset=utf-8'       --data-binary @-       "$PLATFORM_REGISTRY_URL/activate")"
if [ "$activation_status" -lt 200 ] || [ "$activation_status" -ge 300 ]; then
  printf 'Release activation failed with HTTP %s: ' "$activation_status" >&2
  jq -c '{code,message,data}' "$activation_response" >&2 || cat "$activation_response" >&2
  exit 1
fi
test "$(jq -r '.code' "$activation_response")" = "0"

printf 'Activated module %s release %s\n' "$MODULE_ID" "$LIVESHOP_RELEASE_VERSION"
