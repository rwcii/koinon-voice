#!/bin/sh
# Commits and pull requests carry the maintainer's identity and no tool attribution
# (WDR 004, Decision 5). Usage: check-attribution.sh FILE... checks message text;
# check-attribution.sh --identity "Name <email>"... checks commit identities.
status=0
if [ "$1" = --identity ]; then
  shift
  for who in "$@"; do
    case "$(printf '%s' "$who" | tr '[:upper:]' '[:lower:]')" in
      *claude*|*anthropic*)
        echo "Blocked: commit identity is not the maintainer's: $who" >&2
        status=1
        ;;
    esac
  done
  exit "$status"
fi
pattern='^[[:space:]]*(co-authored-by|claude-session):|generated with \[?claude|claude\.ai/code|noreply@anthropic\.com'
for file in "$@"; do
  if grep -Eiq "$pattern" "$file"; then
    echo "Blocked: tool attribution in $file:" >&2
    grep -Ei "$pattern" "$file" >&2
    status=1
  fi
done
exit "$status"
