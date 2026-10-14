# LLM Wiki: Operational Prompts (Ingest, Query, Lint, Targeted Draft)

Based on Karpathy's llm-wiki pattern: https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f

Each of the four prompts below is self-contained. Only the fenced code
block is meant to be copy-pasted into a session, nothing outside it should
be required for the agent to understand what to do. For a new wiki, paste
the prompt alone. For an existing wiki, paste the prompt together with the
zip file from the previous session.

---

## 1. INGEST prompt

Use when a new source is provided.

```
You maintain a personal wiki: markdown pages (entity pages, concept pages,
summary pages, one per ingested source), plus index.md (a catalog of every
page) and log.md (an append-only chronological log). Every page carries
this frontmatter:
---
type: entity | concept | source
created: YYYY-MM-DD
updated: YYYY-MM-DD
sources: N
---
The wiki has no storage between chat sessions, it travels as a zip file.

You are ingesting a new source into this wiki.

Required steps, in order:

0. Load the wiki
   If a zip file of the wiki was provided, unpack it, this is the current
   state to build on. If none was provided, start from an empty wiki.

1. Read
   Read the entire source before summarizing anything.

2. Clarify
   Identify anything in the source that is ambiguous, incomplete, or
   requires context only the user can provide (an acronym, a name, an
   implicit situation). You may find several such points, but ask them
   one at a time, never as a batch list. Wait for the answer before asking
   the next question. This step exists to resolve genuine gaps in the
   source, not to get approval for what you're about to write. If the
   source has no ambiguity, skip this step entirely.

3. Summary page
   Create a page dedicated to this source (date, type, origin, faithful
   summary). This page is not the source itself, the original stays as
   provided, untouched. The summary page is a wiki artifact like any
   other, its role is provenance: every entity/concept page this source
   touches will link back to it.

4. Integrate into existing pages
   Determine yourself, from the source's content, which pages this source
   is relevant to, whether they already exist or need to be created. No
   category is fixed in advance, a source can surface a page type that
   doesn't exist yet (an employer, a project, a skill, a contact, a
   review, or anything else the source actually contains). Creating a new
   page is not a fallback for when an existing one doesn't fit, it's an
   equally normal outcome of this step.

   Updating a page is a revision, not an append. Once updated, the page
   must read as a complete, current synthesis of its subject, folding the
   new information into what was already there, not stacking a new
   paragraph at the end. Note in the recap whether this source reinforces
   or challenges what the page said before. This is what keeps the wiki
   as a whole current without ever needing to be re-derived from raw
   sources.

   - link to the summary page from every page you update
   - if a relevant page doesn't exist yet, create it directly, and note
     in the recap why this page type is justified by what you read
   - if a new recurring page type emerges that future sources might also
     need, mention it in the recap, but don't build out a general
     structure ahead of need, a category exists only because a page
     needed it

5. Flag contradictions
   If the new source contradicts something already stated on an existing
   page, never silently overwrite the old claim. Write both versions on
   the page, each with its source and date, and flag the contradiction to
   the user explicitly.

6. Cross-references
   Verify every page you touched links to the other pages it's related
   to, real markdown links in the file, not just links mentioned in
   conversation.

7. Update index.md
   Add or update the entry for every page created or modified (link,
   one-line summary, category, date).

8. Update log.md
   Append an entry in this exact format so the log stays grep-able:
   ## [YYYY-MM-DD] ingest | <source title>
   List pages created, pages modified, and any contradictions flagged.

9. Recap
   End with a clear list: pages created, pages modified, contradictions
   flagged. This is a record of what was done, not a list awaiting
   sign-off.

10. Package and deliver
   Zip the full current state of the wiki (every page, index.md, log.md)
   into a single file and deliver it to the user. This zip is the only
   thing that carries the wiki into the next session.
```

---

## 2. QUERY prompt

Use when a question is asked against the existing wiki.

```
You maintain a personal wiki: markdown pages (entity pages, concept pages,
summary pages, one per ingested source), plus index.md (a catalog of every
page) and log.md (an append-only chronological log). Every page carries
this frontmatter:
---
type: entity | concept | source
created: YYYY-MM-DD
updated: YYYY-MM-DD
sources: N
---
The wiki has no storage between chat sessions, it travels as a zip file.

You are answering a question using only this wiki, not general knowledge
or assumptions.

Required steps:

0. Wait for the question
   This prompt runs against a specific question. If the message carrying
   this prompt doesn't also contain the question, wait for it, do not
   load or read anything yet.

1. Load the wiki
   If a zip file of the wiki was provided, unpack it and use it as the
   source of truth for this query. If none was provided, say so, there is
   no wiki to query yet.

2. Retrieve with qmd instead of reading everything
   Do not read the entire wiki manually. Use qmd
   (https://github.com/tobi/qmd, npm install -g @tobilu/qmd), a ready-made
   document index:
   - add the wiki as a collection, indexing every markdown file
     recursively:
     qmd collection add <wiki_dir> --name wiki
   - query it with the question, using pure keyword search:
     qmd search "<question>" -c wiki --format files

   Never run qmd embed, qmd vsearch, or qmd query, those require
   downloading models. qmd search alone is pure BM25 keyword matching, no
   model involved.

   If qmd cannot be installed or run in this environment, fall back to
   reading index.md to locate potentially relevant pages instead.

3. Targeted reading
   Open only the pages qmd returned (or, in the fallback case, the pages
   identified from index.md). If they aren't enough to answer the
   question, follow their cross-references to related pages, or run
   additional qmd searches with different keyword phrasings drawn from
   the question.

4. Coverage check
   If no page in the wiki covers the question, say so explicitly. Never
   fill a gap with a guess.

5. Answer with citations
   Answer citing the specific pages used (page name, and the underlying
   source when relevant). If pages disagree on the topic, present both
   versions with their dates instead of picking one unilaterally.

   Choose the output format that fits the question: a short answer, a
   table, a timeline, whichever actually serves the question best.

6. File the answer
   If the answer produces a synthesis or analysis that doesn't exist
   anywhere in the wiki yet, file it directly as a new page or as an
   addition to an existing page. A good answer should not disappear into
   chat history, it compounds into the wiki exactly like an ingested
   source would.

7. Update log.md
   If a new page was created from this question, append:
   ## [YYYY-MM-DD] query | <question, summarized>

8. Package and deliver
   Only if step 6 changed the wiki: zip its full current state and
   deliver it to the user, same as after an ingest. If nothing was filed,
   no new zip is needed, the wiki did not change.
```

---

## 3. LINT prompt

Use periodically, not every session, to check the health of the wiki and fix what can be fixed directly.

```
You maintain a personal wiki: markdown pages (entity pages, concept pages,
summary pages, one per ingested source), plus index.md (a catalog of every
page) and log.md (an append-only chronological log). The wiki has no
storage between chat sessions, it travels as a zip file.

You are running a health check on this wiki. Fixing what you find is part
of maintaining the wiki, same as ingest or query, you write directly, you
do not just produce a report and wait.

0. Load the wiki
   If a zip file of the wiki was provided, unpack it. If none was
   provided, there is nothing to lint.

Check for, and fix directly:

1. Stale claims
   Claims a newer source has superseded, where the page was never
   updated to reflect it. Revise the page.

2. Orphan pages
   Pages with no inbound link from any other page in the wiki. Add the
   missing links.

3. Missing pages
   Entities or concepts that recur across multiple pages but don't have
   their own page yet. Create them.

4. Missing cross-references
   Pages that should logically link to each other and don't. Add the
   links.

5. Data gaps
   Topics where a web search or an additional source would meaningfully
   fill a gap. Note these, do not fabricate content to fill them.

Check for, but do not resolve:

6. Contradictions
   Pages holding contradictory claims. Two claims that cannot both hold
   is a judgment call, not something to settle unilaterally. List them
   with both sides, do not pick a winner.

7. Suggestions
   Propose new questions worth investigating and new sources worth
   looking for, based on what the wiki currently contains and where it's
   thin.

Report every fix applied, every unresolved contradiction, and every
suggestion.

End with an entry to append to log.md:
## [YYYY-MM-DD] lint | <fixes applied, open contradictions, suggestions>

Package and deliver an updated zip if any fix was applied. If nothing
needed fixing, no new zip is needed.
```

---

## 4. TARGETED DRAFT prompt (e.g. a CV against a job posting)

Use when the goal is not a question about the wiki, but a document to
produce against an external brief (a job posting, a call for applications),
grounded in the wiki. Provide the brief and the wiki zip together with this
prompt.

```
You maintain a personal wiki: markdown pages (entity pages, concept pages,
summary pages, one per ingested source), plus index.md (a catalog of every
page) and log.md (an append-only chronological log). The wiki has no
storage between chat sessions, it travels as a zip file.

You are drafting a document (e.g. a CV) targeted at an external brief (e.g.
a job posting), grounded only in the wiki, never in general assumptions
about the user.

Required steps, in order:

0. Load the wiki
   Unpack the zip file into a working directory. If none was provided,
   there is no wiki to draft from yet.

1. Read the brief
   Read the external target document in full before touching the wiki.
   Extract its explicit requirements (skills, experience, seniority,
   tools) and its implicit ones (tone, domain vocabulary, what it
   emphasizes through ordering or repetition).

2. Retrieve with qmd instead of reading everything
   Do not load the entire wiki into context. Use qmd
   (https://github.com/tobi/qmd, npm install -g @tobilu/qmd), a ready-made
   document index: add the wiki once, then query it.

   - add the unpacked wiki as a collection, indexing every markdown file
     recursively:
     qmd collection add <wiki_dir> --name wiki

   - run one query per requirement identified in step 1, using pure
     keyword search only:
     qmd search "<requirement>" -c wiki --format files

   Never run qmd embed, qmd vsearch, or qmd query, those require
   downloading models. qmd search alone is pure BM25 keyword matching, no
   model involved.

   Read the full content only of the files returned this way, not the
   rest of the wiki. One query per requirement, never one combined query
   for everything, a combined query blurs which requirement each match
   actually supports.

3. Identify gaps
   For every requirement from the brief that has no supporting match in
   the retrieved pages, note it as a gap. Never fabricate content to fill
   it. Ask the user about each gap one at a time, never as a batch list,
   and wait for the answer before asking about the next gap.

4. Draft
   Write the document using only the retrieved, grounded content, in the
   brief's own vocabulary and priorities. Before including any claim, you
   must be able to point to the specific wiki page that supports it, this
   is an internal check against fabrication, it does not appear in the
   document itself. The delivered document must read as a normal,
   standalone CV: no mention of the wiki, no citations, no source
   references, nothing that would look out of place to the person reading
   it.

5. Deliver
   Hand back the drafted document as a file, clean of any trace of the
   wiki or its internal structure. It is a one-off artifact for this
   specific application, not persistent professional knowledge, so it is
   not filed into the wiki.

6. Optional: file a genuine insight
   If preparing this draft revealed a real, reusable pattern about the
   user's profile that isn't captured anywhere in the wiki yet (not the
   draft itself, an actual new synthesis), propose filing it as a new
   page or as a revision to an existing one. If the user accepts:
   - write the page as a full, current synthesis of its subject, not an
     appended note
   - link it to the pages it relates to
   - update index.md with its entry
   - append to log.md:
     ## [YYYY-MM-DD] draft | <brief description>
   - package the full current state of the wiki into a zip and deliver it
```
