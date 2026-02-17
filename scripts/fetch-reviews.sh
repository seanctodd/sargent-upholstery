#!/usr/bin/env bash
# Fetch 5-star Google reviews for Sargent Upholstery Co.
# Appends new non-duplicate reviews (>=20 words) to data/reviews.json
#
# Uses both the Places API (New) and legacy Places API with different
# sort orders to maximize the number of unique reviews collected.
set -euo pipefail

PLACE_ID="ChIJvQmmCSS35YgR-3H9ajzGCHk"
API_KEY="${GOOGLE_API_KEY:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATA_FILE="$SCRIPT_DIR/../data/reviews.json"
MIN_WORDS=20

if [ -z "$API_KEY" ]; then
  echo "GOOGLE_API_KEY not set, skipping review fetch"
  exit 0
fi

# Initialize data file if missing
if [ ! -f "$DATA_FILE" ]; then
  echo '[]' > "$DATA_FILE"
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Fetch from Places API (New) — returns most relevant reviews
curl -sf "https://places.googleapis.com/v1/places/$PLACE_ID" \
  -H "X-Goog-Api-Key: $API_KEY" \
  -H "X-Goog-FieldMask: reviews" \
  -o "$TMPDIR/new.json" 2>/dev/null || echo '{}' > "$TMPDIR/new.json"

# Fetch from legacy Places API — most relevant
curl -sf "https://maps.googleapis.com/maps/api/place/details/json?place_id=$PLACE_ID&fields=reviews&reviews_sort=most_relevant&key=$API_KEY" \
  -o "$TMPDIR/legacy_relevant.json" 2>/dev/null || echo '{}' > "$TMPDIR/legacy_relevant.json"

# Fetch from legacy Places API — newest
curl -sf "https://maps.googleapis.com/maps/api/place/details/json?place_id=$PLACE_ID&fields=reviews&reviews_sort=newest&key=$API_KEY" \
  -o "$TMPDIR/legacy_newest.json" 2>/dev/null || echo '{}' > "$TMPDIR/legacy_newest.json"

# Process and merge all reviews
python3 "$SCRIPT_DIR/process-reviews.py" "$DATA_FILE" "$MIN_WORDS" \
  "$TMPDIR/new.json" "$TMPDIR/legacy_relevant.json" "$TMPDIR/legacy_newest.json"
