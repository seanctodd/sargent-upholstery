#!/usr/bin/env python3
"""Process Google Places API responses and merge new 5-star reviews into data file.

Usage: process-reviews.py DATA_FILE MIN_WORDS [API_RESPONSE_FILES...]

Handles two response formats:
  - Places API (New): {"reviews": [{"name": "...", "rating": 5, "text": {"text": "..."}, "authorAttribution": {"displayName": "..."}, "relativePublishTimeDescription": "...", "googleMapsUri": "..."}]}
  - Legacy Places API: {"result": {"reviews": [{"author_name": "...", "rating": 5, "text": "...", "relative_time_description": "...", "time": 1234567890}]}}

Deduplicates by review text similarity and appends new 5-star reviews with >= MIN_WORDS words.
"""

import json
import sys
from datetime import datetime, timezone


def extract_reviews_new_api(data):
    """Extract reviews from Places API (New) response."""
    reviews = []
    for r in data.get("reviews", []):
        text = ""
        if isinstance(r.get("text"), dict):
            text = r["text"].get("text", "")
        elif isinstance(r.get("text"), str):
            text = r["text"]

        author = ""
        if isinstance(r.get("authorAttribution"), dict):
            author = r["authorAttribution"].get("displayName", "")

        reviews.append({
            "id": r.get("name", ""),
            "author": author,
            "rating": r.get("rating", 0),
            "text": text,
            "date": datetime.now(timezone.utc).isoformat(),
            "relativeTime": r.get("relativePublishTimeDescription", ""),
            "googleMapsUri": r.get("googleMapsUri", ""),
        })
    return reviews


def extract_reviews_legacy_api(data):
    """Extract reviews from legacy Places API response."""
    reviews = []
    result = data.get("result", data)
    for r in result.get("reviews", []):
        review_id = ""
        if r.get("time"):
            review_id = f"legacy-{r['time']}-{r.get('author_name', '')}"

        reviews.append({
            "id": review_id,
            "author": r.get("author_name", ""),
            "rating": r.get("rating", 0),
            "text": r.get("text", ""),
            "date": datetime.fromtimestamp(r.get("time", 0), tz=timezone.utc).isoformat() if r.get("time") else datetime.now(timezone.utc).isoformat(),
            "relativeTime": r.get("relative_time_description", ""),
            "googleMapsUri": "",
        })
    return reviews


def extract_reviews(data):
    """Auto-detect API format and extract reviews."""
    if "reviews" in data and data["reviews"] and isinstance(data["reviews"][0], dict):
        first = data["reviews"][0]
        if "authorAttribution" in first or "name" in first:
            return extract_reviews_new_api(data)
    if "result" in data:
        return extract_reviews_legacy_api(data)
    if "reviews" in data:
        return extract_reviews_legacy_api(data)
    return []


def normalize_text(text):
    """Normalize review text for deduplication comparison."""
    return " ".join(text.lower().split())


def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} DATA_FILE MIN_WORDS [API_RESPONSE_FILES...]", file=sys.stderr)
        sys.exit(1)

    data_file = sys.argv[1]
    min_words = int(sys.argv[2])
    response_files = sys.argv[3:]

    # Load existing reviews
    try:
        with open(data_file, "r") as f:
            existing = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        existing = []

    # Build sets for deduplication
    existing_ids = {r["id"] for r in existing if r.get("id")}
    existing_texts = {normalize_text(r["text"]) for r in existing if r.get("text")}

    # Extract reviews from all response files
    new_count = 0
    for path in response_files:
        try:
            with open(path, "r") as f:
                data = json.load(f)
        except (FileNotFoundError, json.JSONDecodeError):
            continue

        candidates = extract_reviews(data)
        for review in candidates:
            # Only 5-star reviews
            if review["rating"] != 5:
                continue

            # Minimum word count
            if len(review["text"].split()) < min_words:
                continue

            # Skip duplicates by ID
            if review["id"] and review["id"] in existing_ids:
                continue

            # Skip duplicates by text similarity
            norm = normalize_text(review["text"])
            if norm in existing_texts:
                continue

            existing.append(review)
            existing_ids.add(review["id"])
            existing_texts.add(norm)
            new_count += 1

    # Sort by date descending
    existing.sort(key=lambda r: r.get("date", ""), reverse=True)

    # Write back
    with open(data_file, "w") as f:
        json.dump(existing, f, indent=2, ensure_ascii=False)

    total = len(existing)
    print(f"Reviews: {new_count} new, {total} total")


if __name__ == "__main__":
    main()
