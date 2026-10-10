# Lab journal

Open **Journal** in the sidebar or command palette to see lab activity in time
order. Changes, alerts, document edits and notes appear alongside failed syncs
and syncs that produced changes or alerts. Enable **Show all sync runs** to include
quiet successful runs. Filter by connector, source or UTC date range, and use
**Load more** to continue through older activity. **View source** opens the related
change, alert, document or connector when a member can identify that source;
instance admins can open audit rows in Audit.

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
or document clears that link while keeping the note.

Journal includes the allowlisted lab actions from the audit log. Each audit row
keeps its connector scope from the time the action was recorded, including rows
backfilled from existing audit records. A member sees a scoped row only when they
have a viewer or operator grant on every connector in that scope. A row with no
connector scope is visible only to instance admins. Connector-restricted API keys
also need to allow every connector in a nonempty row scope; they cannot read
unscoped rows. Instance admins can see unscoped rows and rows scoped to connectors
that have since been deleted; live connector scopes still require the matching
grant. Security actions remain on the Audit page and are not included in Journal.
Journal shows the audit actor but not audit detail. Member source links point to
the related document or connector when the row fields identify one; otherwise no
source link is shown.
