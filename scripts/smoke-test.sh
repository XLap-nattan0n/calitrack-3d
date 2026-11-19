#!/usr/bin/env bash
# Smoke test for a deployed CaliTrack preview. All config comes from the environment; no values live in this file.
#   API_BASE_URL              required, Go API preview origin
#   WEB_BASE_URL              optional, frontend preview origin (also checks the /api/v1 rewrite)
#   VERCEL_PROTECTION_BYPASS  optional, Vercel deployment protection bypass secret
#   SUPABASE_URL, SUPABASE_ANON_KEY, SMOKE_EMAIL, SMOKE_PASSWORD
#                             optional, enable the authenticated checks; use a dedicated test user
set -uo pipefail

API_BASE_URL="${API_BASE_URL:?set API_BASE_URL to the API preview origin}"
API_BASE_URL="${API_BASE_URL%/}"
WEB_BASE_URL="${WEB_BASE_URL:-}"
WEB_BASE_URL="${WEB_BASE_URL%/}"

for bin in curl jq; do
  command -v "$bin" >/dev/null || { echo "missing dependency: $bin" >&2; exit 2; }
done

body_file=$(mktemp)
TOKEN=""
plan_id=""
session_id=""
passed=0
failed=0
skipped=0

pass() { echo "PASS  $1"; passed=$((passed + 1)); }
fail() { echo "FAIL  $1 -- $2"; failed=$((failed + 1)); }
skip() { echo "SKIP  $1 -- $2"; skipped=$((skipped + 1)); }

bypass_args=()
if [ -n "${VERCEL_PROTECTION_BYPASS:-}" ]; then
  bypass_args=(-H "x-vercel-protection-bypass: $VERCEL_PROTECTION_BYPASS")
fi

# request METHOD URL [curl args...] sets STATUS and leaves the response body in $body_file.
request() {
  local method=$1 url=$2
  shift 2
  STATUS=$(curl -sS -X "$method" --max-time 30 -o "$body_file" -w '%{http_code}' \
    ${bypass_args[@]+"${bypass_args[@]}"} "$@" "$url") || STATUS=000
}

describe() {
  if grep -q "Vercel Authentication" "$body_file" 2>/dev/null; then
    echo "blocked by Vercel deployment protection; set VERCEL_PROTECTION_BYPASS"
  else
    head -c 200 "$body_file" | tr '\n' ' '
  fi
}

# expect NAME WANT_STATUS METHOD URL [curl args...]
expect() {
  local name=$1 want=$2
  shift 2
  request "$@"
  if [ "$STATUS" = "$want" ]; then
    pass "$name ($STATUS)"
  else
    fail "$name" "want $want, got $STATUS: $(describe)"
  fi
}

cleanup() {
  if [ -n "$TOKEN" ] && [ -n "$session_id" ]; then
    request POST "$API_BASE_URL/api/v1/sessions/$session_id/finish" \
      -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{}'
    echo "cleanup: finish smoke session ($STATUS)"
  fi
  if [ -n "$TOKEN" ] && [ -n "$plan_id" ]; then
    request DELETE "$API_BASE_URL/api/v1/plans/$plan_id" -H "Authorization: Bearer $TOKEN"
    echo "cleanup: delete smoke plan ($STATUS)"
  fi
  rm -f "$body_file"
}
trap cleanup EXIT

echo "== API: $API_BASE_URL"
request GET "$API_BASE_URL/api/v1/health"
if [ "$STATUS" = 200 ] && jq -e '.status == "ok"' "$body_file" >/dev/null 2>&1; then
  pass "api health (200, status ok)"
else
  fail "api health" "want 200 with status ok, got $STATUS: $(describe)"
fi
expect "api rejects anonymous /me" 401 GET "$API_BASE_URL/api/v1/me"

if [ -n "$WEB_BASE_URL" ]; then
  echo "== Web: $WEB_BASE_URL"
  expect "web /login" 200 GET "$WEB_BASE_URL/login"
  expect "web 3D muscle model asset" 200 GET "$WEB_BASE_URL/models/anatomy-muscles.glb"
  request GET "$WEB_BASE_URL/anatomy"
  if [[ "$STATUS" =~ ^(200|30[1-8])$ ]] && ! grep -q "Vercel Authentication" "$body_file"; then
    pass "web /anatomy responds ($STATUS; 3xx means redirected to login)"
  else
    fail "web /anatomy" "got $STATUS: $(describe)"
  fi
  expect "web proxies /api/v1/health to API" 200 GET "$WEB_BASE_URL/api/v1/health"
fi

echo "== Auth"
if [ -n "${SUPABASE_URL:-}" ] && [ -n "${SUPABASE_ANON_KEY:-}" ] && [ -n "${SMOKE_EMAIL:-}" ] && [ -n "${SMOKE_PASSWORD:-}" ]; then
  login_json=$(jq -cn --arg e "$SMOKE_EMAIL" --arg p "$SMOKE_PASSWORD" '{email: $e, password: $p}')
  request POST "${SUPABASE_URL%/}/auth/v1/token?grant_type=password" \
    -H "apikey: $SUPABASE_ANON_KEY" -H "Content-Type: application/json" -d "$login_json"
  TOKEN=$(jq -r '.access_token // empty' "$body_file" 2>/dev/null)
  if [ -n "$TOKEN" ]; then
    pass "supabase password login ($STATUS)"
  else
    fail "supabase password login" "got $STATUS, no access_token: $(describe)"
  fi
else
  skip "authenticated checks" "set SUPABASE_URL, SUPABASE_ANON_KEY, SMOKE_EMAIL, SMOKE_PASSWORD"
fi

if [ -n "$TOKEN" ]; then
  auth=(-H "Authorization: Bearer $TOKEN")
  today=$(date -u +%F)

  echo "== Authenticated flow"
  expect "me" 200 GET "$API_BASE_URL/api/v1/me" "${auth[@]}"

  exercise_id=""
  request GET "$API_BASE_URL/api/v1/exercises" "${auth[@]}"
  if [ "$STATUS" = 200 ] && exercise_id=$(jq -er '.items[0].id' "$body_file" 2>/dev/null); then
    pass "exercise library lists exercises (200)"
  else
    fail "exercise library" "got $STATUS: $(describe)"
  fi

  expect "planner: list plans" 200 GET "$API_BASE_URL/api/v1/plans?from=$today&to=$today" "${auth[@]}"

  if [ -n "$exercise_id" ]; then
    plan_json=$(jq -cn --arg d "$today" --arg ex "$exercise_id" \
      '{plan_date: $d, title: "smoke-test plan", category: "custom", exercises: [{exercise_id: $ex, target_sets: 3, target_reps: 8, rest_seconds: 60}]}')
    request POST "$API_BASE_URL/api/v1/plans" "${auth[@]}" -H "Content-Type: application/json" -d "$plan_json"
    if [ "$STATUS" = 201 ] && plan_id=$(jq -er '.id' "$body_file" 2>/dev/null); then
      pass "planner: create plan (201)"
    else
      fail "planner: create plan" "got $STATUS: $(describe)"
    fi
  else
    skip "planner: create plan" "no exercise id available"
  fi

  if [ -n "$plan_id" ]; then
    expect "planner: get plan" 200 GET "$API_BASE_URL/api/v1/plans/$plan_id" "${auth[@]}"

    session_json=$(jq -cn --arg p "$plan_id" '{plan_id: $p}')
    request POST "$API_BASE_URL/api/v1/sessions" "${auth[@]}" -H "Content-Type: application/json" -d "$session_json"
    if [ "$STATUS" = 201 ] && session_id=$(jq -er '.id' "$body_file" 2>/dev/null); then
      pass "session: start from plan (201)"
    else
      fail "session: start from plan" "got $STATUS: $(describe)"
    fi
  else
    skip "session checks" "no plan created"
  fi

  if [ -n "$session_id" ]; then
    expect "session: active (resume)" 200 GET "$API_BASE_URL/api/v1/sessions/active" "${auth[@]}"
    expect "session: get by id" 200 GET "$API_BASE_URL/api/v1/sessions/$session_id" "${auth[@]}"
    expect "session: finish" 200 POST "$API_BASE_URL/api/v1/sessions/$session_id/finish" "${auth[@]}" \
      -H "Content-Type: application/json" -d '{}'
    session_id=""
  fi

  expect "analytics: summary" 200 GET "$API_BASE_URL/api/v1/analytics/summary" "${auth[@]}"
  expect "analytics: muscle frequency" 200 GET "$API_BASE_URL/api/v1/analytics/muscle-frequency" "${auth[@]}"
  expect "records" 200 GET "$API_BASE_URL/api/v1/records" "${auth[@]}"
fi

echo
echo "Result: $passed passed, $failed failed, $skipped skipped"
if [ "$failed" -gt 0 ]; then
  exit 1
fi
