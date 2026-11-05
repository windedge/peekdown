# Issue tracker: GitHub

Issues and specs for this repo live in GitHub Issues on `windedge/peekdown`. Use the `gh` CLI for all operations.

## Conventions

- Create: `gh issue create --title "..." --body-file <file>` (use a temp file for multi-line bodies).
- Read: `gh issue view <n> --json number,title,body,labels,comments`.
- List: `gh issue list --state open --json number,title,body,labels,comments`.
- Sub-issue: `gh issue create --parent <map> ...` or `gh issue edit <map> --add-sub-issue <child>`.
- Comment: `gh issue comment <n> --body-file <file>`.
- Label: `gh issue edit <n> --add-label "..."` / `--remove-label "..."`.
- Close: `gh issue close <n>`.
- Assignee is the claim: `gh issue edit <n> --add-assignee @me`.

## Wayfinding operations

- **Map**: a single issue labelled `wayfinder:map`, holding Destination / Notes / Decisions so far / Not yet specified / Out of scope.
- **Child ticket**: a GitHub sub-issue of the map, labelled `wayfinder:<type>` (`research`, `prototype`, `grilling`, `task`). The type lives in the label, not the body.
- **Blocking**: GitHub native issue dependencies.
  - Add: `gh api --method POST repos/windedge/peekdown/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`
  - Read: `gh api repos/windedge/peekdown/issues/<n>/dependencies/blocked_by`
  - The `<blocker-db-id>` is the database id (`gh api repos/windedge/peekdown/issues/<n> --jq .id`), not the issue number.
  - Fallback if the API is unavailable: a `Blocked by: #<n>` line at the top of the child body.
- **Frontier**: open children of the map with no open blocker and no assignee; lowest issue number wins.
- **Claim**: assign to yourself as the first write of the session.
- **Resolve**: comment the answer, close the issue, append one line (name + link + gist) to the map's Decisions so far.

## Research tickets

Resolved on a throwaway `research/<name>` branch: findings go to `docs/research/<name>.md`, the branch is pushed and never merged, and the ticket comment points at the file and commit.
