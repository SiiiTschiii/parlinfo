# Contacts Package

Maps politicians to their social media accounts, and tags them in posts.

One curated file per jurisdiction — `data/zurich-city/contacts.yaml`,
`data/zurich-canton/contacts.yaml`, `data/nationalrat/contacts.yaml` and
`data/staenderat/contacts.yaml` — with an identical schema: a name, and the
accounts verified for it. The Ständerat's is kept but read by nothing yet: no
source serves its votes. A channel serving more than one body loads them
together (`LoadContactFiles`).

Tagging is name matching: the mapper looks for a contact's name in the text of a
post and appends their handle. Two things follow from that.

- A post that names nobody tags nobody. Stadt Zürich's titles carry their
  submitters, so the city has always worked; Kanton Zürich's and the
  Nationalrat's do not, and their authorship is put back onto the label line by
  the formatters — see `votes.Affair.Authors`. A Nationalrat post can name a
  Ständerat member who filed the business, so it loads both files.
- An entry with no handles is valid and useful. It costs nothing, and it marks
  a member as known-but-not-yet-found.

## Verified accounts

An account is posted only once it carries `verified: true` — a human opened it
and recognised the person. `Contact.Verified(platform)` is the single gate every
reader goes through, so an unverified handle cannot reach a post by someone
forgetting to check a flag.

That gate is what lets candidates live in the same file as confirmed handles.
Curating 180 cantonal members is weeks of one-by-one checking; the alternative
was keeping the leads somewhere the tooling could not see, which meant the work
could not start until it was finished.

Every account states it, in one shape:

```yaml
  - name: Anna Graff
    x:
      - url: https://x.com/annagraff
        verified: true
      - url: https://x.com/maybe        # a candidate: not posted
        verified: false
        confidence: high
```

There is deliberately no shape that means "verified" without saying so. A bare
URL string — what the mapping used before candidates existed — still parses, as
*unverified*, so a stale file degrades to posting without tags rather than
failing a run; `cmd/validate_contacts` rejects it loudly instead. The defaults
therefore fall the safe way round: publishing takes a deliberate act.

`confidence` is an ordering aid on unverified entries only, and says how well a
search result's page title matched the name — never evidence of identity. Drop
it when you confirm the account; the validator requires that.

Party and Fraktion are deliberately not stored here. The parliaments publish
both, so a copy would only go stale; `cmd/generate_search_urls` reads them live
for the curation work, and `cmd/update_contacts` refreshes the names from the
body's own roster.

The mapping is only worth having if every handle in it is right: a wrong one
puts a real person's account next to a vote they did not cast. That is why the
handles are verified by hand and `cmd/validate_contacts` guards the rest.
