# Spec Delta

## ADDED Requirements

### Requirement: Guarded vault archive reading
The import SHALL accept a zip of at most 100 MB and reject archives whose entries use absolute paths, drive letters or `..` segments, hold more than 2000 entries, expand beyond 500 MB counted while reading, or exceed a 100:1 compression ratio. Only `.md`/`.markdown` notes and png, jpeg, gif, webp, pdf or text attachments that pass content sniffing SHALL be read; SVG and every other file SHALL be skipped and reported.

#### Scenario: Zip slip
- **WHEN** an archive contains an entry named `../evil.md`
- **THEN** the upload is rejected and nothing is staged

#### Scenario: Zip bomb
- **WHEN** an entry declares a compression ratio above 100:1
- **THEN** the upload is rejected

### Requirement: Vault to doc mapping
Each note SHALL become a human doc titled by front-matter `title:`, else its first H1, else its filename, with YAML front-matter stripped. Each folder SHALL become a parent doc whose content comes from `index.md`, `README.md` or `<folder>.md`, or is empty. Front-matter `connector:` SHALL match a connector by ID or name; a connector doc under a lab-scoped folder SHALL be placed at its connector's root with a warning.

#### Scenario: Nested vault
- **WHEN** a vault holds `Servers/index.md` and `Servers/Proxmox.md`
- **THEN** the preview shows a `Servers` doc with index content and a `Proxmox` child

### Requirement: Link and embed rewriting
Image and file embeds (`![[x.png]]`, `![[x.png|300]]`, relative `![](…)`, `![[file.pdf]]`) SHALL become doc attachments referenced as `attachment:<id>`. Wikilinks (`[[Note]]`, `[[Note|alias]]`, `[[Note#Heading]]`) and relative `.md` links SHALL become `/docs/<id>` using Obsidian basename-then-shortest-path resolution. Ambiguous or unresolved links SHALL be left as written and reported.

#### Scenario: Ambiguous wikilink
- **WHEN** two notes in sibling folders share a basename and a third note links to it from elsewhere
- **THEN** the link is left as text and the preview reports it as ambiguous

### Requirement: Preview then atomic commit
`POST /api/docs/import` SHALL stage the archive and return a preview with the doc tree, link mappings, attachment count, warnings, skipped files and title collisions. `POST /api/docs/import/{id}/commit` SHALL create every doc and attachment in one transaction with origin `human` and version trigger `import`, suffix a title that collides with a sibling in the same parent with " (imported)", and sync embeddings afterwards. Both routes SHALL require an instance admin. Staged imports SHALL expire after one hour and be removed by a sweep job.

#### Scenario: Title collision
- **WHEN** a lab root doc titled `Network` exists and the vault holds `Network.md` at its root
- **THEN** the imported doc is titled `Network (imported)`

#### Scenario: Non-admin
- **WHEN** a non-admin calls either import route
- **THEN** the response is 403

#### Scenario: Expired staging
- **WHEN** an import is committed more than one hour after staging
- **THEN** the commit returns 404 and the sweep removes its files

### Requirement: Import dialog
The docs page SHALL offer admins an Import dialog that uploads a zip, shows the preview tree with warnings, skipped files and collisions, commits on confirmation and navigates to the first imported doc.

#### Scenario: Confirm import
- **WHEN** an admin uploads a vault and confirms the preview
- **THEN** the docs are created and the browser opens the first imported doc
