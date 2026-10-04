# Lab journal

Open **Journal** in the sidebar or command palette to see lab activity in time
order. Changes, alerts, document edits and notes appear alongside failed syncs
and syncs that produced changes or alerts. Enable **Show all sync runs** to include
quiet successful runs. Filter by connector, source or UTC date range, and use
**Load more** to continue through older activity. **View source** opens the related
change, alert, document, connector or Audit page.

Choose **New entry** to add a Markdown note. Set **Occurred at** in your local time
to record past maintenance where it belongs among automated events. Choose a
connector, optionally pick an entity from its latest snapshot and link a document
in the same scope. Entity kind, name and external reference are retained as text,
so context survives even when the entity disappears. The preview shows Markdown
before you save.

Connector operators can create notes in their connectors. Instance admins can
create lab-wide notes, which all signed-in members can read. Authors and admins
can edit or delete entries they can view; moving an entry requires write access
in the destination scope. Deletion asks for confirmation and is permanent.

Notes are included in backups and kept indefinitely. Deleting a linked connector
or document clears that link while keeping the note. Only instance admins see lab
audit actions in Journal; security events stay on the Audit page.
