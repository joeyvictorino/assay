#!/usr/bin/env sh
# dvwa-setup.sh [BASE_URL]
# curl-based fallback for the DVWA adapter's Setup: creates/resets the
# database through /setup.php (with its CSRF token), logs in as the
# documented admin account and leaves a cookie jar with security=low at
# $DVWA_COOKIE_JAR (default ./dvwa-cookies.txt). Loopback targets only.
set -eu
BASE="${1:-http://127.0.0.1:8080}"
JAR="${DVWA_COOKIE_JAR:-./dvwa-cookies.txt}"
case "$BASE" in
  http://127.0.0.1:*|http://localhost:*) ;;
  *) echo "dvwa-setup: refusing non-loopback base url $BASE" >&2; exit 2 ;;
esac
rm -f "$JAR"
token() { sed -n "s/.*name='user_token' value='\([0-9a-f]*\)'.*/\1/p; s/.*name=\"user_token\" value=\"\([0-9a-f]*\)\".*/\1/p" | head -n1; }

echo "dvwa-setup: fetching setup token"
T=$(curl -s -c "$JAR" -b "$JAR" --max-time 20 "$BASE/setup.php" | token)
[ -n "$T" ] || { echo "dvwa-setup: no user_token on setup.php" >&2; exit 1; }
echo "dvwa-setup: creating database"
curl -s -o /dev/null -c "$JAR" -b "$JAR" --max-time 60 \
  --data-urlencode "create_db=Create / Reset Database" --data-urlencode "user_token=$T" "$BASE/setup.php"

echo "dvwa-setup: logging in as admin"
T=$(curl -s -c "$JAR" -b "$JAR" --max-time 20 "$BASE/login.php" | token)
[ -n "$T" ] || { echo "dvwa-setup: no user_token on login.php" >&2; exit 1; }
# -b must name the jar file alone; appending a literal cookie turns the
# argument into a cookie string and drops the PHPSESSID the token is bound to.
LOC=$(curl -s -o /dev/null -w '%{redirect_url}' -c "$JAR" -b "$JAR" --max-time 20 \
  --data-urlencode "username=admin" --data-urlencode "password=password" \
  --data-urlencode "Login=Login" --data-urlencode "user_token=$T" "$BASE/login.php")
case "$LOC" in
  *index.php*) ;;
  *) echo "dvwa-setup: login failed (redirect: ${LOC:-none})" >&2; exit 1 ;;
esac
# Pin the security level in the jar (Netscape cookie format).
HOST=$(printf '%s' "$BASE" | sed 's#^http://##; s#:.*##; s#/.*##')
printf '%s\tFALSE\t/\tFALSE\t0\tsecurity\tlow\n' "$HOST" >> "$JAR"
echo "dvwa-setup: ok (cookie jar: $JAR)"
