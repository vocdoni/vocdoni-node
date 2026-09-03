An **Organization** is an account with an `infoUri` that contains organization-associated  metadata. 
An **Account** could be a validator, an oracle, a voter or just someone who wants to transfer tokens. 

The `/chain/organizations` endpoints are related only to the Organization account type.

- Return list of organizations ids.
- If no page is defined, will assume page 0.

### Sorting

`sortBy` picks the ordering and `order` its direction. Both are optional, and an
unsupported value is rejected with a 400 rather than ignored.

| `sortBy` | orders by | default `order` |
| --- | --- | --- |
| `createdAt` (default) | when the organization first appeared in the index, i.e. the creation time of its oldest indexed election | `desc`, newest organizations first |
| `electionCount` | how many elections the organization has | `desc`, busiest organizations first |
| `name` | the organization name resolved from its account metadata, case-insensitively (ASCII case folding only). Organizations whose account resolves no name always sort last, in either direction | `asc`, alphabetical |

The ordering is total: organizations that tie are ordered by their id, so paging
through a sorted list never repeats nor skips an organization. Sorting composes
with the `organizationId` and `name` filters and with `page`/`limit`, so a
ranking such as the top five organizations by election count is a single
`?sortBy=electionCount&order=desc&limit=5` request.

Omitting `sortBy` keeps the ordering this endpoint had before it took one.
