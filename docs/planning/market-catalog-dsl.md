# Instance Market Catalog Entries

`market_profiles.catalogSource` optionally expands one inventory template into
named market catalog entries. Ordinary commodities and profiles without this
field keep their existing behavior.

```json
{
  "catalogKeys": [{ "source": "instance", "path": "/fishData/speciesId", "compare": "exact" }],
  "catalogSource": { "table": "fishing_species", "namesPath": "/names", "valuePaths": ["/id"] }
}
```

- `table` names a registered game-data table. Disabled rows are excluded.
- `namesPath` reads localized display names using a JSON pointer.
- `valuePaths` reads stable nonempty string identifiers, one per `catalogKeys`
  selector. Selectors must be exact instance fields. Duplicate tuples, unsafe
  paths and missing source tables/values fail the data build.
- Entries exist even when there are no offers. The inventory picker applies
  the same selectors, while server quoting and purchase validation stay authoritative.
- The groups API accepts `catalogValues`, a JSON string array. The server
  validates its shape and hashes it with the existing catalog identity function;
  filtering happens in SQL before pagination. Do not pass both `catalogValues`
  and `catalogKey`. Invalid filters are rejected rather than widened.
- A catalog entry is not an inventory template or a same-stat offer group.
  `groupKeys` and `uniqueByInstance` still control offer identity independently.
- Catalog presentation does not enter the grouping revision. Existing listings,
  original instances and hidden metadata remain intact. No schema migration is
  required for this addition.

Deploy the updated backend with rebuilt game data before relying on scoped
pagination in the frontend. Older servers ignore the new query parameter; the
frontend hides mismatched groups defensively but cannot repair their pagination.
