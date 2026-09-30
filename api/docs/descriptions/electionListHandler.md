Get a list of election summaries, filtered, sorted and paginated.

- If no page is defined, will assume page 0.
- Dates are RFC3339 or `YYYY-MM-DD`, and date bounds are inclusive.
- `title` matches a case-insensitive substring of the title resolved from the
  election metadata (ASCII case folding only). Elections whose metadata title was
  never resolved never match.

### Sorting

`sortBy` picks the ordering and `order` its direction. Both are optional, and an
unsupported value is rejected with a 400 rather than ignored.

| `sortBy` | orders by | default `order` |
| --- | --- | --- |
| `createdAt` (default) | when the election was created | `desc`, newest first |
| `startDate` | when voting opens | `desc`, most recently opened first |
| `endDate` | when voting closes | `desc`, latest closing first |
| `voteCount` | how many votes were cast | `desc`, most voted first |
| `title` | the title resolved from the election metadata, case-insensitively (ASCII case folding only). Elections without a resolved title always sort last, in either direction | `asc`, alphabetical |

The ordering is total: elections that tie are ordered by their id, so paging
through a sorted list never repeats nor skips an election. Sorting composes with
every filter and with `page`/`limit`, so the five most voted elections of an
organization are a single
`?organizationId=...&sortBy=voteCount&limit=5` request.

Omitting `sortBy` orders by `createdAt` descending, as before.
