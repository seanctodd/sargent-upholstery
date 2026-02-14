#!/usr/bin/env bash
# Fetch 5-star Google reviews for Sargent Upholstery Co.
# Appends new non-duplicate reviews (>=20 words) to data/reviews.json
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

# Fetch reviews from Google Places API (New)
RESPONSE=$(curl -sf "https://places.googleapis.com/v1/places/$PLACE_ID" \
  -H "X-Goog-Api-Key: $API_KEY" \
  -H "X-Goog-FieldMask: reviews" 2>/dev/null || echo '{}')

if ! echo "$RESPONSE" | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null; then
  echo "Invalid API response, skipping"
  exit 0
fi

# Initialize data file if missing
if [ ! -f "$DATA_FILE" ]; then
  echo '[]' > "$DATA_FILE"
fi

# Process reviews: filter 5-star, >=20 words, append non-duplicates
python3 - "$RESPONSE" "$DATA_FILE" "$MIN_WORDS" <<'PYEOF'
import json, sys

response = json.loads(sys.argv[1])
data_file = sys.argv[2]
min_words = int(sys.argv[3])

# Load existing reviews
with open(data_file, 'r') as f:
    existing = json.load(f)

existing_ids = {r['id'] for r in existing}

new_reviews = []
for review in response.get('reviews', []):
    if review.get('rating', 0) != 5:
        continue
    text = review.get('text', {}).get('text', '')
    if len(text.split()) < min_words:
        continue
    review_id = review.get('name', '')
    if review_id in existing_ids:
        continue
    new_reviews.append({
        'id': review_id,
        'author': review.get('authorAttribution', {}).get('displayName', 'Anonymous'),
        'rating': review['rating'],
        'text': text,
        'date': review.get('publishTime', ''),
        'relativeTime': review.get('relativePublishTimeDescription', ''),
        'googleMapsUri': review.get('googleMapsUri', '')
    })

if new_reviews:
    existing.extend(new_reviews)
    # Sort by date descending, keep most recent 20
    existing.sort(key=lambda r: r.get('date', ''), reverse=True)
    existing = existing[:20]
    with open(data_file, 'w') as f:
        json.dump(existing, f, indent=2)
    print(f"Added {len(new_reviews)} new review(s). Total: {len(existing)}")
else:
    print(f"No new reviews. Total: {len(existing)}")
PYEOF
